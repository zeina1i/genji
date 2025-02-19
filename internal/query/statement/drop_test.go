package statement_test

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/genjidb/genji"
	errs "github.com/genjidb/genji/internal/errors"
	"github.com/genjidb/genji/internal/testutil"
	"github.com/genjidb/genji/internal/testutil/assert"
	"github.com/genjidb/genji/types"
	"github.com/stretchr/testify/require"
)

func TestDropTable(t *testing.T) {
	db, err := genji.Open(":memory:")
	assert.NoError(t, err)
	defer db.Close()

	err = db.Exec("CREATE TABLE test1(a INT UNIQUE); CREATE TABLE test2; CREATE TABLE test3")
	assert.NoError(t, err)

	err = db.Exec("DROP TABLE test1")
	assert.NoError(t, err)

	err = db.Exec("DROP TABLE IF EXISTS test1")
	assert.NoError(t, err)

	// Dropping a table that doesn't exist without "IF EXISTS"
	// should return an error.
	err = db.Exec("DROP TABLE test1")
	assert.Error(t, err)

	// Assert that no other table has been dropped.
	res, err := db.Query("SELECT name FROM __genji_catalog WHERE type = 'table'")
	assert.NoError(t, err)
	var tables []string
	err = res.Iterate(func(d types.Document) error {
		v, err := d.GetByField("name")
		if err != nil {
			return err
		}
		tables = append(tables, types.As[string](v))
		return nil
	})
	assert.NoError(t, err)
	assert.NoError(t, res.Close())

	require.Equal(t, []string{"__genji_catalog", "__genji_sequence", "test2", "test3"}, tables)

	// Assert the unique index test1_a_idx, created upon the creation of the table,
	// has been dropped as well.
	_, err = db.QueryDocument("SELECT 1 FROM __genji_catalog WHERE name = 'test1_a_idx'")
	assert.Error(t, err)

	// Assert the docid sequence test1_seq, created upon the creation of the table,
	// has been dropped as well.
	_, err = db.QueryDocument("SELECT 1 FROM __genji_catalog WHERE name = 'test1_seq'")
	assert.Error(t, err)
	_, err = db.QueryDocument("SELECT 1 FROM __genji_sequence WHERE name = 'test1_seq'")
	assert.Error(t, err)

	// Dropping a read-only table should fail.
	err = db.Exec("DROP TABLE __genji_catalog")
	assert.Error(t, err)
}

func TestDropIndex(t *testing.T) {
	db, tx, cleanup := testutil.NewTestTx(t)
	defer cleanup()

	testutil.MustExec(t, db, tx, `
		CREATE TABLE test1(foo text, bar int unique); CREATE INDEX idx_test1_foo ON test1(foo);
		CREATE TABLE test2(bar text); CREATE INDEX idx_test2_bar ON test2(bar);
	`)

	testutil.MustExec(t, db, tx, "DROP INDEX idx_test2_bar")

	// Assert that the good index has been dropped.
	indexes := db.Catalog.ListIndexes("")
	require.Len(t, indexes, 2)
	require.Equal(t, "idx_test1_foo", indexes[0])
	require.Equal(t, "test1_bar_idx", indexes[1])

	// Dropping a non existing index with IF EXISTS should not fail.
	err := testutil.Exec(db, tx, "DROP INDEX IF EXISTS unknown")
	assert.NoError(t, err)

	// Dropping an index created with a table constraint should fail.
	err = testutil.Exec(db, tx, "DROP INDEX test1_bar_idx")
	assert.Error(t, err)
}

func TestDropSequence(t *testing.T) {
	db, tx, cleanup := testutil.NewTestTx(t)
	defer cleanup()

	testutil.MustExec(t, db, tx, `
		CREATE TABLE test1(foo int);
		CREATE SEQUENCE seq1;
		CREATE SEQUENCE seq2;
	`)

	testutil.MustExec(t, db, tx, "DROP SEQUENCE seq1")

	// Assert that the good index has been dropped.
	_, err := db.Catalog.GetSequence("seq1")
	require.IsType(t, &errs.NotFoundError{}, errors.Unwrap(err))
	_, err = db.Catalog.GetSequence("seq2")
	assert.NoError(t, err)

	// Dropping a non existing sequence with IF EXISTS should not fail.
	err = testutil.Exec(db, tx, "DROP SEQUENCE IF EXISTS unknown")
	assert.NoError(t, err)

	// Dropping a sequence created with a table constraint should fail.
	err = testutil.Exec(db, tx, "DROP SEQUENCE test1_seq")
	assert.Error(t, err)
}
