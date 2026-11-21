package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// passRE: a conservative alphabet that needs no escaping inside a SQL string literal (no quotes, backslash or whitespace).
var passRE = regexp.MustCompile(`^[A-Za-z0-9_\-+=.@#%^*!~:,]{24,128}$`)

// Provision creates (or re-keys) the application database and the two non-admin accounts of the three-account model, idempotently:
//
//	migrator: DDL + DML on the database (runs `migrate`)
//	app:      NO database-level grant; it gets per-table rights from GrantApp after migrations (so audit_log stays append-only)
//
// Re-running resets both passwords, which is how rotation is done. Passwords come from the caller (read from env/secret files, never argv).
func Provision(ctx context.Context, admin *sql.DB, database, host, appUser, appPass, migUser, migPass string) error {
	for _, v := range []string{database, host, appUser, migUser} {
		if !identRE.MatchString(v) {
			return fmt.Errorf("db: unsafe identifier %q", v)
		}
	}
	if appUser == migUser {
		return fmt.Errorf("db: the application and migrator accounts must differ")
	}
	for _, p := range []string{appPass, migPass} {
		if !passRE.MatchString(p) {
			return fmt.Errorf("db: passwords must be 24-128 chars from [A-Za-z0-9_-+=.@#%%^*!~:,]")
		}
	}
	if appPass == migPass {
		return fmt.Errorf("db: the application and migrator passwords must differ")
	}
	stmts := []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci", database),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", appUser, host, appPass),
		fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", appUser, host, appPass),
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", migUser, host, migPass),
		fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", migUser, host, migPass),
		fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, ALTER, DROP, INDEX, REFERENCES ON `%s`.* TO '%s'@'%s'", database, migUser, host),
	}
	for _, s := range stmts {
		if _, err := admin.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("db: provisioning failed (a password is never echoed): %w", sanitize(err, appPass, migPass))
		}
	}
	return nil
}

func sanitize(err error, secrets ...string) error {
	msg := err.Error()
	for _, s := range secrets {
		msg = strings.ReplaceAll(msg, s, "***")
	}
	return errors.New(msg)
}

// BackupGrants are the global privileges of the backup account, and no more: reading data (SELECT, SHOW VIEW, TRIGGER, EVENT), a consistent dump with a fresh
// binlog file (RELOAD), the binlog position and file list (REPLICATION CLIENT), and streaming closed binlog files (REPLICATION SLAVE). It cannot write,
// change schema or manage accounts; MySQL has no narrower way to allow the last three, which are global-only privileges.
const BackupGrants = "SELECT, SHOW VIEW, TRIGGER, EVENT, RELOAD, REPLICATION CLIENT, REPLICATION SLAVE"

// ProvisionBackup creates (or re-keys) the backup account used by infrastructure/backup/backup.sh. Idempotent; re-running rotates the password.
func ProvisionBackup(ctx context.Context, admin *sql.DB, host, user, pass string) error {
	for _, v := range []string{host, user} {
		if !identRE.MatchString(v) {
			return fmt.Errorf("db: unsafe identifier %q", v)
		}
	}
	if !passRE.MatchString(pass) {
		return fmt.Errorf("db: passwords must be 24-128 chars from [A-Za-z0-9_-+=.@#%%^*!~:,]")
	}
	for _, s := range []string{
		fmt.Sprintf("CREATE USER IF NOT EXISTS '%s'@'%s' IDENTIFIED BY '%s'", user, host, pass),
		fmt.Sprintf("ALTER USER '%s'@'%s' IDENTIFIED BY '%s'", user, host, pass),
		fmt.Sprintf("GRANT %s ON *.* TO '%s'@'%s'", BackupGrants, user, host),
	} {
		if _, err := admin.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("db: provisioning the backup account failed (the password is never echoed): %w", sanitize(err, pass))
		}
	}
	return nil
}
