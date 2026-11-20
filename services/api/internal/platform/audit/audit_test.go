package audit_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"veyrogram/api/internal/platform/audit"
	"veyrogram/api/internal/platform/db"
	"veyrogram/api/internal/platform/logx"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/migrations"
)

func setup(t *testing.T) (*sql.DB, string) {
	t.Helper()
	d, dsn := testdb.NewWithDSN(t)
	ms, _ := migrate.Load(migrations.FS)
	if _, err := (&migrate.Runner{DB: d, Migrations: ms}).Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	return d, dsn
}

func fill(t *testing.T, l *audit.Log, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := l.Append(logx.WithTrace(context.Background(), "abcdef0123456789"), audit.Entry{Actor: "admin:1", Action: "user.ban", Target: fmt.Sprintf("user:%d", i), Detail: "reason=spam"}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAppendAndVerify(t *testing.T) {
	d, _ := setup(t)
	l := &audit.Log{DB: d}
	if h, err := l.Verify(context.Background()); err != nil || h.Rows != 0 {
		t.Fatalf("empty log must verify: %+v %v", h, err)
	}
	fill(t, l, 5)
	h, err := l.Verify(context.Background())
	if err != nil || h.Rows != 5 || len(h.Hash) != 64 {
		t.Fatalf("%+v %v", h, err)
	}
	if err := l.Append(context.Background(), audit.Entry{Action: "x"}); err == nil {
		t.Fatal("actor and action are required")
	}
	var trace string
	d.QueryRow("SELECT trace_id FROM audit_log LIMIT 1").Scan(&trace)
	if trace != "abcdef0123456789" {
		t.Fatalf("trace id must be recorded, got %q", trace)
	}
}

func TestConcurrentAppendsKeepAValidChain(t *testing.T) {
	d, _ := setup(t)
	l := &audit.Log{DB: d}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fill(t, l, 5)
		}()
	}
	wg.Wait()
	if h, err := l.Verify(context.Background()); err != nil || h.Rows != 100 {
		t.Fatalf("100 concurrent appends must form one valid chain: %+v %v", h, err)
	}
}

func TestVerifyDetectsEditsDeletionsAndReordering(t *testing.T) {
	cases := map[string]string{
		"edit a field":      "UPDATE audit_log SET actor='mallory' WHERE id=3",
		"edit the detail":   "UPDATE audit_log SET detail='nothing to see' WHERE id=2",
		"delete a middle":   "DELETE FROM audit_log WHERE id=3",
		"delete the first":  "DELETE FROM audit_log WHERE id=1",
		"swap timestamps":   "UPDATE audit_log SET created_at = created_at + INTERVAL 1 SECOND WHERE id=4",
		"rewrite prev link": "UPDATE audit_log SET prev_hash=hash WHERE id=5",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			d, _ := setup(t)
			l := &audit.Log{DB: d}
			fill(t, l, 6)
			if _, err := d.Exec(stmt); err != nil {
				t.Fatal(err)
			}
			if _, err := l.Verify(context.Background()); !errors.Is(err, audit.ErrTampered) {
				t.Fatalf("tampering (%s) must be detected, got %v", name, err)
			}
		})
	}
}

type memSink struct {
	mu   sync.Mutex
	puts map[string][]byte
}

func (m *memSink) Put(_ context.Context, k string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.puts == nil {
		m.puts = map[string][]byte{}
	}
	m.puts[k] = b
	return nil
}

func TestAnchorIsWrittenOffHostAndCatchesTailTruncationAndFullRewrite(t *testing.T) {
	d, _ := setup(t)
	now := time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)
	l := &audit.Log{DB: d, Now: func() time.Time { return now }}
	fill(t, l, 6)
	s := &memSink{}
	a, err := l.AnchorHead(context.Background(), s)
	if err != nil || a.Rows != 6 || a.HeadID != 6 {
		t.Fatalf("%+v %v", a, err)
	}
	body := s.puts["audit-anchor/2026-11-20.json"]
	var got audit.Anchor
	if json.Unmarshal(body, &got) != nil || got.HeadHash != a.HeadHash {
		t.Fatalf("anchor not stored under the dated key: %v", s.puts)
	}
	if err := l.CheckAnchor(context.Background(), a); err != nil {
		t.Fatalf("an untouched log matches its anchor: %v", err)
	}
	fill(t, l, 2) // growth after the anchor is fine
	if err := l.CheckAnchor(context.Background(), a); err != nil {
		t.Fatalf("appending after the anchor must stay valid: %v", err)
	}
	// 1) truncating the tail leaves a self-consistent chain (Verify passes) but the anchor exposes it
	d.Exec("DELETE FROM audit_log WHERE id > 4")
	if _, err := l.Verify(context.Background()); err != nil {
		t.Fatalf("premise: a truncated chain still verifies on its own: %v", err)
	}
	if err := l.CheckAnchor(context.Background(), a); !errors.Is(err, audit.ErrTampered) {
		t.Fatalf("tail truncation must be caught by the off-host anchor, got %v", err)
	}
}

func TestAnchorDetectsAFullyRecomputedRewrite(t *testing.T) {
	d, _ := setup(t)
	l := &audit.Log{DB: d}
	fill(t, l, 4)
	a, _ := l.AnchorHead(context.Background(), &memSink{})
	// an attacker with full database control rewrites row 2 AND recomputes every later hash: Verify alone would accept this chain
	d.Exec("DELETE FROM audit_log")
	d.Exec("UPDATE audit_head SET last_id=0, hash=UNHEX(REPEAT('00',32)) WHERE id=1")
	l2 := &audit.Log{DB: d, Now: func() time.Time { return time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) }}
	for i := 0; i < 4; i++ {
		l2.Append(context.Background(), audit.Entry{Actor: "forged", Action: "a", Target: fmt.Sprint(i)})
	}
	if _, err := l2.Verify(context.Background()); err != nil {
		t.Fatalf("premise: the forged chain is internally valid: %v", err)
	}
	if err := l.CheckAnchor(context.Background(), audit.Anchor{HeadID: a.HeadID + 4, HeadHash: a.HeadHash}); !errors.Is(err, audit.ErrTampered) {
		t.Fatalf("a forged-but-consistent chain must not match the anchored hash, got %v", err)
	}
}

func TestBrokenChainIsNeverAnchored(t *testing.T) {
	d, _ := setup(t)
	l := &audit.Log{DB: d}
	fill(t, l, 3)
	d.Exec("UPDATE audit_log SET actor='x' WHERE id=2")
	s := &memSink{}
	if _, err := l.AnchorHead(context.Background(), s); !errors.Is(err, audit.ErrTampered) || len(s.puts) != 0 {
		t.Fatalf("a tampered chain must not be anchored: %v puts=%d", err, len(s.puts))
	}
}

func TestHTTPSinkPutsWithTokenAndRequiresHTTPS(t *testing.T) {
	var method, path, auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path, auth = r.Method, r.URL.Path, r.Header.Get("Authorization")
		io.Copy(io.Discard, r.Body)
	}))
	defer srv.Close()
	if err := (audit.HTTPSink{Base: srv.URL + "/bucket", Token: "tok"}).Put(context.Background(), "audit-anchor/x.json", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if method != "PUT" || path != "/bucket/audit-anchor/x.json" || auth != "Bearer tok" {
		t.Fatalf("%s %s %s", method, path, auth)
	}
	if err := (audit.HTTPSink{Base: "http://anchors.example.com"}).Put(context.Background(), "k", nil); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("plain http to a remote host must be refused, got %v", err)
	}
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "no", 403) }))
	defer bad.Close()
	if err := (audit.HTTPSink{Base: bad.URL}).Put(context.Background(), "k", nil); err == nil {
		t.Fatal("a non-2xx answer must be an error so a failed anchor is noticed")
	}
}

// The application's database account must not be able to alter the audit trail. Needs an administrator DSN to create the account.
func TestApplicationAccountCannotRewriteTheAuditLog(t *testing.T) {
	adminDSN := os.Getenv("VG_TEST_MYSQL_ADMIN_DSN")
	if adminDSN == "" {
		t.Skip("VG_TEST_MYSQL_ADMIN_DSN not set: skipping database-privilege test")
	}
	d, dsn := setup(t)
	cfg, _ := mysql.ParseDSN(dsn)
	admin, err := sql.Open("mysql", adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() }) // registered first, so it runs after the DROP USER cleanup below
	user := "vg_app_" + strings.TrimPrefix(cfg.DBName, "veyrogram_it_")
	pass := "p" + strings.Repeat("x", 6) + cfg.DBName
	if _, err := admin.Exec(fmt.Sprintf("CREATE USER '%s'@'127.0.0.1' IDENTIFIED BY '%s'", user, pass)); err != nil {
		t.Fatalf("create app account: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(fmt.Sprintf("DROP USER '%s'@'127.0.0.1'", user)); err != nil {
			t.Errorf("could not drop test account %s: %v", user, err)
		}
	})
	granted, err := db.GrantApp(context.Background(), admin, cfg.DBName, user, "127.0.0.1")
	if err != nil || len(granted) < 6 {
		t.Fatalf("grant: %v %v", granted, err)
	}
	appCfg := *cfg
	appCfg.User, appCfg.Passwd = user, pass
	app, err := sql.Open("mysql", appCfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	l := &audit.Log{DB: app}
	fill(t, l, 3) // the app can append (INSERT on audit_log, UPDATE on audit_head)
	if h, err := (&audit.Log{DB: d}).Verify(context.Background()); err != nil || h.Rows != 3 {
		t.Fatalf("appends through the app account must work: %+v %v", h, err)
	}
	for _, stmt := range []string{
		"UPDATE audit_log SET actor='x' WHERE id=1",
		"DELETE FROM audit_log WHERE id=1",
		"DELETE FROM audit_log",
		"TRUNCATE TABLE audit_log",
		"DROP TABLE audit_log",
		"ALTER TABLE audit_log DROP COLUMN actor",
		"REPLACE INTO audit_log (id, tenant_id, actor, action, target, trace_id, created_at, prev_hash, hash) VALUES (1,1,'x','x','x','',NOW(),'0','0')",
	} {
		_, err := app.Exec(stmt)
		var me *mysql.MySQLError
		if !errors.As(err, &me) || (me.Number != 1142 && me.Number != 1143 && me.Number != 1044) {
			t.Errorf("%q must be refused by database privileges, got %v", stmt, err)
		}
	}
	if _, err := app.Exec("UPDATE outbox SET attempts=0"); err != nil {
		t.Errorf("the app must still be able to update ordinary tables: %v", err)
	}
	if _, err := app.Exec("CREATE TABLE nope (id INT)"); err == nil {
		t.Error("the app account must have no DDL rights")
	}
	if h, err := (&audit.Log{DB: d}).Verify(context.Background()); err != nil || h.Rows != 3 {
		t.Fatalf("the log must be untouched after all attempts: %+v %v", h, err)
	}
}

type memFetch map[string][]byte

func (m memFetch) Get(_ context.Context, k string) ([]byte, bool, error) {
	b, ok := m[k]
	return b, ok, nil
}

func TestCheckAnchorsComparesHistoricalAnchorsAndReportsGaps(t *testing.T) {
	d, _ := setup(t)
	ctx := context.Background()
	day := func(n int) time.Time { return time.Date(2026, 11, 20+n, 3, 0, 0, 0, time.UTC) }
	cur := day(0)
	l := &audit.Log{DB: d, Now: func() time.Time { return cur }}
	store := memFetch{}
	sink := &memSink{}
	for n, rows := range []int{3, 2, 4} { // days 20, 21, 22; day 23 gets no anchor
		cur = day(n)
		fill(t, l, rows)
		if _, err := l.AnchorHead(ctx, sink); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range sink.puts {
		store[k] = v
	}
	cur = day(3)
	r, err := l.CheckAnchors(ctx, store, 4)
	if err != nil || r.Checked != 3 || len(r.Missing) != 1 || r.Missing[0] != "2026-11-23" {
		t.Fatalf("report=%+v err=%v: want 3 checked and 2026-11-23 missing", r, err)
	}
	store["audit-anchor/2026-11-23.json"] = []byte("not json")
	if _, err := l.CheckAnchors(ctx, store, 4); err == nil {
		t.Fatal("a corrupt anchor must fail the check")
	}
	delete(store, "audit-anchor/2026-11-23.json")
	// An old anchor (day 21, covering rows 1..5) must still be satisfied while the log only grows; truncating to before it must be caught
	// even though today's anchor alone would also notice: remove only what day 21 vouched for.
	d.Exec("DELETE FROM audit_log WHERE id > 4")
	if _, err := l.CheckAnchors(ctx, store, 4); !errors.Is(err, audit.ErrTampered) {
		t.Fatalf("truncation below an anchored row must be reported as tampering, got %v", err)
	}
}
