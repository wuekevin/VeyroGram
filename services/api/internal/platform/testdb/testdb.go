// Package testdb gives integration tests a throwaway MySQL database. Set VG_TEST_MYSQL_DSN to an account that may create and drop
// databases named veyrogram_it_* (e.g. "vg_it:pass@tcp(127.0.0.1:3306)/"); without it the tests are skipped, never faked.
package testdb

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"testing"

	"github.com/go-sql-driver/mysql"
)

// New creates veyrogram_it_<random>, returns a pool on it, and drops it at test cleanup.
func New(t testing.TB) *sql.DB {
	t.Helper()
	db, _ := NewWithDSN(t)
	return db
}

// NewWithDSN is New plus the DSN of the created database (for tests that start the real binary).
func NewWithDSN(t testing.TB) (*sql.DB, string) {
	t.Helper()
	raw := os.Getenv("VG_TEST_MYSQL_DSN")
	if raw == "" {
		t.Skip("VG_TEST_MYSQL_DSN not set: skipping MySQL integration test")
	}
	cfg, err := mysql.ParseDSN(raw)
	if err != nil {
		t.Fatalf("bad VG_TEST_MYSQL_DSN: %v", err)
	}
	cfg.ParseTime, cfg.MultiStatements = true, false
	cfg.Loc, cfg.DBName = timeUTC(), ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "veyrogram_it_" + hex.EncodeToString(b)
	if _, err := admin.Exec("CREATE DATABASE `" + name + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	cfg.DBName = name
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec("DROP DATABASE `" + name + "`")
		admin.Close()
	})
	return db, cfg.FormatDSN()
}
