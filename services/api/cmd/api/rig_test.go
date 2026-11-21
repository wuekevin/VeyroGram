package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/testim"
)

// rig is a real stack for end-to-end tests: MySQL (migrated), Redis, the patched WuKongIM with before_send and msg.notify pointed at the real API's
// callback listener, the API process and the worker.
type rig struct {
	t     *testing.T
	d     *sql.DB
	dsn   string
	bin   string
	p     *proc
	im    *testim.IM
	base  string
	wsURL string
	extra []string
}

func newRig(t *testing.T, more ...string) *rig {
	t.Helper()
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	hookAddr := freeAddr(t)
	hookURL := "http://" + hookAddr + "/im/webhook/" + msgHookSecret
	im := testim.StartOpts(t, testim.Options{BeforeSendURL: hookURL, NotifyURL: hookURL})
	d, dsn, bin := migrated(t)
	extra := append([]string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef",
		"VG_IM_WEBHOOK_SECRET=" + msgHookSecret, "VG_IM_WEBHOOK_ADDR=" + hookAddr, "VG_MESSAGE_RATE_LIMIT=100000"}, more...)
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		for i := 1; i <= 8; i++ {
			r.C.Del(context.Background(), fmt.Sprintf("vg:deny:dev:1:%d", i))
		}
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	startWorker(t, bin, dsn, extra)
	return &rig{t: t, d: d, dsn: dsn, bin: bin, p: p, im: im, base: "http://" + p.pub, wsURL: fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"]), extra: extra}
}

// user signs a phone number in on a device and returns its access token, uid and IM token.
func (r *rig) user(phone, device string) (access, uid, imTok string) {
	t := r.t
	t.Helper()
	_, _ = r.d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE") // step over the 60 s resend cooldown
	re := regexp.MustCompile(`验证码 (\d{6})`)
	before := len(re.FindAllStringSubmatch(r.p.stderr.String(), -1))
	jpost(t, r.base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
	var code string
	eventually(t, "sms code", 3*time.Second, func() bool {
		all := re.FindAllStringSubmatch(r.p.stderr.String(), -1)
		if len(all) > before {
			code = all[len(all)-1][1]
			return true
		}
		return false
	})
	c, body := jpost(t, r.base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"app"}}`)
	if c != 200 {
		t.Fatalf("login: %d %v", c, body)
	}
	access, _ = body["access_token"].(string)
	c2, b := get(t, r.base+"/v1/im/token", "Bearer "+access)
	var o struct{ UID, Token string }
	if err := json.Unmarshal([]byte(strings.ReplaceAll(strings.ReplaceAll(b, `"uid"`, `"UID"`), `"token"`, `"Token"`)), &o); err != nil || c2 != 200 {
		t.Fatalf("im token: %d %s", c2, b)
	}
	return access, o.UID, o.Token
}

func (r *rig) ws(uid, tok, device string) *wsClient {
	c := dialWS(r.t, r.wsURL)
	if !c.connect(uid, tok, device, 0) {
		r.t.Fatalf("%s must connect to the IM", uid)
	}
	return c
}

func text(s string) map[string]any { return map[string]any{"type": 1, "content": s} }
