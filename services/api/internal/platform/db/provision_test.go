package db_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"

	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/migrations"
)

func TestProvisionThenMigrateThenGrantGivesTheThreeAccountModel(t *testing.T) {
	adminDSN := os.Getenv("VG_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("VG_TEST_MYSQL_ADMIN_DSN not set: skipping provisioning test")
	}
	admin, err := sql.Open("mysql", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() }) // registered first so it runs after the DROP cleanups below
	b := make([]byte, 4)
	rand.Read(b)
	sfx := hex.EncodeToString(b)
	database, app, mig := "veyrogram_it_prov_"+sfx, "vg_app_p"+sfx, "vg_mig_p"+sfx
	t.Cleanup(func() {
		admin.Exec("DROP DATABASE IF EXISTS `" + database + "`")
		admin.Exec("DROP USER IF EXISTS '" + app + "'@'127.0.0.1'")
		admin.Exec("DROP USER IF EXISTS '" + mig + "'@'127.0.0.1'")
	})
	ctx := context.Background()
	appPass, migPass := "A"+strings.Repeat("a", 30)+sfx, "M"+strings.Repeat("m", 30)+sfx

	// unsafe input is refused before anything is sent
	for _, tc := range [][]string{{"x'; DROP", "pass"}, {database, "short"}} {
		if err := db.Provision(ctx, admin, tc[0], "127.0.0.1", app, appPass, mig, migPass+"z"); err == nil && tc[0] != database {
			t.Fatalf("unsafe identifier %q accepted", tc[0])
		}
	}
	if err := db.Provision(ctx, admin, database, "127.0.0.1", app, "short", mig, migPass); err == nil {
		t.Fatal("weak password accepted")
	}
	if err := db.Provision(ctx, admin, database, "127.0.0.1", app, appPass, app, migPass); err == nil {
		t.Fatal("app and migrator must differ")
	}
	if err := db.Provision(ctx, admin, database, "127.0.0.1", app, appPass, mig, migPass); err != nil {
		t.Fatal(err)
	}
	if err := db.Provision(ctx, admin, database, "127.0.0.1", app, appPass, mig, migPass); err != nil {
		t.Fatalf("provisioning must be idempotent: %v", err)
	}
	open := func(user, pass string) *sql.DB {
		cfg, _ := mysql.ParseDSN(adminDSN)
		cfg.User, cfg.Passwd, cfg.DBName, cfg.ParseTime = user, pass, database, true
		d, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { d.Close() })
		return d
	}
	migDB, appDB := open(mig, migPass), open(app, appPass)
	if err := migDB.Ping(); err != nil {
		t.Fatalf("migrator must connect: %v", err)
	}
	set, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&migrate.Runner{DB: migDB, Migrations: set}).Up(ctx); err != nil {
		t.Fatalf("the migrator account must be able to run the migrations: %v", err)
	}
	if _, err := appDB.Exec("SELECT 1 FROM outbox"); err == nil {
		t.Fatal("before grant-app the application account must have no table access")
	}
	if _, err := db.GrantApp(ctx, admin, database, app, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := appDB.Exec("UPDATE outbox SET attempts=0"); err != nil {
		t.Fatalf("app DML after grant: %v", err)
	}
	if _, err := appDB.Exec("CREATE TABLE nope (id INT)"); err == nil {
		t.Fatal("app must have no DDL")
	}
	if _, err := appDB.Exec("DELETE FROM audit_log"); err == nil {
		t.Fatal("app must not delete audit_log")
	}
	// rotation: re-provisioning with a new password invalidates the old one
	newPass := "N" + strings.Repeat("n", 30) + sfx
	if err := db.Provision(ctx, admin, database, "127.0.0.1", app, newPass, mig, migPass); err != nil {
		t.Fatal(err)
	}
	if err := open(app, appPass).Ping(); err == nil {
		t.Fatal("the old password must stop working after rotation")
	}
	if err := open(app, newPass).Ping(); err != nil {
		t.Fatalf("new password must work: %v", err)
	}
}
