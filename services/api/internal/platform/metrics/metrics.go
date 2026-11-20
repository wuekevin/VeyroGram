// Package metrics is a dependency-free Prometheus text-format (0.0.4) registry: counters, gauges and histograms with labels,
// plus the platform placeholder metrics required by Story 1.5. The endpoint is served only on the internal listener behind a bearer token.
package metrics

import (
	"crypto/subtle"
	"database/sql"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type kind string

const (
	counterK kind = "counter"
	gaugeK   kind = "gauge"
	histK    kind = "histogram"
)

type family struct {
	name, help string
	kind       kind
	buckets    []float64
	mu         sync.Mutex
	series     map[string]*series
	labelNames []string
}

type series struct {
	labels  []string
	value   float64
	counts  []uint64 // histogram buckets (non-cumulative), last is +Inf
	sum     float64
	observe uint64
}

// Registry holds metric families.
type Registry struct {
	mu      sync.Mutex
	fams    map[string]*family
	order   []string
	pollers []func()
}

func NewRegistry() *Registry { return &Registry{fams: map[string]*family{}} }

func (r *Registry) add(name, help string, k kind, buckets []float64, labels []string) *family {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f, ok := r.fams[name]; ok {
		return f
	}
	f := &family{name: name, help: help, kind: k, buckets: buckets, labelNames: labels, series: map[string]*series{}}
	r.fams[name] = f
	r.order = append(r.order, name)
	return f
}

// Counter / Gauge / Histogram handles.
type Counter struct{ f *family }
type Gauge struct{ f *family }
type Histogram struct{ f *family }

func (r *Registry) Counter(name, help string, labels ...string) Counter {
	return Counter{r.add(name, help, counterK, nil, labels)}
}
func (r *Registry) Gauge(name, help string, labels ...string) Gauge {
	return Gauge{r.add(name, help, gaugeK, nil, labels)}
}
func (r *Registry) Histogram(name, help string, buckets []float64, labels ...string) Histogram {
	return Histogram{r.add(name, help, histK, buckets, labels)}
}

// OnScrape registers a function run before each exposition (e.g. refresh DB pool gauges).
func (r *Registry) OnScrape(fn func()) {
	r.mu.Lock()
	r.pollers = append(r.pollers, fn)
	r.mu.Unlock()
}

func (f *family) get(vals []string) *series {
	if len(vals) != len(f.labelNames) {
		panic(fmt.Sprintf("metrics: %s expects %d label values, got %d", f.name, len(f.labelNames), len(vals)))
	}
	key := strings.Join(vals, "\xff")
	s := f.series[key]
	if s == nil {
		s = &series{labels: append([]string(nil), vals...)}
		if f.kind == histK {
			s.counts = make([]uint64, len(f.buckets)+1)
		}
		f.series[key] = s
	}
	return s
}

func (c Counter) Add(v float64, labels ...string) {
	if v < 0 {
		return
	}
	c.f.mu.Lock()
	c.f.get(labels).value += v
	c.f.mu.Unlock()
}
func (c Counter) Inc(labels ...string) { c.Add(1, labels...) }
func (g Gauge) Set(v float64, labels ...string) {
	g.f.mu.Lock()
	g.f.get(labels).value = v
	g.f.mu.Unlock()
}
func (g Gauge) Add(v float64, labels ...string) {
	g.f.mu.Lock()
	g.f.get(labels).value += v
	g.f.mu.Unlock()
}
func (h Histogram) Observe(v float64, labels ...string) {
	h.f.mu.Lock()
	defer h.f.mu.Unlock()
	s := h.f.get(labels)
	i := sort.SearchFloat64s(h.f.buckets, v)
	s.counts[i]++
	s.sum += v
	s.observe++
}

// Touch creates a zero-valued series so a placeholder metric is visible before its first event.
func (c Counter) Touch(labels ...string) { c.Add(0, labels...) }
func (g Gauge) Touch(labels ...string)   { g.Add(0, labels...) }

func esc(s string) string {
	return strings.NewReplacer(`\`, `\\`, "\n", `\n`, `"`, `\"`).Replace(s)
}

func lbl(names, vals []string, extra string) string {
	var parts []string
	for i, n := range names {
		parts = append(parts, fmt.Sprintf(`%s="%s"`, n, esc(vals[i])))
	}
	if extra != "" {
		parts = append(parts, extra)
	}
	if len(parts) == 0 {
		return ""
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func num(v float64) string {
	if math.IsInf(v, 1) {
		return "+Inf"
	}
	return fmt.Sprintf("%g", v)
}

// Write renders the exposition.
func (r *Registry) Write(w io.Writer) {
	r.mu.Lock()
	pollers := append([]func(){}, r.pollers...)
	names := append([]string(nil), r.order...)
	r.mu.Unlock()
	for _, p := range pollers {
		p()
	}
	for _, n := range names {
		f := r.fams[n]
		f.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", f.name, esc(f.help), f.name, f.kind)
		keys := make([]string, 0, len(f.series))
		for k := range f.series {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			s := f.series[k]
			if f.kind != histK {
				fmt.Fprintf(w, "%s%s %s\n", f.name, lbl(f.labelNames, s.labels, ""), num(s.value))
				continue
			}
			var cum uint64
			for i, b := range f.buckets {
				cum += s.counts[i]
				fmt.Fprintf(w, "%s_bucket%s %d\n", f.name, lbl(f.labelNames, s.labels, `le="`+num(b)+`"`), cum)
			}
			cum += s.counts[len(f.buckets)]
			fmt.Fprintf(w, "%s_bucket%s %d\n", f.name, lbl(f.labelNames, s.labels, `le="+Inf"`), cum)
			fmt.Fprintf(w, "%s_sum%s %s\n%s_count%s %d\n", f.name, lbl(f.labelNames, s.labels, ""), num(s.sum), f.name, lbl(f.labelNames, s.labels, ""), s.observe)
		}
		f.mu.Unlock()
	}
}

// Handler serves the exposition for GET only, requiring "Authorization: Bearer <token>" (constant-time compare).
// An empty token means the endpoint is disabled and answers 404, never open.
func (r *Registry) Handler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if token == "" {
			http.NotFound(w, req)
			return
		}
		if req.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		got := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		if !strings.HasPrefix(req.Header.Get("Authorization"), "Bearer ") || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		r.Write(w)
	})
}

// Platform holds the placeholder metrics Story 1.5 requires; later stories feed them.
type Platform struct {
	HTTPRequests    Counter
	HTTPDuration    Histogram
	DeliveryLat     Histogram // message delivery latency (seconds)
	Connections     Gauge     // IM connections
	AckTotal        Counter   // result=acked|unacked
	PushErrors      Counter   // push/file/risk/openapi error counters share one shape
	FileErrors      Counter
	RiskErrors      Counter
	OpenAPIErrors   Counter
	OutboxLag       Gauge
	IMGWCalls       Counter   // op, result
	IMGWDuration    Histogram // op
	IMWebhookReject Counter   // reason
	OutboxEvents    Counter   // topic, result (done|retry|dead) - fed by the worker process
}

// NewPlatform registers the platform metrics and Go/DB gauges on r.
func NewPlatform(r *Registry) *Platform {
	p := &Platform{
		HTTPRequests:    r.Counter("vg_http_requests_total", "HTTP requests by method, route class and status class.", "method", "class"),
		HTTPDuration:    r.Histogram("vg_http_request_duration_seconds", "HTTP request latency.", []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}, "method"),
		DeliveryLat:     r.Histogram("vg_message_delivery_latency_seconds", "Message delivery latency (placeholder until the IM path lands).", []float64{.05, .1, .25, .5, 1, 2, 5}),
		Connections:     r.Gauge("vg_im_connections", "Current IM connections (placeholder)."),
		AckTotal:        r.Counter("vg_message_ack_total", "Message ACK outcomes (placeholder).", "result"),
		PushErrors:      r.Counter("vg_push_errors_total", "Push provider errors (placeholder).", "provider"),
		FileErrors:      r.Counter("vg_file_errors_total", "File service errors (placeholder).", "op"),
		RiskErrors:      r.Counter("vg_risk_errors_total", "Risk-control errors (placeholder).", "rule"),
		OpenAPIErrors:   r.Counter("vg_openapi_errors_total", "Open API errors (placeholder).", "code"),
		IMGWCalls:       r.Counter("vg_imgw_calls_total", "Calls from the imgw adapter to the IM API by op and result.", "op", "result"),
		IMGWDuration:    r.Histogram("vg_imgw_call_duration_seconds", "imgw adapter call latency including retries.", []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}, "op"),
		IMWebhookReject: r.Counter("vg_im_webhook_rejected_total", "IM callbacks rejected before reaching policy (bad secret, source, shape).", "reason"),
		OutboxEvents:    r.Counter("vg_outbox_events_total", "Outbox events handled by the worker, by topic and result.", "topic", "result"),
		OutboxLag:       r.Gauge("vg_outbox_pending", "Pending outbox rows (placeholder until Story 1.4)."),
	}
	p.Connections.Touch()
	p.AckTotal.Touch("acked")
	p.AckTotal.Touch("unacked")
	p.PushErrors.Touch("none")
	p.FileErrors.Touch("none")
	p.RiskErrors.Touch("none")
	p.OpenAPIErrors.Touch("none")
	p.OutboxLag.Touch()
	p.IMGWCalls.Touch("health", "ok")
	p.IMWebhookReject.Touch("bad_secret")
	p.OutboxEvents.Touch("none", "done")
	return p
}

// DBPool exports database/sql pool stats on each scrape.
func DBPool(r *Registry, db *sql.DB) {
	open := r.Gauge("vg_db_pool_open_connections", "Open DB connections.")
	inuse := r.Gauge("vg_db_pool_in_use_connections", "DB connections in use.")
	wait := r.Counter("vg_db_pool_wait_total", "Total waits for a DB connection.")
	var last int64
	r.OnScrape(func() {
		s := db.Stats()
		open.Set(float64(s.OpenConnections))
		inuse.Set(float64(s.InUse))
		if d := s.WaitCount - last; d > 0 {
			wait.Add(float64(d))
			last = s.WaitCount
		}
	})
}

// Middleware records request count and latency; route class is the first path segments only (bounded cardinality).
func (p *Platform) Middleware(next http.Handler, clock func() float64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := clock()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		p.HTTPRequests.Inc(r.Method, fmt.Sprintf("%dxx", sw.status/100))
		p.HTTPDuration.Observe(clock()-start, r.Method)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }
