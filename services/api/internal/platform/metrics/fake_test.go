package metrics_test

import (
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
)

// openFake returns an unconnected pool: sql.Open does not dial, and Stats() works without a connection.
func openFake(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("mysql", "u:p@tcp(127.0.0.1:1)/x")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}
