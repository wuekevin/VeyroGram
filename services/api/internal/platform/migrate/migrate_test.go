package migrate_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/migrations"
)

func files(kv ...string) fstest.MapFS {
	m := fstest.MapFS{}
	for i := 0; i < len(kv); i += 2 {
		m[kv[i]] = &fstest.MapFile{Data: []byte(kv[i+1])}
	}
	return m
}

const tbl = "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci"

// ---- unit (no database) ----

func TestLoadRequiresDownScriptAndValidNames(t *testing.T) {
	if _, err := migrate.Load(files("0001_a.up.sql", "CREATE TABLE a (id INT) "+tbl)); err == nil {
		t.Fatal("missing down script must be rejected")
	}
	if _, err := migrate.Load(files("0001_a.down.sql", "DROP TABLE a")); err == nil {
		t.Fatal("missing up script must be rejected")
	}
	if _, err := migrate.Load(files("1_bad.up.sql", "x", "1_bad.down.sql", "y")); err == nil {
		t.Fatal("bad file name must be rejected")
	}
	if _, err := migrate.Load(files("0001_a.up.sql", "x", "0001_b.down.sql", "y")); err == nil {
		t.Fatal("two names for one version must be rejected")
	}
	ms, err := migrate.Load(files("0002_b.up.sql", "CREATE TABLE b (id INT) "+tbl, "0002_b.down.sql", "DROP TABLE b", "0001_a.up.sql", "CREATE TABLE a (id INT) "+tbl, "0001_a.down.sql", "DROP TABLE a"))
	if err != nil || len(ms) != 2 || ms[0].Version != 1 || ms[1].Version != 2 || ms[0].Checksum == "" {
		t.Fatalf("expected two ordered migrations, got %v err=%v", ms, err)
	}
}

func TestEmbeddedMigrationsAreWellFormedAndLintClean(t *testing.T) {
	ms, err := migrate.Load(migrations.FS)
	if err != nil || len(ms) == 0 {
		t.Fatalf("embedded migrations must load: %v", err)
	}
	for _, m := range ms {
		for _, f := range migrate.Lint(m) {
			t.Error(f)
		}
	}
}

func TestSplitStatements(t *testing.T) {
	got := migrate.SplitStatements("CREATE TABLE t (a VARCHAR(5) DEFAULT 'x;y'); -- comment; with semi\nINSERT INTO t VALUES ('it''s;ok'); /* c; */ SELECT `a;b` FROM t;\n# hash ; comment\nSELECT 1")
	want := []string{"CREATE TABLE t (a VARCHAR(5) DEFAULT 'x;y')", "INSERT INTO t VALUES ('it''s;ok')", "SELECT `a;b` FROM t", "SELECT 1"}
	norm := func(x string) string { return strings.Join(strings.Fields(x), " ") }
	if len(got) != len(want) {
		t.Fatalf("got %d statements %q, want %d", len(got), got, len(want))
	}
	for i := range want {
		if norm(got[i]) != norm(want[i]) {
			t.Errorf("statement %d = %q want %q", i, got[i], want[i])
		}
	}
}

func TestLintRules(t *testing.T) {
	bad := []struct{ name, sql string }{
		{"text default", "CREATE TABLE a (id INT, note TEXT DEFAULT 'x') " + tbl},
		{"blob default", "CREATE TABLE a (id INT, b MEDIUMBLOB DEFAULT NULL) " + tbl},
		{"json default", "CREATE TABLE a (id INT, j JSON DEFAULT ('{}')) " + tbl},
		{"alter text default", "ALTER TABLE a ADD COLUMN c LONGTEXT NOT NULL DEFAULT ''"},
		{"no engine", "CREATE TABLE a (id INT) DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci"},
		{"wrong collation", "CREATE TABLE a (id INT) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"},
		{"drop database", "DROP DATABASE x"},
	}
	for _, c := range bad {
		if len(migrate.Lint(migrate.Migration{Version: 9, Name: "x", Up: c.sql, Down: "SELECT 1"})) == 0 {
			t.Errorf("%s must be flagged", c.name)
		}
	}
	good := "CREATE TABLE a (id INT NOT NULL DEFAULT 1, note TEXT NULL, name VARCHAR(10) NOT NULL DEFAULT 'x') " + tbl
	if f := migrate.Lint(migrate.Migration{Version: 9, Name: "x", Up: good, Down: "DROP TABLE a"}); len(f) != 0 {
		t.Errorf("clean migration flagged: %v", f)
	}
}

// ---- integration (real MySQL 8.0; skipped without VG_TEST_MYSQL_DSN) ----

func runner(t *testing.T, fs fstest.MapFS) *migrate.Runner {
	t.Helper()
	ms, err := migrate.Load(fs)
	if err != nil {
		t.Fatal(err)
	}
	return &migrate.Runner{DB: testdb.New(t), Migrations: ms}
}

func twoTables() fstest.MapFS {
	return files("0001_a.up.sql", "CREATE TABLE a (id INT NOT NULL PRIMARY KEY) "+tbl, "0001_a.down.sql", "DROP TABLE a",
		"0002_b.up.sql", "CREATE TABLE b (id INT NOT NULL PRIMARY KEY) "+tbl, "0002_b.down.sql", "DROP TABLE b")
}

func tableExists(t *testing.T, r *migrate.Runner, name string) bool {
	t.Helper()
	var n int
	if err := r.DB.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name=?", name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n == 1
}

func TestEmbeddedMigrationsBuildCompliantSchema(t *testing.T) {
	db := testdb.New(t)
	ms, _ := migrate.Load(migrations.FS)
	r := &migrate.Runner{DB: db, Migrations: ms}
	if _, err := r.Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, tb := range []string{"schema_migration", "schema_migration_state", "idempotency_key"} {
		if !tableExists(t, r, tb) {
			t.Errorf("table %s missing", tb)
		}
	}
	var bad int
	// AD-5: no DEFAULT on TEXT/BLOB/JSON columns, anywhere in the schema
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE()
		AND data_type IN ('tinytext','text','mediumtext','longtext','tinyblob','blob','mediumblob','longblob','json') AND column_default IS NOT NULL`).Scan(&bad); err != nil || bad != 0 {
		t.Fatalf("TEXT/BLOB/JSON columns with DEFAULT: %d err=%v", bad, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND (engine <> 'InnoDB' OR table_collation <> 'utf8mb4_0900_ai_ci')`).Scan(&bad); err != nil || bad != 0 {
		t.Fatalf("tables not InnoDB/utf8mb4_0900_ai_ci: %d err=%v", bad, err)
	}
}

func TestUpIsIdempotentAndStatusReflectsState(t *testing.T) {
	r := runner(t, twoTables())
	ctx := context.Background()
	if done, err := r.Up(ctx); err != nil || len(done) != 2 {
		t.Fatalf("first up applies 2, got %v %v", done, err)
	}
	if done, err := r.Up(ctx); err != nil || len(done) != 0 {
		t.Fatalf("second up applies nothing, got %v %v", done, err)
	}
	st, _ := r.Status(ctx)
	if len(st.Applied) != 2 || len(st.Pending) != 0 || st.Dirty {
		t.Fatalf("unexpected status %+v", st)
	}
}

func TestDownRestoresPreviousVersion(t *testing.T) {
	r := runner(t, twoTables())
	ctx := context.Background()
	_, _ = r.Up(ctx)
	if done, err := r.Down(ctx, 1); err != nil || len(done) != 1 || done[0] != 2 {
		t.Fatalf("down 1 must revert version 2, got %v %v", done, err)
	}
	if tableExists(t, r, "b") || !tableExists(t, r, "a") {
		t.Fatal("table b must be gone and table a kept")
	}
	st, _ := r.Status(ctx)
	if len(st.Applied) != 1 || len(st.Pending) != 1 {
		t.Fatalf("status after down: %+v", st)
	}
	if _, err := r.Up(ctx); err != nil || !tableExists(t, r, "b") {
		t.Fatalf("up again must re-apply 2: %v", err)
	}
}

func brokenSecond() fstest.MapFS {
	return files("0001_a.up.sql", "CREATE TABLE a (id INT NOT NULL PRIMARY KEY) "+tbl, "0001_a.down.sql", "DROP TABLE a",
		// first statement succeeds (and commits implicitly), the second fails: a half-applied migration
		"0002_b.up.sql", "CREATE TABLE b (id INT NOT NULL PRIMARY KEY) "+tbl+"; ALTER TABLE does_not_exist ADD COLUMN x INT;", "0002_b.down.sql", "DROP TABLE IF EXISTS b")
}

func TestFailedMigrationLeavesDirtyStateAndIsNotRecorded(t *testing.T) {
	r := runner(t, brokenSecond())
	ctx := context.Background()
	done, err := r.Up(ctx)
	if err == nil || len(done) != 1 {
		t.Fatalf("expected failure after applying version 1, got done=%v err=%v", done, err)
	}
	var n int
	_ = r.DB.QueryRow("SELECT COUNT(*) FROM schema_migration WHERE version=2").Scan(&n)
	if n != 0 {
		t.Fatal("the failed version must not be recorded in schema_migration")
	}
	st, _ := r.Status(ctx)
	if !st.Dirty || st.DirtyVersion != 2 || st.DirtyDir != "up" {
		t.Fatalf("state must be dirty at version 2 (up), got %+v", st)
	}
	if !tableExists(t, r, "b") {
		t.Fatal("test premise: DDL of the failed migration was partially applied (MySQL DDL is not transactional)")
	}
	if _, err := r.Up(ctx); !errors.Is(err, migrate.ErrDirty) {
		t.Fatalf("a dirty database must refuse to continue, got %v", err)
	}
	if _, err := r.Down(ctx, 1); !errors.Is(err, migrate.ErrDirty) {
		t.Fatalf("down must also refuse while dirty, got %v", err)
	}
	// operator repairs by running the down script by hand, then clears the flag; the previous version is intact
	if _, err := r.DB.Exec("DROP TABLE IF EXISTS b"); err != nil {
		t.Fatal(err)
	}
	if err := r.ClearDirty(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ = r.Status(ctx)
	if st.Dirty || len(st.Applied) != 1 || !tableExists(t, r, "a") {
		t.Fatalf("after repair expected clean state at version 1, got %+v", st)
	}
}

func TestEditedAppliedMigrationIsRefused(t *testing.T) {
	fs := twoTables()
	r := runner(t, fs)
	ctx := context.Background()
	_, _ = r.Up(ctx)
	fs["0001_a.up.sql"] = &fstest.MapFile{Data: []byte("CREATE TABLE a (id INT NOT NULL PRIMARY KEY, evil INT) " + tbl)}
	ms, _ := migrate.Load(fs)
	r2 := &migrate.Runner{DB: r.DB, Migrations: ms}
	if _, err := r2.Up(ctx); !errors.Is(err, migrate.ErrChecksum) {
		t.Fatalf("editing an applied migration must be refused, got %v", err)
	}
}

func TestAppliedVersionWithoutFileIsRefused(t *testing.T) {
	r := runner(t, twoTables())
	ctx := context.Background()
	_, _ = r.Up(ctx)
	ms, _ := migrate.Load(files("0001_a.up.sql", "CREATE TABLE a (id INT NOT NULL PRIMARY KEY) "+tbl, "0001_a.down.sql", "DROP TABLE a"))
	if _, err := (&migrate.Runner{DB: r.DB, Migrations: ms}).Up(ctx); !errors.Is(err, migrate.ErrUnknownApplied) {
		t.Fatalf("an applied version missing from the files must be refused, got %v", err)
	}
}

func TestOutOfOrderPendingMigrationIsRefused(t *testing.T) {
	r := runner(t, files("0002_b.up.sql", "CREATE TABLE b (id INT NOT NULL PRIMARY KEY) "+tbl, "0002_b.down.sql", "DROP TABLE b"))
	ctx := context.Background()
	_, _ = r.Up(ctx)
	ms, _ := migrate.Load(files("0001_a.up.sql", "CREATE TABLE a (id INT NOT NULL PRIMARY KEY) "+tbl, "0001_a.down.sql", "DROP TABLE a",
		"0002_b.up.sql", "CREATE TABLE b (id INT NOT NULL PRIMARY KEY) "+tbl, "0002_b.down.sql", "DROP TABLE b"))
	if _, err := (&migrate.Runner{DB: r.DB, Migrations: ms}).Up(ctx); !errors.Is(err, migrate.ErrOutOfOrder) {
		t.Fatalf("inserting a lower version after a higher one is applied must be refused, got %v", err)
	}
}

func TestConcurrentRunnersApplyEachMigrationOnce(t *testing.T) {
	r := runner(t, twoTables())
	ctx := context.Background()
	var wg sync.WaitGroup
	var mu sync.Mutex
	total, errs := 0, 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			done, err := (&migrate.Runner{DB: r.DB, Migrations: r.Migrations}).Up(ctx)
			mu.Lock()
			defer mu.Unlock()
			total += len(done)
			if err != nil {
				errs++
			}
		}()
	}
	wg.Wait()
	if errs != 0 || total != 2 {
		t.Fatalf("6 concurrent runners must apply exactly 2 migrations in total without errors, got applied=%d errors=%d", total, errs)
	}
}
