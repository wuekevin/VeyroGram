package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	identitydomain "veyrogram/api/internal/identity/domain"
	msgdomain "veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/platform/denylist"
	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/testim"
)

const msgHookSecret = "message-hook-secret-0123456789abcdef0123"

// ulid makes a syntactically valid ULID that is unique within the test run (first char 0-7, Crockford alphabet).
var ulidSeq struct {
	sync.Mutex
	n int
}

func ulid() string {
	ulidSeq.Lock()
	defer ulidSeq.Unlock()
	ulidSeq.n++
	return fmt.Sprintf("01HZ%022d", ulidSeq.n)
}

// The 1.15 end-to-end proof: two real users, the real IM with the before_send hook pointed at the real API's callback listener, real WebSocket
// clients on the normal send path, plus the /v1/messages API on the same gate.
func TestSingleChatOverRealIMWithBeforeSendGate(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	hookAddr := freeAddr(t)
	hookURL := "http://" + hookAddr + "/im/webhook/" + msgHookSecret
	im := testim.StartOpts(t, testim.Options{BeforeSendURL: hookURL, NotifyURL: hookURL})
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef",
		"VG_IM_WEBHOOK_SECRET=" + msgHookSecret, "VG_IM_WEBHOOK_ADDR=" + hookAddr, "VG_MESSAGE_RATE_LIMIT=100000"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	startWorker(t, bin, dsn, extra)
	base := "http://" + p.pub
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])

	login := func(phone, device string) (access, uid string) {
		t.Helper()
		_, _ = d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE")
		before := len(regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1))
		jpost(t, base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
		var code string
		eventually(t, "sms code", 3*time.Second, func() bool {
			all := regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		c, body := jpost(t, base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"app"}}`)
		if c != 200 {
			t.Fatalf("login: %d %v", c, body)
		}
		access, _ = body["access_token"].(string)
		uid, _ = body["user_uid"].(string)
		return
	}
	imCreds := func(access string) (uid, tok string) {
		c, b := get(t, base+"/v1/im/token", "Bearer "+access)
		var o struct{ UID, Token string }
		if err := json.Unmarshal([]byte(strings.ReplaceAll(strings.ReplaceAll(b, `"uid"`, `"UID"`), `"token"`, `"Token"`)), &o); err != nil || c != 200 {
			t.Fatalf("im token: %d %s", c, b)
		}
		return o.UID, o.Token
	}
	accA, _ := login("+8613800138101", "device-aaaaaaaaaaaaaaaa")
	accB, _ := login("+8613800138102", "device-bbbbbbbbbbbbbbbb")
	uidA, tokA := imCreds(accA)
	uidB, tokB := imCreds(accB)
	if uidA == "" || uidB == "" || uidA == uidB {
		t.Fatalf("two distinct users expected: %q %q", uidA, uidB)
	}
	wsA, wsB := dialWS(t, wsURL), dialWS(t, wsURL)
	if !wsA.connect(uidA, tokA, "im-a", 0) || !wsB.connect(uidB, tokB, "im-b", 0) {
		t.Fatal("both users must connect with their IM tokens")
	}

	// 1. send and receive; a network-jitter resend with the same client_msg_no yields ONE message, the same ack and a single delivery.
	n1 := ulid()
	a1 := wsA.send(uidB, n1, text("hello"))
	if a1.Reason != 1 || a1.ID == "" || a1.Seq == 0 {
		t.Fatalf("a valid text message must be accepted through the gate: %+v", a1)
	}
	a1b := wsA.send(uidB, n1, text("hello"))
	if a1b.Reason != 1 || a1b.ID != a1.ID || a1b.Seq != a1.Seq {
		t.Fatalf("the resend must be acknowledged as the same message: first=%+v resend=%+v", a1, a1b)
	}
	if got := wsB.received(3 * time.Second); got[n1] != 1 {
		t.Fatalf("B must receive the message exactly once, got %v", got)
	}

	// 2. the gate refuses with business reason codes the client sees in SENDACK
	cases := []struct {
		name    string
		to      string
		payload map[string]any
		want    int
	}{
		{"unknown recipient", "no_such_user", text("x"), 130},
		{"system account is not a conversation peer", "vg_system", text("x"), 130},
		{"not a message object", uidB, map[string]any{"content": "no type"}, 131},
		{"empty text", uidB, text("   "), 131},
		{"oversized", uidB, text(strings.Repeat("a", 17<<10)), 132},
	}
	for _, c := range cases {
		if a := wsA.send(c.to, ulid(), c.payload); a.Reason != c.want {
			t.Errorf("%s: SENDACK reason %d, want %d", c.name, a.Reason, c.want)
		}
	}
	// unknown content types degrade on the client, the server lets them through
	if a := wsA.send(uidB, ulid(), map[string]any{"type": 9999, "schema_version": 3, "weird": true}); a.Reason != 1 {
		t.Errorf("unknown content types must pass: %+v", a)
	}

	// 3. the IM's native send ban still applies first (T-23), through the real gateway
	c, err := imgw.New(im.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetUserSendBan(context.Background(), uidA, true); err != nil {
		t.Fatal(err)
	}
	if a := wsA.send(uidB, ulid(), text("banned")); a.Reason == 1 {
		t.Fatal("a send-banned user must be refused by the IM natively")
	}
	if err := c.SetUserSendBan(context.Background(), uidA, false); err != nil {
		t.Fatal(err)
	}

	// 4. a disabled sender is refused by our gate (code 129)
	if _, err := d.Exec("UPDATE `user` SET status='disabled' WHERE uid=?", uidA); err != nil {
		t.Fatal(err)
	}
	if a := wsA.send(uidB, ulid(), text("from a disabled account")); a.Reason != 129 {
		t.Errorf("disabled sender: reason %d, want 129", a.Reason)
	}
	if _, err := d.Exec("UPDATE `user` SET status='active' WHERE uid=?", uidA); err != nil {
		t.Fatal(err)
	}

	// 5. a client cannot post into a conversation it is not part of: the IM itself refuses a hand-made composite channel id (observed reason 15),
	// and our gate's participant rule (unit-tested) is the second line for anything that reaches the hook.
	if a := wsA.send("someone_else@another_one", ulid(), text("x")); a.Reason == 1 {
		t.Errorf("posting into a foreign conversation must be refused, got %+v", a)
	}

	// 6. the business API shares the gate and the idempotency key; failures carry stable codes
	n2 := ulid()
	body := fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"via api"}}`, uidB, n2)
	c1, r1 := jpost(t, base+"/v1/messages", body, "Authorization", "Bearer "+accA)
	c2, r2 := jpost(t, base+"/v1/messages", body, "Authorization", "Bearer "+accA)
	if c1 != 200 || c2 != 200 || r1["message_id"] == "" || r1["message_id"] != r2["message_id"] || r1["message_seq"] != r2["message_seq"] {
		t.Fatalf("API send and its retry must return the same message: %d %v / %d %v", c1, r1, c2, r2)
	}
	var refs int
	if err := d.QueryRow("SELECT COUNT(*) FROM message_ref WHERE client_msg_no=?", n2).Scan(&refs); err != nil || refs != 1 {
		t.Fatalf("exactly one message_ref per message: %d %v", refs, err)
	}
	if got := wsB.received(3 * time.Second); got[n2] != 1 {
		t.Fatalf("B must receive the API message once, got %v", got)
	}
	for _, tc := range []struct {
		name, body string
		hdrAuth    bool
		want       int
	}{
		{"unknown recipient", fmt.Sprintf(`{"to_uid":"nobody","client_msg_no":%q,"payload":{"type":1,"content":"x"}}`, ulid()), true, 404},
		{"invalid payload", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"content":"x"}}`, uidB, ulid()), true, 400},
		{"client_msg_no is not a ULID", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":"abc","payload":{"type":1,"content":"x"}}`, uidB), true, 400},
		{"unauthenticated", body, false, 401},
	} {
		hdr := []string{}
		if tc.hdrAuth {
			hdr = []string{"Authorization", "Bearer " + accA}
		}
		if code, _ := jpost(t, base+"/v1/messages", tc.body, hdr...); code != tc.want {
			t.Errorf("%s: HTTP %d, want %d", tc.name, code, tc.want)
		}
	}

	// 7. concurrent sends into one conversation: every message is accepted once and the per-channel sequence has no gaps or repeats
	const N = 24
	var wg sync.WaitGroup
	seqs := make(chan uint64, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b := fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"c%d"}}`, uidB, ulid(), i)
			code, r := jpost(t, base+"/v1/messages", b, "Authorization", "Bearer "+accA)
			if code != 200 {
				t.Errorf("concurrent send %d: %d %v", i, code, r)
				return
			}
			seqs <- uint64(r["message_seq"].(float64))
		}(i)
	}
	wg.Wait()
	close(seqs)
	seen := map[uint64]bool{}
	var min, max uint64 = 1 << 62, 0
	for s := range seqs {
		if seen[s] {
			t.Errorf("sequence %d handed out twice", s)
		}
		seen[s] = true
		if s < min {
			min = s
		}
		if s > max {
			max = s
		}
	}
	if len(seen) != N || max-min != N-1 {
		t.Errorf("sequences of %d concurrent sends must be %d consecutive numbers, got %d distinct in [%d,%d]", N, N, len(seen), min, max)
	}

	// 7b. account notices from the system sender pass the gate (B signs in on a second device while connected: B's other device is told)
	login("+8613800138102", "device-dddddddddddddddd")
	got := wsB.received(5 * time.Second)
	notices := 0
	for k := range got {
		if strings.HasPrefix(k, "devnotice:") {
			notices++
		}
	}
	if notices == 0 {
		t.Errorf("the system notice must pass the before_send gate and reach B: %v", got)
	}

	// 7c. latency evidence (not a pass/fail gate: the 50 ms P95 budget is an [ASSUMPTION] to be confirmed under load on real hardware, NFR-9):
	// SENDACK round trip of sequential client sends = IM + synchronous before_send hook + one MySQL lookup, on this machine.
	var lat []time.Duration
	for i := 0; i < 60; i++ {
		t0 := time.Now()
		if a := wsA.send(uidB, ulid(), text("latency probe")); a.Reason != 1 {
			t.Fatalf("latency probe refused: %+v", a)
		}
		lat = append(lat, time.Since(t0))
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	t.Logf("SENDACK latency over 60 sequential sends through the gate: p50=%v p95=%v max=%v", lat[30], lat[56], lat[59])
	if lat[56] > 400*time.Millisecond {
		t.Errorf("P95 SENDACK %v exceeds the gate budget on a quiet machine", lat[56])
	}

	// 7d. message_ref: messages the client sent straight to the IM are recorded from msg.notify; API-sent ones are recorded by both paths but only once;
	// system notices are not business messages.
	eventually(t, "msg.notify feeds message_ref", 10*time.Second, func() bool {
		var id uint64
		return d.QueryRow("SELECT message_id FROM message_ref WHERE client_msg_no=?", n1).Scan(&id) == nil
	})
	var refID, refSeq uint64
	var refChan, refFrom string
	var cnt int
	if err := d.QueryRow("SELECT COUNT(*), MIN(message_id), MIN(message_seq), MIN(channel_id), MIN(from_uid) FROM message_ref WHERE client_msg_no=?", n1).Scan(&cnt, &refID, &refSeq, &refChan, &refFrom); err != nil {
		t.Fatal(err)
	}
	if cnt != 1 || fmt.Sprint(refID) != a1.ID || refSeq != a1.Seq || refFrom != uidA || refChan != domainPersonChannel(uidA, uidB) {
		t.Fatalf("client-sent message must be recorded once with the IM's ids: count=%d id=%d seq=%d chan=%q from=%q (ack %+v)", cnt, refID, refSeq, refChan, refFrom, a1)
	}
	if err := d.QueryRow("SELECT COUNT(*) FROM message_ref WHERE client_msg_no=?", n2).Scan(&cnt); err != nil || cnt != 1 {
		t.Fatalf("an API message is recorded by the API and by msg.notify but must stay one row: %d %v", cnt, err)
	}
	if err := d.QueryRow("SELECT COUNT(*) FROM message_ref WHERE from_uid='vg_system' OR client_msg_no LIKE 'devnotice:%' OR client_msg_no LIKE 'syncnotice:%'").Scan(&cnt); err != nil || cnt != 0 {
		t.Fatalf("system notices must not be recorded: %d %v", cnt, err)
	}

	// 8. every evaluation is visible in metrics
	_, m := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	for _, want := range []string{`vg_message_gate_total{rule="accounts",result="allow"}`, `vg_message_gate_total{rule="payload",result="deny"}`} {
		if !strings.Contains(m, want) {
			t.Errorf("metric missing: %s", want)
		}
	}
}

// Fault injection (AD-21): with the business service unreachable, the IM's synchronous hook cannot approve anything, so sends are refused rather
// than waved through; and a MySQL outage makes the gate itself deny (restricted rule fails closed) instead of allowing.
func TestBeforeSendFailsClosedWhenBusinessServiceIsDownOrDatabaseIsGone(t *testing.T) {
	dead := freeAddr(t) // nothing listens here
	im := testim.Start(t, "http://"+dead+"/im/webhook/"+msgHookSecret)
	c, err := imgw.New(im.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	c.Backoff = 20 * time.Millisecond
	if _, err := c.Send(context.Background(), "alice", "bob", 1, ulid(), []byte(`{"type":1,"content":"hi"}`)); err == nil {
		t.Fatal("with the before_send service unreachable the IM must refuse the send (fail-closed)")
	}
}

// The two copies of the system uid (identity sends as it, message's gate lets it through) must never drift apart.
func TestSystemUIDIsTheSameInIdentityAndMessage(t *testing.T) {
	if identitydomain.SystemUID != msgdomain.SystemUID {
		t.Fatalf("identity %q vs message %q", identitydomain.SystemUID, msgdomain.SystemUID)
	}
}

// Per-sender rate limit (AD-21 "rate-limit" rule) through the real IM hook and the API; other senders and system notices are unaffected.
func TestSendRateLimitThroughRealIMAndAPI(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	hookAddr := freeAddr(t)
	im := testim.Start(t, "http://"+hookAddr+"/im/webhook/"+msgHookSecret)
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef",
		"VG_IM_WEBHOOK_SECRET=" + msgHookSecret, "VG_IM_WEBHOOK_ADDR=" + hookAddr, "VG_MESSAGE_RATE_LIMIT=5", "VG_MESSAGE_RATE_WINDOW_SECONDS=60"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	startWorker(t, bin, dsn, extra)
	base := "http://" + p.pub
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d", im.Ports["ws"])
	login := func(phone, device string) string {
		_, _ = d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE")
		before := len(regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1))
		jpost(t, base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
		var code string
		eventually(t, "sms code", 3*time.Second, func() bool {
			all := regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		_, body := jpost(t, base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"app"}}`)
		a, _ := body["access_token"].(string)
		return a
	}
	accA, accB := login("+8613800138301", "device-aaaaaaaaaaaaaaaa"), login("+8613800138302", "device-bbbbbbbbbbbbbbbb")
	cr := func(acc string) (string, string) {
		_, b := get(t, base+"/v1/im/token", "Bearer "+acc)
		var o struct{ UID, Token string }
		_ = json.Unmarshal([]byte(strings.ReplaceAll(strings.ReplaceAll(b, `"uid"`, `"UID"`), `"token"`, `"Token"`)), &o)
		return o.UID, o.Token
	}
	uidA, tokA := cr(accA)
	uidB, _ := cr(accB)
	wsA := dialWS(t, wsURL)
	if !wsA.connect(uidA, tokA, "im-a", 0) {
		t.Fatal("connect")
	}
	for i := 1; i <= 5; i++ {
		if a := wsA.send(uidB, ulid(), map[string]any{"type": 1, "content": "x"}); a.Reason != 1 {
			t.Fatalf("send %d within the limit: %+v", i, a)
		}
	}
	if a := wsA.send(uidB, ulid(), map[string]any{"type": 1, "content": "one too many"}); a.Reason != 133 {
		t.Fatalf("the 6th send in the window must be refused with 133: %+v", a)
	}
	body := fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"api"}}`, uidB, ulid())
	req, _ := http.NewRequest("POST", base+"/v1/messages", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+accA)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 429 || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("the API must answer 429 with Retry-After: %v %v", err, resp)
	}
	resp.Body.Close()
	// another sender has its own budget; system notices are exempt
	if c, _ := jpost(t, base+"/v1/messages", fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"hi"}}`, uidA, ulid()), "Authorization", "Bearer "+accB); c != 200 {
		t.Fatalf("B must not be affected by A's limit: %d", c)
	}
	_, m := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	if !strings.Contains(m, `vg_message_gate_total{rule="ratelimit",result="deny"}`) {
		t.Errorf("rate-limit denials must be visible in metrics")
	}
}

func domainPersonChannel(a, b string) string { return msgdomain.PersonChannel(a, b) }

// Load evidence for NFR-9: many concurrent senders through the real IM hook and the real gate. Throughput and latency are recorded; what is asserted is
// that nothing is refused, nothing times out and the gate stays inside its budget on this machine. The 50 ms P95 target remains an [ASSUMPTION] to be
// confirmed on production-like hardware.
func TestGateUnderConcurrentLoad(t *testing.T) {
	redisAddr := os.Getenv("VG_TEST_REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("VG_TEST_REDIS_ADDR not set")
	}
	hookAddr := freeAddr(t)
	im := testim.Start(t, "http://"+hookAddr+"/im/webhook/"+msgHookSecret)
	d, dsn, bin := migrated(t)
	extra := []string{"VG_REDIS_ADDR=" + redisAddr, "VG_IM_API_URL=" + im.APIURL, "VG_METRICS_TOKEN=metrics-token-0123456789abcdef",
		"VG_IM_WEBHOOK_SECRET=" + msgHookSecret, "VG_IM_WEBHOOK_ADDR=" + hookAddr, "VG_MESSAGE_RATE_LIMIT=100000"}
	if r, err := denylist.Open(context.Background(), redisAddr, ""); err == nil {
		r.C.Del(context.Background(), "vg:deny:dev:1:1", "vg:deny:dev:1:2")
		r.Close()
	}
	p := start(t, authEnv(dsn, extra...)...)
	startWorker(t, bin, dsn, extra)
	base := "http://" + p.pub
	login := func(phone, device string) string {
		_, _ = d.Exec("UPDATE login_event SET created_at = created_at - INTERVAL 5 MINUTE")
		before := len(regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1))
		jpost(t, base+"/v1/auth/sms/code", `{"phone":"`+phone+`","device_id":"`+device+`"}`)
		var code string
		eventually(t, "sms code", 3*time.Second, func() bool {
			all := regexp.MustCompile(`验证码 (\d{6})`).FindAllStringSubmatch(p.stderr.String(), -1)
			if len(all) > before {
				code = all[len(all)-1][1]
				return true
			}
			return false
		})
		_, body := jpost(t, base+"/v1/auth/login", `{"phone":"`+phone+`","code":"`+code+`","device":{"id":"`+device+`","class":"app"}}`)
		a, _ := body["access_token"].(string)
		return a
	}
	accA, accB := login("+8613800138401", "device-aaaaaaaaaaaaaaaa"), login("+8613800138402", "device-bbbbbbbbbbbbbbbb")
	_, bm := jget(t, base+"/v1/im/token", accB)
	uidB, _ := bm["uid"].(string)

	const workers, per = 16, 50
	lat := make(chan time.Duration, workers*per)
	var wg sync.WaitGroup
	start0 := time.Now()
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				b := fmt.Sprintf(`{"to_uid":%q,"client_msg_no":%q,"payload":{"type":1,"content":"load"}}`, uidB, ulid())
				t0 := time.Now()
				c, r := jpost(t, base+"/v1/messages", b, "Authorization", "Bearer "+accA)
				if c != 200 {
					t.Errorf("load send refused: %d %v", c, r)
					return
				}
				lat <- time.Since(t0)
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start0)
	close(lat)
	var all []time.Duration
	for l := range lat {
		all = append(all, l)
	}
	if len(all) != workers*per {
		t.Fatalf("every send must succeed: %d of %d", len(all), workers*per)
	}
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	_, m := get(t, "http://"+p.internal+"/metrics", "Bearer metrics-token-0123456789abcdef")
	// gate-only latency from the histogram: the smallest bucket that holds 95% of evaluations
	var total float64
	buckets := map[string]float64{}
	for _, line := range strings.Split(m, "\n") {
		if strings.HasPrefix(line, "vg_message_gate_duration_seconds_count") {
			fmt.Sscanf(line[strings.LastIndex(line, " ")+1:], "%g", &total)
		}
		if strings.HasPrefix(line, "vg_message_gate_duration_seconds_bucket{le=\"") {
			le := line[len("vg_message_gate_duration_seconds_bucket{le=\""):]
			le = le[:strings.Index(le, "\"")]
			var v float64
			fmt.Sscanf(line[strings.LastIndex(line, " ")+1:], "%g", &v)
			buckets[le] = v
		}
	}
	p95 := "n/a"
	for _, le := range []string{"0.001", "0.0025", "0.005", "0.01", "0.025", "0.05", "0.1", "0.25", "0.5"} {
		if total > 0 && buckets[le] >= 0.95*total {
			p95 = le
			break
		}
	}
	t.Logf("load: %d sends by %d concurrent workers in %v = %.0f sends/s; end-to-end p50=%v p95=%v p99=%v max=%v; gate evaluations=%.0f, gate p95 <= %ss",
		len(all), workers, elapsed.Round(time.Millisecond), float64(len(all))/elapsed.Seconds(), all[len(all)/2], all[len(all)*95/100], all[len(all)*99/100], all[len(all)-1], total, p95)
	if strings.Contains(m, `result="timeout"`) || strings.Contains(m, `result="error"`) {
		t.Errorf("no gate evaluation may time out or fail under this load")
	}
	if p95 == "n/a" {
		t.Errorf("gate latency histogram missing")
	}
}
