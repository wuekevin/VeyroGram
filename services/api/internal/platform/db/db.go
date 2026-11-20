// Package db opens the MySQL pool with the settings the platform relies on (UTC times, no multi-statements).
package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Open parses dsn, forces parseTime/UTC and disables multiStatements (SQL injection blast radius), and returns a pool. It does not connect.
func Open(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid VG_MYSQL_DSN: %w", err)
	}
	cfg.ParseTime, cfg.MultiStatements, cfg.Loc = true, false, time.UTC
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["sql_mode"] = "'STRICT_ALL_TABLES,NO_ENGINE_SUBSTITUTION,ONLY_FULL_GROUP_BY'"
	d, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(50)
	d.SetMaxIdleConns(10)
	d.SetConnMaxLifetime(30 * time.Minute)
	return d, nil
}

// Ready is a health.Check that pings the database.
func Ready(d *sql.DB) func(context.Context) error {
	return func(ctx context.Context) error { return d.PingContext(ctx) }
}
