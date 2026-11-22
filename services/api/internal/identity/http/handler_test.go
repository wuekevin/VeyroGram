package identityhttp

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/identity/adapters"
	"veyrogram/api/internal/identity/app"
	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/internal/platform/token"
	"veyrogram/api/migrations"
)

type fakeSMS struct {
	mu   sync.Mutex
	last map[string]string // phone -> last message
	n    int
	fail bool
}

func (f *fakeSMS) Send(_ context.Context, p domain.Phone, msg string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return fmt.Errorf("down")
	}
	f.last[string(p)] = msg
	f.n++
	return nil
}

var codeRE = regexp.MustCompile(`\b(\d{6})\b`)

func (f *fakeSMS) code(t *testing.T, phone string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := codeRE.FindStringSubmatch(f.last[phone])
	if m == nil {
		t.Fatalf("no code sent to %s", phone)
	}
	return m[1]
}

type fakeIM struct {
	mu    sync.Mutex
	calls []string
	fail  bool
}

func (f *fakeIM) UpdateToken(_ context.Context, uid, tok string, flag, level uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return fmt.Errorf("im down")
	}
	f.calls = append(f.calls, fmt.Sprintf("token %s %d %s", uid, flag, tok))
	return nil
}
func (f *fakeIM) DeviceQuit(_ context.Context, uid string, flag uint8) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return fmt.Errorf("im down")
	}
	f.calls = append(f.calls, fmt.Sprintf("quit %s %d", uid, flag))
	return nil
}
func (f *fakeIM) take() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.calls
	f.calls = nil
	return c
}

type env struct {
	t        *testing.T
	db       *sql.DB
	im       *fakeIM
	deny     *denylist.Redis
	degraded int
	authCfg  authn.Config
	h        http.Handler
	svc      *app.Service
	sms      *fakeSMS
	now      time.Time
	ring     *token.Ring
	query    func(q string, a ...any) int
}

func newEnv(t *testing.T, trusted ...string) *env {
	t.Helper()
	db := testdb.New(t)
	if _, err := (&migrate.Runner{DB: db, Migrations: mustLoad(t)}).Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	seed := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdeX"))
	ring, err := token.ParseRing("k1:" + seed)
	if err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, db: db, im: &fakeIM{}, sms: &fakeSMS{last: map[string]string{}}, now: time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC), ring: ring}
	e.svc = &app.Service{Store: &adapters.MySQLStore{DB: db}, SMS: e.sms, Keys: ring, HashKey: []byte("test-phone-hash-key-0123456789abcdef"), Policy: domain.DefaultPolicy(), Now: func() time.Time { return e.now }}
	e.svc.Tenant = uint64(time.Now().UnixNano()%1_000_000_000) + 10 // unique per test: deny-list keys in a shared Redis must not collide across fresh databases
	e.svc.IM, e.svc.IMKey = e.im, []byte("test-im-token-key-0123456789abcdef-xx")
	var dl denylist.List
	if addr := os.Getenv("VG_TEST_REDIS_ADDR"); addr != "" {
		r, err := denylist.Open(context.Background(), addr, "")
		if err != nil {
			t.Fatalf("redis: %v", err)
		}
		t.Cleanup(func() { r.Close() })
		e.deny, dl, e.svc.Deny = r, r, r
	}
	tr, err := clientip.ParseTrusted(trusted)
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	e.authCfg = authn.Config{Ring: ring, Tenant: e.svc.Tenant, Deny: dl, Now: func() time.Time { return e.now }, OnDegraded: func() { e.degraded++ }}
	(&Handler{Svc: e.svc, Trusted: tr, Auth: authn.Require(e.authCfg)}).Mount(mux)
	e.h = httpx.RequestID(mux)
	e.query = func(q string, a ...any) int {
		var n int
		if err := db.QueryRow(q, a...).Scan(&n); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		return n
	}
	return e
}

func mustLoad(t *testing.T) []migrate.Migration {
	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

type resp struct {
	Code int
	Body map[string]any
	Hdr  http.Header
	Raw  string
}

func (e *env) post(path, remote string, body any, hdr ...string) resp {
	b, _ := json.Marshal(body)
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	r.RemoteAddr = remote
	for i := 0; i+1 < len(hdr); i += 2 {
		r.Header.Set(hdr[i], hdr[i+1])
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	out := resp{Code: w.Code, Hdr: w.Header(), Raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &out.Body)
	return out
}

const (
	phoneA = "+8613800138000"
	phoneB = "+8613900139000"
	dev1   = "device-aaaaaaaaaaaaaaaa"
	dev2   = "device-bbbbbbbbbbbbbbbb"
)

func (e *env) sendCode(phone, dev, ip string) resp {
	return e.post("/v1/auth/sms/code", ip+":4000", map[string]any{"phone": phone, "device_id": dev})
}
func (e *env) login(phone, code, dev, ip string) resp {
	return e.post("/v1/auth/login", ip+":4000", map[string]any{"phone": phone, "code": code, "device": map[string]any{"id": dev, "class": "app", "name": "Pixel 8"}})
}

// registers phone on dev and returns the login response
func (e *env) register(phone, dev, ip string) resp {
	e.t.Helper()
	if r := e.sendCode(phone, dev, ip); r.Code != 202 {
		e.t.Fatalf("send code: %d %s", r.Code, r.Raw)
	}
	r := e.login(phone, e.sms.code(e.t, phone), dev, ip)
	if r.Code != 200 {
		e.t.Fatalf("login: %d %s", r.Code, r.Raw)
	}
	return r
}

func TestEndToEndRegisterLoginRefresh(t *testing.T) {
	e := newEnv(t)
	r := e.register(phoneA, dev1, "198.51.100.1")
	if r.Body["is_new_user"] != true || r.Body["token_type"] != "Bearer" || r.Hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("unexpected login response: %s", r.Raw)
	}
	claims, err := e.ring.Verify(r.Body["access_token"].(string), e.now)
	if err != nil || claims.Subject != r.Body["user_id"] || claims.Class != "app" || claims.Device != r.Body["device_id"] {
		t.Fatalf("access token claims: %+v %v", claims, err)
	}
	if claims.Expires-claims.IssuedAt != 900 {
		t.Fatalf("access token TTL must be 15 min, got %ds", claims.Expires-claims.IssuedAt)
	}
	if e.query("SELECT COUNT(*) FROM `user`") != 1 || e.query("SELECT COUNT(*) FROM device") != 1 {
		t.Fatal("login must create exactly one user and one device")
	}

	// refresh rotates: new pair, old token now dead
	rt1 := r.Body["refresh_token"].(string)
	r2 := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": rt1})
	if r2.Code != 200 || r2.Body["refresh_token"] == rt1 || r2.Body["user_id"] != r.Body["user_id"] {
		t.Fatalf("refresh: %d %s", r2.Code, r2.Raw)
	}
	// second login of the same user on another device is not a new user
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phoneA, dev2, "198.51.100.1")
	r3 := e.login(phoneA, e.sms.code(t, phoneA), dev2, "198.51.100.1")
	if r3.Code != 200 || r3.Body["is_new_user"] != false || r3.Body["user_id"] != r.Body["user_id"] || r3.Body["device_id"] == r.Body["device_id"] {
		t.Fatalf("second device: %s", r3.Raw)
	}
	if e.query("SELECT COUNT(*) FROM `user`") != 1 || e.query("SELECT COUNT(*) FROM device") != 2 {
		t.Fatal("same user, two devices expected")
	}
}

func TestNothingSensitiveStoredInClear(t *testing.T) {
	e := newEnv(t)
	r := e.register(phoneA, dev1, "198.51.100.1")
	code := e.sms.code(t, phoneA)
	rt := r.Body["refresh_token"].(string)
	for _, q := range []struct{ name, sql string }{
		{"phone in user", "SELECT COUNT(*) FROM `user` WHERE phone_hash LIKE '%13800138000%'"},
		{"phone in sms_code", "SELECT COUNT(*) FROM sms_code WHERE code_hash LIKE '%13800138000%' OR code_hash='" + code + "'"},
		{"phone in login_event", "SELECT COUNT(*) FROM login_event WHERE phone_hash LIKE '%13800138000%'"},
		{"refresh in clear", "SELECT COUNT(*) FROM refresh_token WHERE token_hash='" + rt + "'"},
	} {
		if e.query(q.sql) != 0 {
			t.Errorf("%s: plaintext found", q.name)
		}
	}
	if e.query("SELECT COUNT(*) FROM `user` WHERE phone_last4='8000'") != 1 {
		t.Error("only the last four digits are kept")
	}
}

func TestCodesAreRandomOneTimeAndNeverFixed(t *testing.T) {
	e := newEnv(t)
	seen := map[string]bool{}
	for i := 0; i < 12; i++ {
		e.now = e.now.Add(2 * time.Minute) // clear cooldown, stay under hourly cap by using distinct phones
		phone := fmt.Sprintf("+86138001380%02d", i)
		if r := e.sendCode(phone, fmt.Sprintf("device-%016d", i), fmt.Sprintf("198.51.100.%d", i+1)); r.Code != 202 {
			t.Fatalf("send: %d %s", r.Code, r.Raw)
		}
		c := e.sms.code(t, phone)
		if c == "123456" && seen[c] {
			t.Fatal("fixed code detected")
		}
		seen[c] = true
	}
	if len(seen) < 8 {
		t.Fatalf("codes are not random enough: only %d distinct of 12", len(seen))
	}
	// replay: a code that worked once never works again
	e.register(phoneA, dev1, "203.0.113.5")
	used := e.sms.code(t, phoneA)
	if r := e.login(phoneA, used, dev1, "203.0.113.5"); r.Code != 401 || r.Body["code"] != "invalid_code" {
		t.Fatalf("code replay must fail: %d %s", r.Code, r.Raw)
	}
	// a newer code supersedes the older one
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phoneB, dev1, "203.0.113.6")
	first := e.sms.code(t, phoneB)
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phoneB, dev1, "203.0.113.6")
	second := e.sms.code(t, phoneB)
	if first != second {
		if r := e.login(phoneB, first, dev1, "203.0.113.6"); r.Code != 401 {
			t.Fatalf("superseded code must be dead: %d", r.Code)
		}
	}
}

func TestCodeAndRefreshExpiry(t *testing.T) {
	e := newEnv(t)
	e.sendCode(phoneA, dev1, "198.51.100.1")
	c := e.sms.code(t, phoneA)
	e.now = e.now.Add(5*time.Minute + time.Second)
	if r := e.login(phoneA, c, dev1, "198.51.100.1"); r.Code != 401 {
		t.Fatalf("expired code accepted: %d", r.Code)
	}
	r := e.register(phoneB, dev1, "198.51.100.2")
	e.now = e.now.Add(31 * 24 * time.Hour)
	if x := e.post("/v1/auth/refresh", "198.51.100.2:1", map[string]any{"refresh_token": r.Body["refresh_token"]}); x.Code != 401 {
		t.Fatalf("expired refresh token accepted: %d", x.Code)
	}
}

func TestNoAccountEnumeration(t *testing.T) {
	e := newEnv(t)
	e.register(phoneA, dev1, "198.51.100.1")
	e.now = e.now.Add(2 * time.Minute)
	known := e.sendCode(phoneA, dev2, "198.51.100.2")
	unknown := e.sendCode(phoneB, dev2, "198.51.100.3")
	if known.Code != unknown.Code || known.Raw != unknown.Raw {
		t.Fatalf("code request differs by registration: %d %q vs %d %q", known.Code, known.Raw, unknown.Code, unknown.Raw)
	}
	wk := e.login(phoneA, "000000", dev2, "198.51.100.2")
	wu := e.login(phoneB, "000000", dev2, "198.51.100.3")
	strip := func(s string) string { return regexp.MustCompile(`"trace_id":"[^"]*"`).ReplaceAllString(s, "") }
	if wk.Code != wu.Code || strip(wk.Raw) != strip(wu.Raw) {
		t.Fatalf("wrong-code answer differs by registration: %s vs %s", wk.Raw, wu.Raw)
	}
	nocode := e.login("+8613700137000", "000000", dev2, "198.51.100.4")
	if nocode.Code != wk.Code || strip(nocode.Raw) != strip(wk.Raw) {
		t.Fatalf("never-requested number must look the same: %s", nocode.Raw)
	}
}

func TestSendLimitsAndRiskEvents(t *testing.T) {
	e := newEnv(t)
	if r := e.sendCode(phoneA, dev1, "198.51.100.1"); r.Code != 202 {
		t.Fatal(r.Raw)
	}
	r := e.sendCode(phoneA, dev1, "198.51.100.1") // inside the 60 s cooldown
	if r.Code != 429 || r.Body["code"] != "rate_limited" || r.Hdr.Get("Retry-After") == "" {
		t.Fatalf("cooldown: %d %s hdr=%v", r.Code, r.Raw, r.Hdr)
	}
	if e.query("SELECT COUNT(*) FROM login_event WHERE kind='sms_send' AND result='rate_limited'") != 1 {
		t.Fatal("a 429 must write a risk event")
	}
	if e.sms.n != 1 {
		t.Fatalf("no SMS may be sent when limited, sent %d", e.sms.n)
	}
	// per-phone hourly cap (5): spread over time and IPs
	for i := 0; i < 4; i++ {
		e.now = e.now.Add(61 * time.Second)
		if r := e.sendCode(phoneA, fmt.Sprintf("device-%016d", i), fmt.Sprintf("198.51.100.%d", 10+i)); r.Code != 202 {
			t.Fatalf("send %d: %d", i, r.Code)
		}
	}
	e.now = e.now.Add(61 * time.Second)
	if r := e.sendCode(phoneA, dev2, "198.51.100.40"); r.Code != 429 {
		t.Fatalf("6th send to one number within an hour must be limited, got %d", r.Code)
	}
}

func TestSMSPumpingLimitsPerIPAndGlobal(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 20; i++ { // one IP walking through many numbers
		r := e.sendCode(fmt.Sprintf("+861380013%04d", i), fmt.Sprintf("device-%016d", i), "203.0.113.50")
		if r.Code != 202 {
			t.Fatalf("send %d: %d %s", i, r.Code, r.Raw)
		}
	}
	if r := e.sendCode("+8613800139999", "device-ffffffffffffffff", "203.0.113.50"); r.Code != 429 {
		t.Fatalf("21st send from one IP must be limited, got %d", r.Code)
	}
	// same device id reused across numbers
	e2 := newEnv(t)
	for i := 0; i < 10; i++ {
		if r := e2.sendCode(fmt.Sprintf("+861380014%04d", i), dev1, fmt.Sprintf("198.51.100.%d", i+1)); r.Code != 202 {
			t.Fatalf("send %d: %d", i, r.Code)
		}
	}
	if r := e2.sendCode("+8613800149999", dev1, "198.51.100.99"); r.Code != 429 {
		t.Fatalf("11th send from one device must be limited, got %d", r.Code)
	}
	// global breaker
	e3 := newEnv(t)
	e3.svc.Policy.SendGlobal = 3
	for i := 0; i < 3; i++ {
		e3.sendCode(fmt.Sprintf("+861380015%04d", i), fmt.Sprintf("device-%016d", i), fmt.Sprintf("198.51.100.%d", i+1))
	}
	if r := e3.sendCode("+8613800159999", "device-eeeeeeeeeeeeeeee", "198.51.100.77"); r.Code != 429 {
		t.Fatalf("global breaker must trip, got %d", r.Code)
	}
}

func TestBruteForceIsStopped(t *testing.T) {
	e := newEnv(t)
	e.sendCode(phoneA, dev1, "198.51.100.1")
	good := e.sms.code(t, phoneA)
	wrong := "000000"
	if good == wrong {
		wrong = "111111"
	}
	for i := 0; i < 5; i++ { // 5 wrong guesses kill that code
		if r := e.login(phoneA, wrong, dev1, "198.51.100.1"); r.Code != 401 {
			t.Fatalf("guess %d: %d", i, r.Code)
		}
	}
	// the phone is now locked: even the correct code is refused, with Retry-After, and nothing is consumed
	r := e.login(phoneA, good, dev1, "198.51.100.1")
	if r.Code != 429 || r.Body["code"] != "too_many_attempts" || r.Hdr.Get("Retry-After") == "" {
		t.Fatalf("lockout expected: %d %s", r.Code, r.Raw)
	}
	if e.query("SELECT COUNT(*) FROM login_event WHERE kind='login' AND result='fail'") != 5 || e.query("SELECT COUNT(*) FROM login_event WHERE kind='login' AND result='locked'") != 1 {
		t.Fatal("login_event must record the failures and the lockout")
	}
	// another IP/device does not unlock the phone
	if r := e.login(phoneA, good, dev2, "198.51.100.9"); r.Code != 429 {
		t.Fatalf("phone lockout must hold across IP/device: %d", r.Code)
	}
	// after the window the lock lifts, but the old code is dead (attempts exhausted): a fresh code works
	e.now = e.now.Add(16 * time.Minute)
	if r := e.login(phoneA, good, dev1, "198.51.100.1"); r.Code != 401 {
		t.Fatalf("exhausted/expired code must stay dead: %d", r.Code)
	}
	e.sendCode(phoneA, dev1, "198.51.100.1")
	if r := e.login(phoneA, e.sms.code(t, phoneA), dev1, "198.51.100.1"); r.Code != 200 {
		t.Fatalf("a fresh code must work after the lock: %d %s", r.Code, r.Raw)
	}
}

func TestPerIPAndPerDeviceFailureLockouts(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < 10; i++ { // one device guessing against many numbers
		e.login(fmt.Sprintf("+861380016%04d", i), "000000", dev1, fmt.Sprintf("198.51.100.%d", i+1))
	}
	if r := e.login("+8613800169999", "000000", dev1, "198.51.100.200"); r.Code != 429 {
		t.Fatalf("device lockout expected: %d", r.Code)
	}
	e2 := newEnv(t)
	for i := 0; i < 30; i++ { // one IP, fresh device each time
		e2.login(fmt.Sprintf("+861380017%04d", i), "000000", fmt.Sprintf("device-%016d", i), "198.51.100.1")
	}
	if r := e2.login("+8613800179999", "000000", "device-zzzzzzzzzzzzzzzz", "198.51.100.1"); r.Code != 429 {
		t.Fatalf("IP lockout expected: %d", r.Code)
	}
}

func TestForwardedForCannotEvadeLimits(t *testing.T) {
	// untrusted peer: rotating X-Forwarded-For must not give a fresh per-IP budget
	e := newEnv(t)
	for i := 0; i < 20; i++ {
		e.post("/v1/auth/sms/code", "203.0.113.50:4000", map[string]any{"phone": fmt.Sprintf("+861380018%04d", i), "device_id": fmt.Sprintf("device-%016d", i)}, "X-Forwarded-For", fmt.Sprintf("9.9.9.%d", i))
	}
	r := e.post("/v1/auth/sms/code", "203.0.113.50:4000", map[string]any{"phone": "+8613800189999", "device_id": "device-yyyyyyyyyyyyyyyy"}, "X-Forwarded-For", "8.8.8.8")
	if r.Code != 429 {
		t.Fatalf("spoofed XFF must not reset the per-IP limit, got %d", r.Code)
	}
	if e.query("SELECT COUNT(*) FROM login_event WHERE ip LIKE '9.9.9.%' OR ip='8.8.8.8'") != 0 {
		t.Fatal("spoofed addresses must never be recorded as the client IP")
	}
	// trusted proxy: the real client behind it is what is counted, right-most untrusted hop
	e2 := newEnv(t, "10.0.0.0/8")
	for i := 0; i < 20; i++ {
		e2.post("/v1/auth/sms/code", "10.1.1.1:80", map[string]any{"phone": fmt.Sprintf("+861380019%04d", i), "device_id": fmt.Sprintf("device-%016d", i)}, "X-Forwarded-For", fmt.Sprintf("6.6.6.%d, 198.51.100.77", i))
	}
	r = e2.post("/v1/auth/sms/code", "10.1.1.1:80", map[string]any{"phone": "+8613800199999", "device_id": "device-xxxxxxxxxxxxxxxx"}, "X-Forwarded-For", "5.5.5.5, 198.51.100.77")
	if r.Code != 429 {
		t.Fatalf("client-prepended XFF entries must not win behind a trusted proxy, got %d", r.Code)
	}
	if e2.query("SELECT COUNT(*) FROM login_event WHERE ip='198.51.100.77' AND result='ok'") != 20 {
		t.Fatal("the real client IP must be recorded")
	}
}

func TestRefreshReuseRevokesDevice(t *testing.T) {
	e := newEnv(t)
	r := e.register(phoneA, dev1, "198.51.100.1")
	rt1 := r.Body["refresh_token"].(string)
	r2 := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": rt1})
	rt2 := r2.Body["refresh_token"].(string)
	// attacker (or a stale client) replays the first token
	if x := e.post("/v1/auth/refresh", "203.0.113.9:1", map[string]any{"refresh_token": rt1}); x.Code != 401 {
		t.Fatalf("reuse must be rejected: %d", x.Code)
	}
	// the legitimate newest token is now dead too: the whole device is revoked
	if x := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": rt2}); x.Code != 401 {
		t.Fatalf("device must be revoked after reuse, newest token still works: %d", x.Code)
	}
	if e.query("SELECT COUNT(*) FROM device WHERE revoked_at IS NOT NULL") != 1 || e.query("SELECT COUNT(*) FROM login_event WHERE kind='refresh' AND result='reuse'") != 1 {
		t.Fatal("device must be marked revoked and the reuse event recorded")
	}
	// logging in again with a fresh code starts a NEW device (the revoked id must stay dead)
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phoneA, dev1, "198.51.100.1")
	if x := e.login(phoneA, e.sms.code(t, phoneA), dev1, "198.51.100.1"); x.Code != 200 {
		t.Fatalf("re-login after revocation: %d %s", x.Code, x.Raw)
	}
	if e.query("SELECT COUNT(*) FROM device WHERE revoked_at IS NOT NULL") != 1 || e.query("SELECT COUNT(*) FROM device") != 2 {
		t.Fatal("a revoked device stays revoked forever; re-login must create a new device row")
	}
}

func TestConcurrentCodeUseSucceedsOnce(t *testing.T) {
	e := newEnv(t)
	e.sendCode(phoneA, dev1, "198.51.100.1")
	code := e.sms.code(t, phoneA)
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := e.login(phoneA, code, dev1, "198.51.100.1"); r.Code == 200 {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("a one-time code must work exactly once under concurrency, got %d successes", ok)
	}
	if e.query("SELECT COUNT(*) FROM `user`") != 1 || e.query("SELECT COUNT(*) FROM refresh_token WHERE status='active'") != 1 {
		t.Fatal("exactly one user and one live refresh token expected")
	}
}

func TestDisabledAccountAndSMSOutage(t *testing.T) {
	e := newEnv(t)
	e.register(phoneA, dev1, "198.51.100.1")
	e.svc.Store.(*adapters.MySQLStore).DB.Exec("UPDATE `user` SET status='disabled'")
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phoneA, dev1, "198.51.100.1")
	if r := e.login(phoneA, e.sms.code(t, phoneA), dev1, "198.51.100.1"); r.Code != 403 || r.Body["code"] != "account_disabled" {
		t.Fatalf("disabled: %d %s", r.Code, r.Raw)
	}
	e.sms.fail = true
	e.now = e.now.Add(2 * time.Minute)
	if r := e.sendCode(phoneB, dev1, "198.51.100.2"); r.Code != 503 || strings.Contains(r.Raw, "down") {
		t.Fatalf("sms outage must be a clean 503: %d %s", r.Code, r.Raw)
	}
}

func TestInputValidation(t *testing.T) {
	e := newEnv(t)
	for name, tc := range map[string]struct {
		path string
		body any
	}{
		"bad phone":       {"/v1/auth/sms/code", map[string]any{"phone": "13800138000", "device_id": dev1}},
		"bad device":      {"/v1/auth/sms/code", map[string]any{"phone": phoneA, "device_id": "x"}},
		"unknown field":   {"/v1/auth/sms/code", map[string]any{"phone": phoneA, "device_id": dev1, "admin": true}},
		"short code":      {"/v1/auth/login", map[string]any{"phone": phoneA, "code": "12", "device": map[string]any{"id": dev1, "class": "app"}}},
		"non-digit code":  {"/v1/auth/login", map[string]any{"phone": phoneA, "code": "12345a", "device": map[string]any{"id": dev1, "class": "app"}}},
		"bad class":       {"/v1/auth/login", map[string]any{"phone": phoneA, "code": "123456", "device": map[string]any{"id": dev1, "class": "tv"}}},
		"empty refresh":   {"/v1/auth/refresh", map[string]any{"refresh_token": ""}},
		"garbage refresh": {"/v1/auth/refresh", map[string]any{"refresh_token": "rt_" + strings.Repeat("a", 43)}},
	} {
		r := e.post(tc.path, "198.51.100.1:1", tc.body)
		want := 400
		if name == "garbage refresh" || name == "empty refresh" {
			want = 401
		}
		if r.Code != want {
			t.Errorf("%s: got %d %s", name, r.Code, r.Raw)
		}
	}
	if r := e.post("/v1/auth/login", "198.51.100.1:1", map[string]any{}); r.Code != 400 {
		t.Errorf("empty body: %d", r.Code)
	}
}
