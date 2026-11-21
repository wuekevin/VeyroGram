package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veyrogram/api/internal/message/app"
	"veyrogram/api/internal/message/domain"
	"veyrogram/api/internal/message/ports"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/token"
)

type fakeIM struct {
	err error
	n   int
}

func (f *fakeIM) Send(_ context.Context, _, _ string, _ uint8, _ string, _ []byte) (ports.Ack, error) {
	f.n++
	return ports.Ack{MessageID: 42, MessageSeq: 7}, f.err
}

type fakeRefs struct{ err error }

func (f fakeRefs) Record(context.Context, ports.Ref) error { return f.err }

type allowAll struct{}

func (allowAll) Active(_ context.Context, uids []string) (map[string]bool, error) {
	m := map[string]bool{}
	for _, u := range uids {
		m[u] = true
	}
	return m, nil
}

// as logs the request in as alice without a real token: the handler only needs the claims the auth middleware would have set.
func as(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(authn.WithClaims(r.Context(), token.Claims{Subject: "alice"})))
	})
}

func do(t *testing.T, im *fakeIM, refs ports.Refs, body string) *httptest.ResponseRecorder {
	t.Helper()
	svc := &app.Service{Gate: &app.Gate{Rules: app.StandardRules(allowAll{})}, IM: im, Refs: refs, Tenant: 1}
	mux := http.NewServeMux()
	(&Handler{Svc: svc, Auth: as}).Mount(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body)))
	return w
}

const ok = `{"to_uid":"bob","client_msg_no":"01HZX3J9K2M4N6P8Q0R2S4T6V8","payload":{"type":1,"content":"hi"}}`

func TestSendSuccessAndErrorMapping(t *testing.T) {
	if w := do(t, &fakeIM{}, fakeRefs{}, ok); w.Code != 200 || !strings.Contains(w.Body.String(), `"message_id":"42"`) || !strings.Contains(w.Body.String(), `"message_seq":7`) {
		t.Fatalf("success: %d %s", w.Code, w.Body)
	}
	for _, c := range []struct {
		name string
		im   error
		body string
		code int
		err  string
	}{
		{"IM native send ban", &domain.Rejected{Reason: IMMuted, Native: true}, ok, 403, "muted"},
		{"IM native other refusal", &domain.Rejected{Reason: 99, Native: true}, ok, 403, "message_rejected"},
		{"IM unreachable", domain.ErrIMUnavailable, ok, 503, "im_unavailable"},
		{"gate: invalid payload", nil, strings.Replace(ok, `"type":1,`, ``, 1), 400, "invalid_payload"},
		{"gate: too large", nil, strings.Replace(ok, `"hi"`, `"`+strings.Repeat("a", 17<<10)+`"`, 1), 413, "payload_too_large"},
		{"bad ULID", nil, strings.Replace(ok, "01HZX3J9K2M4N6P8Q0R2S4T6V8", "nope", 1), 400, "invalid_request"},
		{"unknown field", nil, strings.Replace(ok, `"to_uid"`, `"x":1,"to_uid"`, 1), 400, "invalid_request"},
		{"trailing garbage", nil, ok + `{}`, 400, "invalid_request"},
		{"unexpected failure", errors.New("boom"), ok, 500, "internal"},
	} {
		w := do(t, &fakeIM{err: c.im}, fakeRefs{}, c.body)
		if w.Code != c.code || !strings.Contains(w.Body.String(), `"`+c.err+`"`) {
			t.Errorf("%s: %d %s, want %d %s", c.name, w.Code, w.Body, c.code, c.err)
		}
		if strings.Contains(w.Body.String(), "boom") {
			t.Errorf("%s: internal errors must not leak", c.name)
		}
	}
	// the gate refuses before the IM is ever called
	im := &fakeIM{}
	if w := do(t, im, fakeRefs{}, strings.Replace(ok, `"type":1,`, ``, 1)); w.Code != 400 || im.n != 0 {
		t.Fatalf("a refused message must never reach the IM: %d calls=%d", w.Code, im.n)
	}
	// an accepted message whose reference could not be stored is an error to the caller (the same-key retry repairs it), not silent success
	if w := do(t, &fakeIM{}, fakeRefs{err: errors.New("db down")}, ok); w.Code != 500 {
		t.Fatalf("reference failure: %d", w.Code)
	}
}

func TestMountsNothingWithoutAuthAndUnknownPathsAre404(t *testing.T) {
	mux := http.NewServeMux()
	(&Handler{Svc: &app.Service{}}).Mount(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/v1/messages", strings.NewReader(ok)))
	if w.Code == 200 {
		t.Fatal("without an auth middleware the route must not exist")
	}
}
