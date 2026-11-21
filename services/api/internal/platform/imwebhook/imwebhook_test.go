package imwebhook_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veyrogram/api/internal/platform/imwebhook"
)

const secret = "s3cr3t-path-token-0123456789abcdef0123"

type rig struct {
	h       http.Handler
	rejects []string
}

func newRig(t *testing.T, mutate func(*imwebhook.Config)) *rig {
	t.Helper()
	r := &rig{}
	c := imwebhook.Config{Secret: secret, OnReject: func(s string) { r.rejects = append(r.rejects, s) }}
	if mutate != nil {
		mutate(&c)
	}
	r.h = imwebhook.Handler(c)
	return r
}

func (r *rig) post(path, ct, remote, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.RemoteAddr = remote
	if ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, req)
	return rec
}

const okBody = `{"from_uid":"u1","channel_id":"u2","channel_type":1,"client_msg_no":"c1","payload":"aGk="}`
const okPath = "/im/webhook/" + secret + "?event=msg.before_send"

func TestWrongOrMissingSecretIs403AndReported(t *testing.T) {
	r := newRig(t, nil)
	for _, p := range []string{"/im/webhook/?event=msg.before_send", "/im/webhook/wrong?event=msg.before_send", "/im/webhook/" + secret[:20] + "?event=msg.before_send", "/im/webhook/" + secret + "x?event=msg.before_send", "/other?event=msg.before_send", "/?event=msg.before_send"} {
		if rec := r.post(p, "application/json", "127.0.0.1:5000", okBody); rec.Code != 403 {
			t.Errorf("%s must be 403, got %d", p, rec.Code)
		}
	}
	if len(r.rejects) != 6 {
		t.Errorf("every rejection must be reported for alerting, got %v", r.rejects)
	}
}

func TestSourceAllowlist(t *testing.T) {
	r := newRig(t, nil)
	for _, remote := range []string{"203.0.113.9:1234", "10.0.0.5:1234", "[2001:db8::1]:1234", "garbage"} {
		if rec := r.post(okPath, "application/json", remote, okBody); rec.Code != 403 {
			t.Errorf("source %s must be 403 by default (loopback only), got %d", remote, rec.Code)
		}
	}
	for _, remote := range []string{"127.0.0.1:1", "[::1]:1", "[::ffff:127.0.0.1]:1"} {
		if rec := r.post(okPath, "application/json", remote, okBody); rec.Code != 200 {
			t.Errorf("loopback source %s must be accepted, got %d", remote, rec.Code)
		}
	}
	al, err := imwebhook.ParseAllow("10.0.0.0/24, 192.168.1.7")
	if err != nil {
		t.Fatal(err)
	}
	r2 := newRig(t, func(c *imwebhook.Config) { c.Allow = al })
	if r2.post(okPath, "application/json", "10.0.0.9:1", okBody).Code != 200 || r2.post(okPath, "application/json", "192.168.1.7:1", okBody).Code != 200 {
		t.Error("configured sources must be accepted")
	}
	if r2.post(okPath, "application/json", "127.0.0.1:1", okBody).Code != 403 || r2.post(okPath, "application/json", "10.0.1.9:1", okBody).Code != 403 {
		t.Error("sources outside the configured allowlist must be rejected (including loopback once an allowlist is set)")
	}
	if _, err := imwebhook.ParseAllow("not-an-ip"); err == nil {
		t.Error("bad allowlist entries must be rejected")
	}
}

func TestDisabledWithoutSecretNeverOpens(t *testing.T) {
	r := newRig(t, func(c *imwebhook.Config) { c.Secret = "" })
	for _, p := range []string{"/im/webhook/?event=msg.before_send", "/im/webhook/anything?event=msg.before_send", "/im/webhook/" + "?event=x"} {
		if rec := r.post(p, "application/json", "127.0.0.1:1", okBody); rec.Code != 404 {
			t.Errorf("empty secret must disable the endpoint, got %d for %s", rec.Code, p)
		}
	}
}

func TestRequestShapeValidation(t *testing.T) {
	r := newRig(t, nil)
	get := httptest.NewRequest("GET", okPath, nil)
	get.RemoteAddr = "127.0.0.1:1"
	rec := httptest.NewRecorder()
	r.h.ServeHTTP(rec, get)
	if rec.Code != 405 {
		t.Errorf("GET must be 405, got %d", rec.Code)
	}
	if got := r.post(okPath, "text/plain", "127.0.0.1:1", okBody).Code; got != 415 {
		t.Errorf("non-JSON content type must be 415, got %d", got)
	}
	if got := r.post(okPath, "application/json", "127.0.0.1:1", strings.Repeat("x", imwebhook.MaxBody+10)).Code; got != 413 {
		t.Errorf("oversized body must be 413, got %d", got)
	}
	for _, b := range []string{"", "{", `{"from_uid":"","channel_id":"x"}`, `{"from_uid":"u"}`, "[]"} {
		if got := r.post(okPath, "application/json", "127.0.0.1:1", b).Code; got != 400 {
			t.Errorf("malformed body %q must be 400, got %d", b, got)
		}
	}
}

func decide(t *testing.T, rec *httptest.ResponseRecorder) imwebhook.Decision {
	t.Helper()
	var d imwebhook.Decision
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &d) != nil {
		t.Fatalf("expected a JSON decision, got %d %q", rec.Code, rec.Body.String())
	}
	return d
}

func TestBeforeSendFailsClosedWithoutPolicyAndOnError(t *testing.T) {
	r := newRig(t, nil)
	if d := decide(t, r.post(okPath, "application/json", "127.0.0.1:1", okBody)); d.Allow || d.ReasonCode != imwebhook.DenyUnavailable {
		t.Fatalf("with no policy installed every send must be denied, got %+v", d)
	}
	r = newRig(t, func(c *imwebhook.Config) {
		c.Handlers.BeforeSend = func(context.Context, imwebhook.BeforeSend) (imwebhook.Decision, error) {
			return imwebhook.Decision{Allow: true}, errors.New("redis down")
		}
	})
	if d := decide(t, r.post(okPath, "application/json", "127.0.0.1:1", okBody)); d.Allow {
		t.Fatalf("a policy error must deny (fail-closed), got %+v", d)
	}
}

func TestBeforeSendPassesDecodedMessageAndRelaysDecision(t *testing.T) {
	var got imwebhook.BeforeSend
	r := newRig(t, func(c *imwebhook.Config) {
		c.Handlers.BeforeSend = func(_ context.Context, m imwebhook.BeforeSend) (imwebhook.Decision, error) {
			got = m
			return imwebhook.Decision{Allow: false, ReasonCode: 130}, nil
		}
	})
	d := decide(t, r.post(okPath, "application/json", "127.0.0.1:1", okBody))
	if d.Allow || d.ReasonCode != 130 || got.FromUID != "u1" || got.ChannelID != "u2" || got.ChannelType != 1 || got.ClientMsgNo != "c1" || string(got.Payload) != "hi" {
		t.Fatalf("decision/decoding wrong: %+v %+v", d, got)
	}
}

func TestUnknownFieldsAndUnknownEventsAreTolerated(t *testing.T) {
	r := newRig(t, func(c *imwebhook.Config) {
		c.Handlers.BeforeSend = func(context.Context, imwebhook.BeforeSend) (imwebhook.Decision, error) {
			return imwebhook.Decision{Allow: true}, nil
		}
	})
	if d := decide(t, r.post(okPath, "application/json", "127.0.0.1:1", `{"from_uid":"u","channel_id":"c","future_field":1}`)); !d.Allow {
		t.Error("new upstream fields must not break the gate")
	}
	if rec := r.post("/im/webhook/"+secret+"?event=msg.notify", "application/json", "127.0.0.1:1", `[]`); rec.Code != 200 {
		t.Errorf("unhandled notifications are acknowledged, got %d", rec.Code)
	}
}

func TestNotifyDecodesBatchAndRetriesOnHandlerError(t *testing.T) {
	var got []imwebhook.Notified
	var fail bool
	h := imwebhook.Handler(imwebhook.Config{Secret: secret, Handlers: imwebhook.Handlers{Notify: func(_ context.Context, b []imwebhook.Notified) error {
		got = append(got, b...)
		if fail {
			return errors.New("db down")
		}
		return nil
	}}})
	body := `[{"message_id":2124048718486310912,"message_seq":1,"client_msg_no":"c1","from_uid":"alice","channel_id":"bob@alice","channel_type":1,"timestamp":1795247652,"payload":"eyJ0eXBlIjoxfQ=="}]`
	do := func(b string) int {
		r := httptest.NewRequest("POST", "/im/webhook/"+secret+"?event=msg.notify", strings.NewReader(b))
		r.Header.Set("Content-Type", "application/json")
		r.RemoteAddr = "127.0.0.1:1234"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code
	}
	if c := do(body); c != 200 || len(got) != 1 || got[0].MessageID != 2124048718486310912 || string(got[0].Payload) != `{"type":1}` {
		t.Fatalf("decode: %d %+v", c, got)
	}
	fail = true
	if c := do(body); c != 500 {
		t.Fatalf("a failing consumer must answer 500 so the IM retries: %d", c)
	}
	if c := do(`not json`); c != 400 {
		t.Fatalf("malformed: %d", c)
	}
	// a batch bigger than the before_send cap but within the notify cap is accepted
	big := `[{"message_id":1,"message_seq":1,"client_msg_no":"c","from_uid":"a","channel_id":"a@b","channel_type":1,"payload":"` + strings.Repeat("QUJD", 100<<10) + `"}]`
	fail = false
	if c := do(big); c != 200 {
		t.Fatalf("large notify batch: %d", c)
	}
	// without a consumer the event is acknowledged and ignored
	h2 := imwebhook.Handler(imwebhook.Config{Secret: secret})
	r := httptest.NewRequest("POST", "/im/webhook/"+secret+"?event=msg.notify", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.RemoteAddr = "127.0.0.1:1234"
	w := httptest.NewRecorder()
	h2.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatalf("no consumer: %d", w.Code)
	}
}
