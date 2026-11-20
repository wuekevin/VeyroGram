package db

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
)

var identRE = regexp.MustCompile(`^[A-Za-z0-9_$.\-]{1,64}$`)

// appendOnly tables get SELECT and INSERT only: the application account can never rewrite them (AD-15).
var appendOnly = map[string]bool{"audit_log": true}

// GrantApp grants the application account table-level DML on every table of database, and SELECT+INSERT only on append-only tables.
// Run it as an administrator after every migration. The account must already exist.
func GrantApp(ctx context.Context, admin *sql.DB, database, user, host string) ([]string, error) {
	for _, v := range []string{database, user, host} {
		if !identRE.MatchString(v) {
			return nil, fmt.Errorf("db: unsafe identifier %q", v)
		}
	}
	rows, err := admin.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema=? AND table_type='BASE TABLE' ORDER BY table_name", database)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, n)
	}
	rows.Close()
	var granted []string
	for _, t := range tables {
		if !identRE.MatchString(t) {
			return granted, fmt.Errorf("db: unsafe table name %q", t)
		}
		privs := "SELECT, INSERT, UPDATE, DELETE"
		if appendOnly[t] {
			privs = "SELECT, INSERT"
		}
		q := fmt.Sprintf("GRANT %s ON `%s`.`%s` TO '%s'@'%s'", privs, database, t, user, host)
		if _, err := admin.ExecContext(ctx, q); err != nil {
			return granted, err
		}
		granted = append(granted, t)
	}
	return granted, nil
}
