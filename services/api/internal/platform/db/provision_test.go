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
	if _, err := appDB.Exec("INSERT INTO login_event (kind, result, created_at) VALUES ('login','ok',NOW(3))"); err != nil {
		t.Fatalf("app must be able to append login events: %v", err)
	}
	for _, q := range []string{"UPDATE login_event SET result='ok'", "DELETE FROM login_event"} {
		if _, err := appDB.Exec(q); err == nil {
			t.Fatalf("app must not rewrite login_event (%s)", q)
		}
	}
	if _, err := appDB.Exec("UPDATE sms_code SET attempts=0"); err != nil {
		t.Fatalf("app needs UPDATE on sms_code: %v", err)
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

// The backup account can read and stream binlogs, and nothing else: no writes, no schema changes, no account management, password rotation works.
func TestProvisionBackupAccountIsReadOnlyAndRotates(t *testing.T) {
	adminDSN := os.Getenv("VG_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("VG_TEST_MYSQL_ADMIN_DSN not set")
	}
	admin, err := sql.Open("mysql", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	b := make([]byte, 4)
	rand.Read(b)
	sfx := hex.EncodeToString(b)
	user, database := "vg_bk_p"+sfx, "veyrogram_it_bk_"+sfx
	t.Cleanup(func() {
		admin.Exec("DROP DATABASE IF EXISTS `" + database + "`")
		admin.Exec("DROP USER IF EXISTS '" + user + "'@'127.0.0.1'")
	})
	ctx := context.Background()
	if _, err := admin.Exec("CREATE DATABASE `" + database + "`"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE TABLE `" + database + "`.t (id INT PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("INSERT INTO `" + database + "`.t VALUES (1)"); err != nil {
		t.Fatal(err)
	}
	pass1, pass2 := "B"+strings.Repeat("b", 30)+sfx, "C"+strings.Repeat("c", 30)+sfx
	if err := db.ProvisionBackup(ctx, admin, "127.0.0.1", user, "short"); err == nil {
		t.Fatal("weak password accepted")
	}
	if err := db.ProvisionBackup(ctx, admin, "127.0.0.1", "x'; DROP", pass1); err == nil {
		t.Fatal("unsafe identifier accepted")
	}
	if err := db.ProvisionBackup(ctx, admin, "127.0.0.1", user, pass1); err != nil {
		t.Fatal(err)
	}
	if err := db.ProvisionBackup(ctx, admin, "127.0.0.1", user, pass1); err != nil {
		t.Fatalf("must be idempotent: %v", err)
	}
	open := func(pass string) *sql.DB {
		cfg, _ := mysql.ParseDSN(adminDSN)
		cfg.User, cfg.Passwd, cfg.DBName = user, pass, ""
		c, err := mysql.NewConnector(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return sql.OpenDB(c)
	}
	bk := open(pass1)
	defer bk.Close()
	var n int
	if err := bk.QueryRow("SELECT COUNT(*) FROM `" + database + "`.t").Scan(&n); err != nil || n != 1 {
		t.Fatalf("the backup account must be able to read: %d %v", n, err)
	}
	if rows, err := bk.Query("SHOW BINARY LOGS"); err != nil {
		t.Fatalf("it needs the binlog list: %v", err)
	} else {
		rows.Close()
	}
	for _, stmt := range []string{
		"INSERT INTO `" + database + "`.t VALUES (2)", "DELETE FROM `" + database + "`.t", "UPDATE `" + database + "`.t SET id=9",
		"DROP TABLE `" + database + "`.t", "CREATE TABLE `" + database + "`.u (id INT)", "CREATE USER 'zz_evil'@'%' IDENTIFIED BY 'x'",
		"GRANT ALL ON *.* TO '" + user + "'@'127.0.0.1'", "SET GLOBAL read_only=0",
	} {
		if _, err := bk.Exec(stmt); err == nil {
			t.Errorf("the backup account must not be able to run: %s", stmt)
			admin.Exec("DROP USER IF EXISTS 'zz_evil'@'%'")
		}
	}
	if err := db.ProvisionBackup(ctx, admin, "127.0.0.1", user, pass2); err != nil {
		t.Fatal(err)
	}
	if err := bk2Ping(open(pass1)); err == nil {
		t.Error("the old password must stop working after rotation")
	}
	if err := bk2Ping(open(pass2)); err != nil {
		t.Errorf("the new password must work: %v", err)
	}
}

func bk2Ping(d *sql.DB) error { defer d.Close(); return d.Ping() }
