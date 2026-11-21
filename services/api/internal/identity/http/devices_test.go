package identityhttp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/identity/app"
	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/outbox"
)

func (e *env) do(method, path, tok string) resp {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = "198.51.100.1:5000"
	if tok != "" {
		r.Header.Set("Authorization", "Bearer "+tok)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	out := resp{Code: w.Code, Hdr: w.Header(), Raw: w.Body.String()}
	_ = json.Unmarshal(w.Body.Bytes(), &out.Body)
	return out
}

func needRedis(t *testing.T, e *env) {
	if e.deny == nil {
		t.Skip("VG_TEST_REDIS_ADDR not set: skipping Redis integration test")
	}
}

// registerOn logs phone in on a device of the given class and returns (access, refresh, deviceID).
func (e *env) registerOn(phone, dev, class string) (string, string, string) {
	e.t.Helper()
	e.now = e.now.Add(2 * time.Minute)
	e.sendCode(phone, dev, "198.51.100.1")
	r := e.post("/v1/auth/login", "198.51.100.1:4000", map[string]any{"phone": phone, "code": e.sms.code(e.t, phone), "device": map[string]any{"id": dev, "class": class, "name": class + "-" + dev[7:9]}})
	if r.Code != 200 {
		e.t.Fatalf("login: %d %s", r.Code, r.Raw)
	}
	return r.Body["access_token"].(string), r.Body["refresh_token"].(string), r.Body["device_id"].(string)
}

func (e *env) runWorker() {
	e.t.Helper()
	w := &outbox.Worker{DB: e.db, Now: func() time.Time { return e.now }, Handlers: map[string]outbox.Handler{domain.TopicIMSync: func(ctx context.Context, ev outbox.Event) error {
		var p struct{ UID, Class string }
		if err := json.Unmarshal(ev.Payload, &p); err != nil {
			return err
		}
		return e.svc.SyncIM(ctx, p.UID, domain.DeviceClass(p.Class))
	}}}
	for i := 0; i < 5; i++ {
		if n, err := w.RunOnce(context.Background()); err != nil {
			e.t.Fatal(err)
		} else if n == 0 {
			return
		}
	}
}

func TestRevokeOneOfTwoDevicesTakesEffectImmediately(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, r1, d1 := e.registerOn(phoneA, dev1, "app")
	a2, r2, d2 := e.registerOn(phoneA, dev2, "web")

	list := e.do("GET", "/v1/devices", a1)
	items, _ := list.Body["items"].([]any)
	if list.Code != 200 || len(items) != 2 {
		t.Fatalf("list: %d %s", list.Code, list.Raw)
	}
	for _, it := range items {
		m := it.(map[string]any)
		if (m["id"] == d1) != (m["current"] == true) {
			t.Fatalf("exactly the calling device must be flagged current: %s", list.Raw)
		}
	}
	if x := e.do("GET", "/v1/im/token", a2); x.Code != 200 {
		t.Fatalf("device 2 works before revocation: %d %s", x.Code, x.Raw)
	}

	start := time.Now()
	if x := e.do("DELETE", "/v1/devices/"+d2, a1); x.Code != 204 {
		t.Fatalf("revoke: %d %s", x.Code, x.Raw)
	}
	if x := e.do("GET", "/v1/devices", a2); x.Code != 401 || x.Body["code"] != "invalid_token" {
		t.Fatalf("revoked device's access token must stop working: %d %s", x.Code, x.Raw)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("revocation took %s to bite; target is 60 s", d)
	}
	t.Logf("revocation effective %s after the request started (target <= 60 s)", time.Since(start))
	if x := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": r2}); x.Code != 401 {
		t.Fatalf("revoked device's refresh token must be dead: %d", x.Code)
	}
	if x := e.do("GET", "/v1/devices", a1); x.Code != 200 {
		t.Fatalf("the other device must be unaffected: %d", x.Code)
	}
	if x := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": r1}); x.Code != 200 {
		t.Fatalf("the other device can still refresh: %d", x.Code)
	}
	if x := e.do("DELETE", "/v1/devices/"+d2, a1); x.Code != 204 {
		t.Fatalf("revoking twice must be idempotent: %d", x.Code)
	}
	if left := e.do("GET", "/v1/devices", a1).Body["items"].([]any); len(left) != 1 {
		t.Fatalf("revoked device must disappear from the list: %v", left)
	}
}

func TestCannotRevokeOthersDevicesOrUnknownIDs(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, _, _ := e.registerOn(phoneA, dev1, "app")
	bTok, _, bDev := e.registerOn(phoneB, dev2, "app")
	if x := e.do("DELETE", "/v1/devices/"+bDev, a1); x.Code != 404 {
		t.Fatalf("someone else's device must look nonexistent: %d", x.Code)
	}
	if x := e.do("GET", "/v1/devices", bTok); x.Code != 200 {
		t.Fatalf("victim's token must be untouched: %d", x.Code)
	}
	for _, id := range []string{"999999", "abc", "-1"} {
		if x := e.do("DELETE", "/v1/devices/"+id, a1); x.Code != 404 {
			t.Errorf("id %q: %d", id, x.Code)
		}
	}
	for _, p := range [][2]string{{"GET", "/v1/devices"}, {"DELETE", "/v1/devices/1"}, {"GET", "/v1/im/token"}, {"POST", "/v1/auth/logout"}} {
		if x := e.do(p[0], p[1], ""); x.Code != 401 {
			t.Errorf("%s %s without a token: %d", p[0], p[1], x.Code)
		}
	}
}

func TestLogoutRevokesCurrentDeviceEverywhere(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, r1, _ := e.registerOn(phoneA, dev1, "app")
	if x := e.do("POST", "/v1/auth/logout", a1); x.Code != 204 {
		t.Fatalf("logout: %d %s", x.Code, x.Raw)
	}
	if x := e.do("GET", "/v1/devices", a1); x.Code != 401 {
		t.Fatalf("access token after logout: %d", x.Code)
	}
	if x := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": r1}); x.Code != 401 {
		t.Fatalf("refresh after logout: %d", x.Code)
	}
	if x := e.do("GET", "/v1/im/token", a1); x.Code != 401 {
		t.Fatalf("IM token after logout: %d", x.Code)
	}
}

func TestSameClassRevocationRotatesIMTokenAndOtherDeviceRecovers(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, _, _ := e.registerOn(phoneA, dev1, "app")
	a3, _, d3 := e.registerOn(phoneA, "device-cccccccccccccccc", "app") // same class as device 1
	before := e.do("GET", "/v1/im/token", a1)
	tokBefore := before.Body["token"].(string)
	if before.Code != 200 || before.Body["device_flag"].(float64) != 0 {
		t.Fatalf("im token: %d %s", before.Code, before.Raw)
	}
	if t3 := e.do("GET", "/v1/im/token", a3).Body["token"]; t3 != tokBefore {
		t.Fatal("IM tokens are per (user, class): both app devices share one token (the WuKongIM limitation this story works around)")
	}
	e.im.take()

	if x := e.do("DELETE", "/v1/devices/"+d3, a1); x.Code != 204 {
		t.Fatalf("revoke: %d", x.Code)
	}
	e.runWorker() // the outbox worker kicks the class's sessions and registers the rotated token
	calls := e.im.take()
	if len(calls) != 2 || !strings.HasPrefix(calls[0], "quit ") || !strings.HasSuffix(calls[0], " 0") || !strings.HasPrefix(calls[1], "token ") {
		t.Fatalf("worker must kick the class then register the new token, got %v", calls)
	}
	after := e.do("GET", "/v1/im/token", a1) // the surviving device reconnects and re-fetches: no permanent disconnect
	tokAfter, _ := after.Body["token"].(string)
	if after.Code != 200 || tokAfter == "" || tokAfter == tokBefore {
		t.Fatalf("the class token must rotate and the other device must be able to fetch the new one: %d %s", after.Code, after.Raw)
	}
	if !strings.HasSuffix(calls[1], tokAfter) {
		t.Fatal("the worker and the API must derive the same token")
	}
	if x := e.do("GET", "/v1/im/token", a3); x.Code != 401 {
		t.Fatalf("the revoked device must not obtain an IM token: %d", x.Code)
	}
	// replaying the worker event is harmless (idempotent)
	if err := e.svc.SyncIM(context.Background(), after.Body["uid"].(string), domain.ClassApp); err != nil {
		t.Fatal(err)
	}
}

func TestIMTokenDoesNotDependOnRedisForRevokedDevices(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, _, d1 := e.registerOn(phoneA, dev1, "app")
	e.do("DELETE", "/v1/devices/"+d1, a1)
	// simulate Redis loss: the deny-list is unreachable, so the token is accepted (degraded) ...
	dead, _ := denylist.Open(context.Background(), "127.0.0.1:1", "")
	e.authCfg.Deny = dead
	e.svc.Deny = dead
	mux := http.NewServeMux()
	(&Handler{Svc: e.svc, Auth: authn.Require(e.authCfg)}).Mount(mux)
	e.h = mux
	if x := e.do("GET", "/v1/devices", a1); x.Code != 200 || e.degraded == 0 {
		t.Fatalf("with the deny-list down a still-valid token is served and the degradation reported: %d degraded=%d", x.Code, e.degraded)
	}
	// ... but the IM token and refresh paths consult the database, so a revoked device still cannot connect to the IM
	if x := e.do("GET", "/v1/im/token", a1); x.Code != 404 {
		t.Fatalf("revoked device must not get an IM token even when Redis is down: %d %s", x.Code, x.Raw)
	}
}

func TestRefreshReuseAlsoKillsAccessTokensAndRotatesIM(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, r1, _ := e.registerOn(phoneA, dev1, "app")
	r2 := e.post("/v1/auth/refresh", "198.51.100.1:1", map[string]any{"refresh_token": r1})
	if r2.Code != 200 {
		t.Fatal(r2.Raw)
	}
	if x := e.do("GET", "/v1/devices", a1); x.Code != 200 {
		t.Fatalf("access token valid before reuse: %d", x.Code)
	}
	e.post("/v1/auth/refresh", "203.0.113.9:1", map[string]any{"refresh_token": r1}) // replay of the old token
	if x := e.do("GET", "/v1/devices", a1); x.Code != 401 {
		t.Fatalf("after reuse detection the device's access tokens must die immediately: %d", x.Code)
	}
	if x := e.do("GET", "/v1/devices", r2.Body["access_token"].(string)); x.Code != 401 {
		t.Fatalf("including the newest one: %d", x.Code)
	}
	if e.query("SELECT COUNT(*) FROM outbox WHERE topic='identity.im_sync' AND status='pending'") != 1 {
		t.Fatal("reuse must queue an IM token rotation")
	}
}

func TestStaleTokensOfARevokedDeviceStayDeadAfterReLogin(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, _, d1 := e.registerOn(phoneA, dev1, "app")
	e.do("DELETE", "/v1/devices/"+d1, a1)
	aNew, _, dNew := e.registerOn(phoneA, dev1, "app") // same physical device logs in again
	if dNew == d1 {
		t.Fatal("re-login must issue a new device id")
	}
	if x := e.do("GET", "/v1/devices", a1); x.Code != 401 {
		t.Fatalf("the old access token must stay dead: %d", x.Code)
	}
	if x := e.do("GET", "/v1/devices", aNew); x.Code != 200 {
		t.Fatalf("the new session works: %d", x.Code)
	}
}

func TestIMOutageIsReportedCleanly(t *testing.T) {
	e := newEnv(t)
	needRedis(t, e)
	a1, _, _ := e.registerOn(phoneA, dev1, "app")
	e.im.fail = true
	if x := e.do("GET", "/v1/im/token", a1); x.Code != 503 || x.Body["code"] != "im_unavailable" {
		t.Fatalf("%d %s", x.Code, x.Raw)
	}
}

var _ = app.Tokens{}
