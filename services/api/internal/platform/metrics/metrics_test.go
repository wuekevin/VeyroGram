package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"veyrogram/api/internal/platform/metrics"
)

const tok = "metrics-token-0123456789abcdef"

func scrape(h http.Handler, method, auth string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/metrics", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRequiredPlaceholderMetricsExist(t *testing.T) {
	r := metrics.NewRegistry()
	metrics.NewPlatform(r)
	body := scrape(r.Handler(tok), "GET", "Bearer "+tok).Body.String()
	for _, name := range []string{"vg_message_delivery_latency_seconds", "vg_im_connections", "vg_message_ack_total", "vg_push_errors_total", "vg_file_errors_total",
		"vg_risk_errors_total", "vg_openapi_errors_total", "vg_http_requests_total", "vg_http_request_duration_seconds", "vg_outbox_pending"} {
		if !strings.Contains(body, "# TYPE "+name+" ") {
			t.Errorf("metric %s missing from /metrics", name)
		}
	}
}

func TestEndpointRequiresBearerTokenAndIsDisabledWithoutOne(t *testing.T) {
	r := metrics.NewRegistry()
	metrics.NewPlatform(r)
	h := r.Handler(tok)
	for _, c := range []struct {
		name, method, auth string
		want               int
	}{{"no auth", "GET", "", 401}, {"wrong token", "GET", "Bearer nope", 401}, {"prefix of token", "GET", "Bearer " + tok[:10], 401}, {"basic scheme", "GET", "Basic " + tok, 401},
		{"raw token without scheme", "GET", tok, 401}, {"POST", "POST", "Bearer " + tok, 405}, {"ok", "GET", "Bearer " + tok, 200}} {
		if got := scrape(h, c.method, c.auth).Code; got != c.want {
			t.Errorf("%s: got %d want %d", c.name, got, c.want)
		}
	}
	if got := scrape(r.Handler(""), "GET", "Bearer ").Code; got != 404 {
		t.Errorf("empty token must disable the endpoint (404), got %d", got)
	}
	if got := scrape(r.Handler(""), "GET", "").Code; got != 404 {
		t.Errorf("empty token must disable the endpoint (404), got %d", got)
	}
}

func TestExpositionFormat(t *testing.T) {
	r := metrics.NewRegistry()
	c := r.Counter("t_total", "a \"help\"", "l")
	c.Inc(`a"b`)
	c.Add(2, `a"b`)
	c.Add(-5, `a"b`) // counters never decrease
	h := r.Histogram("t_seconds", "h", []float64{1, 5})
	h.Observe(0.5)
	h.Observe(3)
	h.Observe(9)
	out := scrape(r.Handler(tok), "GET", "Bearer "+tok).Body.String()
	for _, want := range []string{`t_total{l="a\"b"} 3`, `t_seconds_bucket{le="1"} 1`, `t_seconds_bucket{le="5"} 2`, `t_seconds_bucket{le="+Inf"} 3`, "t_seconds_count 3", "t_seconds_sum 12.5"} {
		if !strings.Contains(out, want) {
			t.Errorf("exposition missing %q\n%s", want, out)
		}
	}
}

func TestMiddlewareCountsByStatusClass(t *testing.T) {
	r := metrics.NewRegistry()
	p := metrics.NewPlatform(r)
	now := 0.0
	h := p.Middleware(http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
		if q.URL.Path == "/boom" {
			w.WriteHeader(500)
		}
	}), func() float64 { now += 0.1; return now })
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/ok/123456", nil))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/boom", nil))
	out := scrape(r.Handler(tok), "GET", "Bearer "+tok).Body.String()
	if !strings.Contains(out, `vg_http_requests_total{method="GET",class="2xx"} 1`) || !strings.Contains(out, `vg_http_requests_total{method="GET",class="5xx"} 1`) {
		t.Fatalf("request counters wrong:\n%s", out)
	}
	if strings.Contains(out, "123456") {
		t.Fatal("paths must never become label values (cardinality / data leak)")
	}
}
