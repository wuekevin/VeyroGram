package imgw_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/logx"
)

func client(t *testing.T, h http.HandlerFunc) (*imgw.Client, *[]string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := imgw.New(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var events []string
	c.Observe = func(op, res string, _ time.Duration) { events = append(events, op+":"+res) }
	c.Sleep = func(time.Duration) {}
	return c, &events
}

func TestPublicAddressesAreRefused(t *testing.T) {
	for _, u := range []string{"http://8.8.8.8:5001", "http://example.com:5001", "https://203.0.113.5", "http://0.0.0.0:5001", "http://[2001:db8::1]:5001"} {
		if _, err := imgw.New(u); !errors.Is(err, imgw.ErrNotPrivate) {
			t.Errorf("%s must be refused, got %v", u, err)
		}
	}
	for _, u := range []string{"http://127.0.0.1:5001", "http://localhost:5001", "http://10.1.2.3:5001", "http://192.168.0.9:5001", "http://[::1]:5001"} {
		if _, err := imgw.New(u); err != nil {
			t.Errorf("%s must be accepted: %v", u, err)
		}
	}
	for _, u := range []string{"", "ftp://127.0.0.1", "127.0.0.1:5001", "http://"} {
		if _, err := imgw.New(u); err == nil {
			t.Errorf("%q must be rejected", u)
		}
	}
}

func TestUpdateTokenSendsDeviceFlagAndReportsOK(t *testing.T) {
	var got map[string]any
	c, ev := client(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user/token" || r.Method != "POST" {
			t.Errorf("wrong request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"status":200}`))
	})
	if err := c.UpdateToken(context.Background(), "u1", "tok", 2, 1); err != nil {
		t.Fatal(err)
	}
	if got["uid"] != "u1" || got["token"] != "tok" || got["device_flag"] != float64(2) || got["device_level"] != float64(1) || len(*ev) != 1 || (*ev)[0] != "update_token:ok" {
		t.Fatalf("got %v events %v", got, *ev)
	}
}

func TestRetriesServerErrorsThenSucceeds(t *testing.T) {
	var n atomic.Int32
	c, ev := client(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) < 3 {
			http.Error(w, `{"msg":"busy"}`, 503)
			return
		}
		w.Write([]byte(`{"status":200}`))
	})
	if err := c.DeviceQuit(context.Background(), "u1", 255); err != nil || n.Load() != 3 || (*ev)[0] != "device_quit:retried_ok" {
		t.Fatalf("expected success on 3rd attempt: err=%v calls=%d ev=%v", err, n.Load(), *ev)
	}
}

func TestGivesUpAfterRetriesAndReportsError(t *testing.T) {
	var n atomic.Int32
	c, ev := client(t, func(w http.ResponseWriter, r *http.Request) { n.Add(1); http.Error(w, "down", 500) })
	err := c.SetUserSendBan(context.Background(), "u1", true)
	var ae *imgw.APIError
	if !errors.As(err, &ae) || ae.Status != 500 || n.Load() != 3 || (*ev)[0] != "send_ban:error" {
		t.Fatalf("err=%v calls=%d ev=%v", err, n.Load(), *ev)
	}
}

func TestClientErrorsAreNotRetried(t *testing.T) {
	var n atomic.Int32
	c, _ := client(t, func(w http.ResponseWriter, r *http.Request) { n.Add(1); http.Error(w, `{"msg":"bad uid"}`, 400) })
	err := c.UpdateToken(context.Background(), "u", "t", 1, 0)
	var ae *imgw.APIError
	if !errors.As(err, &ae) || ae.Status != 400 || ae.Msg != "bad uid" || n.Load() != 1 {
		t.Fatalf("4xx must fail fast: err=%v calls=%d", err, n.Load())
	}
}

func TestTransportErrorsAreRetried(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	c, _ := imgw.New(url)
	var slept int
	c.Sleep = func(time.Duration) { slept++ }
	if err := c.Health(context.Background()); err == nil || slept != 2 {
		t.Fatalf("connection errors must be retried twice then fail: err=%v slept=%d", err, slept)
	}
}

func TestContextCancellationStopsRetries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var n atomic.Int32
	c, _ := client(t, func(w http.ResponseWriter, r *http.Request) { n.Add(1); cancel(); http.Error(w, "x", 500) })
	if err := c.Health(ctx); err == nil || n.Load() != 1 {
		t.Fatalf("cancelled context must stop retries: err=%v calls=%d", err, n.Load())
	}
}

func TestSendRequiresClientMsgNoForIdempotentRetries(t *testing.T) {
	c, _ := client(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"client_msg_no":"c1","message_id":42,"message_seq":7,"reason":1}`))
	})
	if _, err := c.Send(context.Background(), "u1", "u2", 1, "", []byte("x")); err == nil {
		t.Fatal("empty client_msg_no must be refused")
	}
	r, err := c.Send(context.Background(), "u1", "u2", 1, "c1", []byte("x"))
	if err != nil || r.MessageID != 42 || r.MessageSeq != 7 || r.ClientMsgNo != "c1" {
		t.Fatalf("got %+v %v", r, err)
	}
	if err := c.UpdateToken(context.Background(), "", "t", 1, 0); err == nil {
		t.Fatal("empty uid must be refused")
	}
}

func TestHTTP200WithFailureReasonIsAnError(t *testing.T) {
	c, ev := client(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"client_msg_no":"c1","message_id":0,"message_seq":0,"reason":25}`))
	})
	_, err := c.Send(context.Background(), "u1", "u2", 1, "c1", []byte("x"))
	var rej *imgw.SendRejected
	if !errors.As(err, &rej) || rej.Reason != 25 || (*ev)[0] != "send:ok" {
		t.Fatalf("a non-success reason must surface as *SendRejected (transport ok, business refusal): %v %v", err, *ev)
	}
}

func TestTraceIDPropagatesToTheIM(t *testing.T) {
	var rid, tp string
	c, _ := client(t, func(w http.ResponseWriter, r *http.Request) {
		rid, tp = r.Header.Get("X-Request-Id"), r.Header.Get("traceparent")
		w.Write([]byte(`{}`))
	})
	ctx := logx.WithTrace(context.Background(), "deadbeefcafe0042")
	if err := c.Health(ctx); err != nil {
		t.Fatal(err)
	}
	if rid != "deadbeefcafe0042" || tp != "00-0000000000000000deadbeefcafe0042-0000000000000001-01" {
		t.Fatalf("trace not propagated: %q %q", rid, tp)
	}
	rid, tp = "x", "x"
	_ = c.Health(logx.WithTrace(context.Background(), "not hex!"))
	if tp != "" {
		t.Fatalf("a non-hex id must not produce a traceparent, got %q", tp)
	}
	_ = c.Health(context.Background())
	if rid != "" {
		t.Fatalf("without a trace nothing is sent, got %q", rid)
	}
}

func TestDeviceQuitFansOutToEveryClusterNodeButOtherCallsDoNot(t *testing.T) {
	var quits, tokens [3]atomic.Int32
	mk := func(i int, failQuit bool) *httptest.Server {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/user/device_quit":
				quits[i].Add(1)
				if failQuit {
					http.Error(w, "node down", http.StatusServiceUnavailable)
					return
				}
			case "/user/token":
				tokens[i].Add(1)
			}
			w.Write([]byte(`{"status":200}`))
		}))
		t.Cleanup(s.Close)
		return s
	}
	n0, n1, n2 := mk(0, false), mk(1, false), mk(2, true)
	c, err := imgw.New(n0.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.Sleep = func(time.Duration) {}
	c.Retries = 0
	if err := c.AddPeers(n1.URL); err != nil {
		t.Fatal(err)
	}
	if err := c.DeviceQuit(context.Background(), "u1", 1); err != nil || quits[0].Load() != 1 || quits[1].Load() != 1 {
		t.Fatalf("every node must get the kick: %v %d %d", err, quits[0].Load(), quits[1].Load())
	}
	if err := c.UpdateToken(context.Background(), "u1", "t", 1, 1); err != nil || tokens[0].Load() != 1 || tokens[1].Load() != 0 {
		t.Fatalf("tokens are cluster-wide state: only the primary is called: %v %d %d", err, tokens[0].Load(), tokens[1].Load())
	}
	// one node failing makes the whole call fail (the worker retries), while the healthy nodes were still kicked
	if err := c.AddPeers(n2.URL); err != nil {
		t.Fatal(err)
	}
	if err := c.DeviceQuit(context.Background(), "u1", 1); err == nil || quits[2].Load() < 1 || quits[0].Load() != 2 {
		t.Fatalf("a failing node must fail the call after the others were kicked: %v %d", err, quits[2].Load())
	}
	// peers are validated like the primary: never a public address
	if err := c.AddPeers("http://8.8.8.8:5001"); !errors.Is(err, imgw.ErrNotPrivate) {
		t.Fatalf("public peer must be refused: %v", err)
	}
}
