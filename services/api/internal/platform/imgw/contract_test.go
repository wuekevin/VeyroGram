package imgw_test

// Contract tests: the imgw client and the imwebhook handler against the REAL WuKongIM built from the pinned SHA (tools/build_wukongim.sh).
// They pin the upstream behaviour the business service depends on; when the pin moves and one of these fails, the integration must be revisited.

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/imwebhook"
	"veyrogram/api/internal/platform/testim"
)

const hookSecret = "contract-secret-0123456789abcdef0123456789"

func newClient(t *testing.T, im *testim.IM) *imgw.Client {
	t.Helper()
	c, err := imgw.New(im.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	c.Backoff = 20 * time.Millisecond
	return c
}

func TestContractTokenSendIdempotencyAndNativeSendBan(t *testing.T) {
	im := testim.Start(t, "")
	c := newClient(t, im)
	ctx := context.Background()
	if err := c.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if err := c.UpdateToken(ctx, "u1", "tok-u1", 1, 0); err != nil {
		t.Fatalf("update token: %v", err)
	}
	r1, err := c.Send(ctx, "u1", "u2", 1, "cmn-1", []byte("hello"))
	if err != nil || r1.MessageID == 0 || r1.MessageSeq != 1 {
		t.Fatalf("first send: %+v %v", r1, err)
	}
	r2, err := c.Send(ctx, "u1", "u2", 1, "cmn-1", []byte("hello"))
	if err != nil || r2.MessageID != r1.MessageID || r2.MessageSeq != r1.MessageSeq {
		t.Fatalf("same client_msg_no must be de-duplicated by the IM (retry safety): %+v vs %+v (%v)", r2, r1, err)
	}
	if err := c.SetUserSendBan(ctx, "u1", true); err != nil {
		t.Fatal(err)
	}
	var rej *imgw.SendRejected
	if _, err := c.Send(ctx, "u1", "u2", 1, "cmn-2", []byte("hi")); !errors.As(err, &rej) || rej.Reason != 25 {
		t.Fatalf("a muted sender must be refused by the IM natively (reason 25), got %v", err)
	}
	if err := c.SetUserSendBan(ctx, "u1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Send(ctx, "u1", "u2", 1, "cmn-3", []byte("hi")); err != nil {
		t.Fatalf("after unmute the send must succeed: %v", err)
	}
	if err := c.DeviceQuit(ctx, "u1", 255); err != nil {
		t.Fatalf("device_quit: %v", err)
	}
}

type hookRig struct {
	mu      sync.Mutex
	got     []imwebhook.BeforeSend
	rejects []string
	policy  func(imwebhook.BeforeSend) (imwebhook.Decision, error)
}

func (h *hookRig) start(t *testing.T, pathSecret string) string {
	t.Helper()
	return h.startSigned(t, pathSecret, "")
}

func (h *hookRig) startSigned(t *testing.T, pathSecret, signing string) string {
	t.Helper()
	handler := imwebhook.Handler(imwebhook.Config{
		Secret:        hookSecret,
		SigningSecret: signing,
		OnReject:      func(r string) { h.mu.Lock(); h.rejects = append(h.rejects, r); h.mu.Unlock() },
		Handlers: imwebhook.Handlers{BeforeSend: func(_ context.Context, m imwebhook.BeforeSend) (imwebhook.Decision, error) {
			h.mu.Lock()
			h.got = append(h.got, m)
			p := h.policy
			h.mu.Unlock()
			return p(m)
		}},
	})
	srv := httptest.NewServer(handler) // real TCP from loopback, so the source allowlist sees 127.0.0.1
	t.Cleanup(srv.Close)
	return srv.URL + "/im/webhook/" + pathSecret
}

func TestContractBeforeSendCallbackWireFormatAndDecisions(t *testing.T) {
	h := &hookRig{policy: func(m imwebhook.BeforeSend) (imwebhook.Decision, error) { return imwebhook.Decision{Allow: true}, nil }}
	im := testim.Start(t, h.start(t, hookSecret))
	c := newClient(t, im)
	ctx := context.Background()

	// allow: the real callback must reach our handler with the documented shape and the message is accepted
	if _, err := c.Send(ctx, "alice", "bob", 1, "cmn-allow", []byte("hello")); err != nil {
		t.Fatalf("allowed send failed: %v", err)
	}
	h.mu.Lock()
	if len(h.got) == 0 {
		h.mu.Unlock()
		t.Fatal("IM did not call the before_send webhook")
	}
	m := h.got[0]
	h.mu.Unlock()
	if m.FromUID != "alice" || m.ClientMsgNo != "cmn-allow" || m.ChannelType != 1 || string(m.Payload) != "hello" {
		t.Fatalf("callback decoded wrong: %+v", m)
	}
	// the person channel id is a composite of both uids (policy code must not assume the bare peer uid)
	if m.ChannelID != "bob@alice" && m.ChannelID != "alice@bob" {
		t.Fatalf("person channel id shape changed upstream: %q", m.ChannelID)
	}

	// deny with a business code: the IM relays that exact reason code to the sender
	h.mu.Lock()
	h.policy = func(imwebhook.BeforeSend) (imwebhook.Decision, error) {
		return imwebhook.Decision{Allow: false, ReasonCode: 130}, nil
	}
	h.mu.Unlock()
	var rej *imgw.SendRejected
	if _, err := c.Send(ctx, "alice", "bob", 1, "cmn-deny", []byte("x")); !errors.As(err, &rej) || rej.Reason != 130 {
		t.Fatalf("a denied send must carry the business reason code 130, got %v", err)
	}

	// policy failure: our handler fails closed, so the IM rejects the send
	h.mu.Lock()
	h.policy = func(imwebhook.BeforeSend) (imwebhook.Decision, error) {
		return imwebhook.Decision{Allow: true}, errors.New("policy backend down")
	}
	h.mu.Unlock()
	if _, err := c.Send(ctx, "alice", "bob", 1, "cmn-err", []byte("x")); err == nil {
		t.Fatal("when the policy errors the send must be rejected (fail-closed)")
	}
}

func TestContractWrongCallbackSecretFailsClosedAndIsReported(t *testing.T) {
	h := &hookRig{policy: func(imwebhook.BeforeSend) (imwebhook.Decision, error) { return imwebhook.Decision{Allow: true}, nil }}
	im := testim.Start(t, h.start(t, "not-the-secret"))
	c := newClient(t, im)
	if _, err := c.Send(context.Background(), "alice", "bob", 1, "cmn-x", []byte("x")); err == nil {
		t.Fatal("if the callback is refused (403) the IM must reject the send (on_error=deny)")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.got) != 0 || len(h.rejects) == 0 || h.rejects[0] != "bad_secret" {
		t.Fatalf("policy must never run on a bad secret and the rejection must be reported: got=%d rejects=%v", len(h.got), h.rejects)
	}
}

func TestContractAdapterRecoversAfterIMRestart(t *testing.T) {
	im := testim.Start(t, "")
	c := newClient(t, im)
	ctx := context.Background()
	r1, err := c.Send(ctx, "u1", "u2", 1, "cmn-keep", []byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	im.Stop()
	var events []string
	c.Observe = func(op, res string, _ time.Duration) { events = append(events, op+":"+res) }
	if err := c.Health(ctx); err == nil {
		t.Fatal("health must fail while the IM is down")
	}
	if len(events) != 1 || events[0] != "health:error" {
		t.Fatalf("an outage must be reported to the observer once per call: %v", events)
	}
	im.Restart()
	if err := c.Health(ctx); err != nil {
		t.Fatalf("the adapter must reconnect without being recreated: %v", err)
	}
	r2, err := c.Send(ctx, "u1", "u2", 1, "cmn-keep", []byte("x"))
	if err != nil || r2.MessageID != r1.MessageID {
		t.Fatalf("de-duplication must survive an IM restart (durable): %+v vs %+v %v", r2, r1, err)
	}
}

var _ = http.StatusOK

// The patched WuKongIM (patches/wukongim/0001) signs callbacks. Against the REAL binary: a matching key lets sends through; a mismatching key
// makes our handler refuse every callback (reported as bad_signature) and the IM, which fails closed, rejects the sends.
func TestContractSignedCallbacksAreAuthenticatedAndForgeriesFailClosed(t *testing.T) {
	const key = "contract-signing-key-0123456789abcdef"
	allow := func(imwebhook.BeforeSend) (imwebhook.Decision, error) { return imwebhook.Decision{Allow: true}, nil }

	h := &hookRig{policy: allow}
	im := testim.StartSigned(t, h.startSigned(t, hookSecret, key), key)
	c := newClient(t, im)
	if _, err := c.Send(context.Background(), "alice", "bob", 1, "sig-ok", []byte("hello")); err != nil {
		t.Fatalf("a callback signed with the shared key must be accepted: %v", err)
	}
	h.mu.Lock()
	if len(h.got) == 0 || len(h.rejects) != 0 {
		t.Fatalf("callback not delivered or wrongly rejected: got=%d rejects=%v", len(h.got), h.rejects)
	}
	h.mu.Unlock()

	h2 := &hookRig{policy: allow}
	im2 := testim.StartSigned(t, h2.startSigned(t, hookSecret, "a-different-key-0123456789abcdef0123"), key)
	if _, err := newClient(t, im2).Send(context.Background(), "alice", "bob", 1, "sig-bad", []byte("x")); err == nil {
		t.Fatal("a callback whose signature does not verify must make the send fail (fail-closed)")
	}
	h2.mu.Lock()
	defer h2.mu.Unlock()
	if len(h2.got) != 0 {
		t.Fatalf("the policy must never see an unauthenticated callback: %v", h2.got)
	}
	if len(h2.rejects) == 0 || h2.rejects[0] != "bad_signature" {
		t.Fatalf("a forged callback must be reported as bad_signature for alerting, got %v", h2.rejects)
	}
}
