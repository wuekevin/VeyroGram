// Package migrate is a small forward-only SQL migration runner for MySQL 8.0.
//
// MySQL DDL commits implicitly, so a failed migration cannot be rolled back by a transaction. The runner therefore uses a "dirty" protocol:
// before a migration (or its down script) runs, a state row is written; it is cleared only on success. A failed run leaves the state set and
// the version unrecorded in schema_migration; every later run refuses to continue until an operator repairs the database (down script or backup)
// and calls ClearDirty. Concurrent runners are serialised with a database named lock; applied migrations are checksum-verified.
package migrate

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrDirty          = errors.New("migrate: database is in a dirty state from a failed migration; repair it (down script or backup) then clear the dirty flag")
	ErrChecksum       = errors.New("migrate: an applied migration file was modified after it was applied")
	ErrUnknownApplied = errors.New("migrate: database contains an applied version that has no migration file")
	ErrOutOfOrder     = errors.New("migrate: a pending migration has a lower version than one already applied")
	ErrLockTimeout    = errors.New("migrate: could not obtain the migration lock (another run in progress?)")
	// ErrForeignSchema: the target database holds tables of another application (an upstream TangSengDaoDao deployment) and no VeyroGram
	// migration history. VeyroGram has no compat layer and no in-place migration (ADR-0011); it deploys only into an empty database.
	ErrForeignSchema = errors.New("migrate: the database contains an upstream TangSengDaoDao schema and no VeyroGram migration history; VeyroGram does not migrate or adopt it (ADR-0011) — deploy into a new empty database")
)

// upstreamFingerprint lists tables that only an upstream TangSengDaoDao database has (its migrator records history in gorp_migrations; the
// others are upstream core tables). Several share a name with VeyroGram tables (user, device), so running our DDL there would corrupt it.
var upstreamFingerprint = []string{"gorp_migrations", "app", "app_config", "friend", "group_member", "message_extra", "user_setting", "shortno"}

const lockName = "veyrogram_migrate"

// Migration is one versioned change with its inverse.
type Migration struct {
	Version  uint64
	Name     string
	Up, Down string
	Checksum string // sha256 of the up script
}

var fileRE = regexp.MustCompile(`^(\d{4,})_([a-z0-9_]+)\.(up|down)\.sql$`)

// Load parses migrations from fsys and validates that every version has both scripts and versions are unique.
func Load(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, err
	}
	byV := map[uint64]*Migration{}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		m := fileRE.FindStringSubmatch(e.Name())
		if m == nil {
			return nil, fmt.Errorf("migrate: bad migration file name %q (want NNNN_name.up.sql / NNNN_name.down.sql)", e.Name())
		}
		v, _ := strconv.ParseUint(m[1], 10, 64)
		body, err := fs.ReadFile(fsys, e.Name())
		if err != nil {
			return nil, err
		}
		mg := byV[v]
		if mg == nil {
			mg = &Migration{Version: v, Name: m[2]}
			byV[v] = mg
		} else if mg.Name != m[2] {
			return nil, fmt.Errorf("migrate: version %d has two different names (%s, %s)", v, mg.Name, m[2])
		}
		key := fmt.Sprintf("%d.%s", v, m[3])
		if seen[key] {
			return nil, fmt.Errorf("migrate: duplicate %s script for version %d", m[3], v)
		}
		seen[key] = true
		if m[3] == "up" {
			mg.Up = string(body)
			sum := sha256.Sum256(body)
			mg.Checksum = hex.EncodeToString(sum[:])
		} else {
			mg.Down = string(body)
		}
	}
	out := make([]Migration, 0, len(byV))
	for _, mg := range byV {
		if strings.TrimSpace(mg.Up) == "" {
			return nil, fmt.Errorf("migrate: version %d (%s) has no up script", mg.Version, mg.Name)
		}
		if strings.TrimSpace(mg.Down) == "" {
			return nil, fmt.Errorf("migrate: version %d (%s) has no down script (every migration must be reversible)", mg.Version, mg.Name)
		}
		out = append(out, *mg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Runner applies migrations to one database.
type Runner struct {
	DB         *sql.DB
	Migrations []Migration
	Now        func() time.Time // defaults to time.Now
	LockWait   time.Duration    // defaults to 30s
}

// Status describes the database.
type Status struct {
	Applied      []uint64
	Pending      []uint64
	Dirty        bool
	DirtyVersion uint64
	DirtyDir     string
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

// session holds a dedicated connection that owns the named lock.
type session struct {
	c *sql.Conn
	r *Runner
}

func (r *Runner) begin(ctx context.Context) (*session, error) {
	c, err := r.DB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	wait := r.LockWait
	if wait == 0 {
		wait = 30 * time.Second
	}
	var got sql.NullInt64
	if err := c.QueryRowContext(ctx, "SELECT GET_LOCK(?, ?)", lockName, int(wait.Seconds())).Scan(&got); err != nil {
		c.Close()
		return nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		c.Close()
		return nil, ErrLockTimeout
	}
	s := &session{c: c, r: r}
	if err := s.refuseForeignSchema(ctx); err != nil {
		s.end()
		return nil, err
	}
	for _, q := range []string{
		`CREATE TABLE IF NOT EXISTS schema_migration (
  version BIGINT UNSIGNED NOT NULL PRIMARY KEY, name VARCHAR(190) NOT NULL, checksum CHAR(64) NOT NULL, applied_at DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`,
		`CREATE TABLE IF NOT EXISTS schema_migration_state (
  id TINYINT UNSIGNED NOT NULL PRIMARY KEY, dirty_version BIGINT UNSIGNED NOT NULL, direction VARCHAR(8) NOT NULL, since DATETIME(3) NOT NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci`,
	} {
		if _, err := c.ExecContext(ctx, q); err != nil {
			s.end()
			return nil, err
		}
	}
	return s, nil
}

// refuseForeignSchema fails with ErrForeignSchema when the database has no VeyroGram migration history but has upstream tables.
func (s *session) refuseForeignSchema(ctx context.Context) error {
	var own int
	if err := s.c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name='schema_migration'").Scan(&own); err != nil {
		return err
	}
	if own > 0 {
		return nil
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(upstreamFingerprint)), ",")
	args := make([]any, len(upstreamFingerprint))
	for i, t := range upstreamFingerprint {
		args[i] = t
	}
	var foreign int
	if err := s.c.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ("+ph+")", args...).Scan(&foreign); err != nil {
		return err
	}
	if foreign > 0 {
		return ErrForeignSchema
	}
	return nil
}

func (s *session) end() {
	var x sql.NullInt64
	_ = s.c.QueryRowContext(context.Background(), "SELECT RELEASE_LOCK(?)", lockName).Scan(&x)
	s.c.Close()
}

func (s *session) dirty(ctx context.Context) (v uint64, dir string, dirty bool, err error) {
	err = s.c.QueryRowContext(ctx, "SELECT dirty_version, direction FROM schema_migration_state WHERE id=1").Scan(&v, &dir)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, "", false, nil
	}
	return v, dir, err == nil, err
}

func (s *session) applied(ctx context.Context) (map[uint64]string, []uint64, error) {
	rows, err := s.c.QueryContext(ctx, "SELECT version, checksum FROM schema_migration ORDER BY version")
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	m := map[uint64]string{}
	var order []uint64
	for rows.Next() {
		var v uint64
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			return nil, nil, err
		}
		m[v] = sum
		order = append(order, v)
	}
	return m, order, rows.Err()
}

func (s *session) verify(ctx context.Context) (map[uint64]string, []uint64, error) {
	if _, _, d, err := s.dirty(ctx); err != nil {
		return nil, nil, err
	} else if d {
		return nil, nil, ErrDirty
	}
	applied, order, err := s.applied(ctx)
	if err != nil {
		return nil, nil, err
	}
	known := map[uint64]Migration{}
	for _, m := range s.r.Migrations {
		known[m.Version] = m
	}
	for _, v := range order {
		m, ok := known[v]
		if !ok {
			return nil, nil, fmt.Errorf("%w: version %d", ErrUnknownApplied, v)
		}
		if m.Checksum != applied[v] {
			return nil, nil, fmt.Errorf("%w: version %d (%s)", ErrChecksum, v, m.Name)
		}
	}
	return applied, order, nil
}

func (s *session) setDirty(ctx context.Context, v uint64, dir string) error {
	_, err := s.c.ExecContext(ctx, "REPLACE INTO schema_migration_state (id, dirty_version, direction, since) VALUES (1, ?, ?, ?)", v, dir, s.r.now())
	return err
}

func (s *session) clearDirty(ctx context.Context) error {
	_, err := s.c.ExecContext(ctx, "DELETE FROM schema_migration_state WHERE id=1")
	return err
}

func (s *session) exec(ctx context.Context, script string) error {
	for _, st := range SplitStatements(script) {
		if _, err := s.c.ExecContext(ctx, st); err != nil {
			return fmt.Errorf("%w (statement: %.120s)", err, st)
		}
	}
	return nil
}

// Up applies all pending migrations in version order and returns the versions it applied.
func (r *Runner) Up(ctx context.Context) ([]uint64, error) {
	s, err := r.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.end()
	applied, order, err := s.verify(ctx)
	if err != nil {
		return nil, err
	}
	var max uint64
	if len(order) > 0 {
		max = order[len(order)-1]
	}
	var done []uint64
	for _, m := range r.Migrations {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if m.Version < max {
			return done, fmt.Errorf("%w: pending %d < applied %d", ErrOutOfOrder, m.Version, max)
		}
		if err := s.setDirty(ctx, m.Version, "up"); err != nil {
			return done, err
		}
		if err := s.exec(ctx, m.Up); err != nil {
			return done, fmt.Errorf("migration %d (%s) failed, database left dirty: %w", m.Version, m.Name, err)
		}
		if _, err := s.c.ExecContext(ctx, "INSERT INTO schema_migration (version, name, checksum, applied_at) VALUES (?,?,?,?)", m.Version, m.Name, m.Checksum, r.now()); err != nil {
			return done, err
		}
		if err := s.clearDirty(ctx); err != nil {
			return done, err
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// Down reverts the last `steps` applied migrations (newest first) using their down scripts.
func (r *Runner) Down(ctx context.Context, steps int) ([]uint64, error) {
	if steps < 1 {
		return nil, errors.New("migrate: steps must be >= 1")
	}
	s, err := r.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer s.end()
	_, order, err := s.verify(ctx)
	if err != nil {
		return nil, err
	}
	known := map[uint64]Migration{}
	for _, m := range r.Migrations {
		known[m.Version] = m
	}
	var done []uint64
	for i := len(order) - 1; i >= 0 && len(done) < steps; i-- {
		m := known[order[i]]
		if err := s.setDirty(ctx, m.Version, "down"); err != nil {
			return done, err
		}
		if err := s.exec(ctx, m.Down); err != nil {
			return done, fmt.Errorf("down of %d (%s) failed, database left dirty: %w", m.Version, m.Name, err)
		}
		if _, err := s.c.ExecContext(ctx, "DELETE FROM schema_migration WHERE version=?", m.Version); err != nil {
			return done, err
		}
		if err := s.clearDirty(ctx); err != nil {
			return done, err
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// Status reports applied, pending and dirty state without changing anything.
func (r *Runner) Status(ctx context.Context) (Status, error) {
	s, err := r.begin(ctx)
	if err != nil {
		return Status{}, err
	}
	defer s.end()
	var st Status
	v, dir, d, err := s.dirty(ctx)
	if err != nil {
		return st, err
	}
	st.Dirty, st.DirtyVersion, st.DirtyDir = d, v, dir
	applied, order, err := s.applied(ctx)
	if err != nil {
		return st, err
	}
	st.Applied = order
	for _, m := range r.Migrations {
		if _, ok := applied[m.Version]; !ok {
			st.Pending = append(st.Pending, m.Version)
		}
	}
	return st, nil
}

// ClearDirty removes the dirty flag after an operator has repaired the database by hand. It never runs SQL from migrations.
func (r *Runner) ClearDirty(ctx context.Context) error {
	s, err := r.begin(ctx)
	if err != nil {
		return err
	}
	defer s.end()
	return s.clearDirty(ctx)
}

// SplitStatements splits a SQL script on top-level semicolons, honouring quotes, backticks and comments. No DELIMITER support (no stored programs).
func SplitStatements(script string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if st := strings.TrimSpace(b.String()); st != "" {
			out = append(out, st)
		}
		b.Reset()
	}
	rs := []rune(script)
	for i := 0; i < len(rs); i++ {
		c := rs[i]
		switch {
		case c == '\'' || c == '"' || c == '`':
			b.WriteRune(c)
			for i++; i < len(rs); i++ {
				b.WriteRune(rs[i])
				if rs[i] == '\\' && c != '`' && i+1 < len(rs) {
					i++
					b.WriteRune(rs[i])
					continue
				}
				if rs[i] == c {
					if i+1 < len(rs) && rs[i+1] == c { // doubled quote
						i++
						b.WriteRune(rs[i])
						continue
					}
					break
				}
			}
		case c == '-' && i+1 < len(rs) && rs[i+1] == '-' && (i+2 >= len(rs) || rs[i+2] == ' ' || rs[i+2] == '\t' || rs[i+2] == '\n' || rs[i+2] == '\r'), c == '#':
			for i < len(rs) && rs[i] != '\n' {
				i++
			}
			b.WriteRune('\n')
		case c == '/' && i+1 < len(rs) && rs[i+1] == '*':
			i += 2
			for i+1 < len(rs) && !(rs[i] == '*' && rs[i+1] == '/') {
				i++
			}
			i++
			b.WriteRune(' ')
		case c == ';':
			flush()
		default:
			b.WriteRune(c)
		}
	}
	flush()
	return out
}

var textDefaultRE = regexp.MustCompile(`(?is)\b(tinytext|text|mediumtext|longtext|tinyblob|blob|mediumblob|longblob|json)\b[^,()]*\bdefault\b`)

// Lint returns policy findings for one migration's scripts (AD-5): no DEFAULT on TEXT/BLOB/JSON columns,
// new tables must be InnoDB utf8mb4_0900_ai_ci, no DROP DATABASE.
func Lint(m Migration) []string {
	var out []string
	for _, st := range SplitStatements(m.Up) {
		up := strings.ToUpper(st)
		if textDefaultRE.MatchString(st) {
			out = append(out, fmt.Sprintf("%04d %s: TEXT/BLOB/JSON column must not have a DEFAULT", m.Version, m.Name))
		}
		if strings.HasPrefix(up, "CREATE TABLE") {
			if !strings.Contains(up, "ENGINE=INNODB") {
				out = append(out, fmt.Sprintf("%04d %s: CREATE TABLE must specify ENGINE=InnoDB", m.Version, m.Name))
			}
			if !strings.Contains(st, "utf8mb4_0900_ai_ci") {
				out = append(out, fmt.Sprintf("%04d %s: CREATE TABLE must use COLLATE=utf8mb4_0900_ai_ci", m.Version, m.Name))
			}
		}
	}
	for _, sc := range []string{m.Up, m.Down} {
		if regexp.MustCompile(`(?i)\bdrop\s+database\b`).MatchString(sc) {
			out = append(out, fmt.Sprintf("%04d %s: DROP DATABASE is forbidden in migrations", m.Version, m.Name))
		}
	}
	return out
}
