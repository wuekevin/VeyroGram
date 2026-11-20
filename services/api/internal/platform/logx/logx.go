// Package logx builds the structured JSON logger (Story 1.5, T-17). A redacting handler removes secrets by attribute key and by value pattern
// (bearer tokens, JWTs, phone numbers) so that a careless log call cannot leak tokens, phone numbers or message bodies.
package logx

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const Redacted = "[REDACTED]"

// sensitiveKeys are matched case-insensitively as substrings of the attribute key.
var sensitiveKeys = []string{"token", "authorization", "password", "passwd", "secret", "cookie", "phone", "mobile", "msisdn", "sms_code", "verify_code", "otp", "dsn", "api_key", "apikey", "private_key",
	"payload", "content", "message_body", "msg_body", "text_body"}

var (
	bearerRE = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`)
	jwtRE    = regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`)
	phoneRE  = regexp.MustCompile(`(^|[^0-9A-Za-z])\+?\d[\d\s().-]{8,16}\d`)
	dsnRE    = regexp.MustCompile(`[A-Za-z0-9_.-]+:[^@\s/]+@(tcp|unix)\(`)
)

func sensitiveKey(k string) bool {
	l := strings.ToLower(k)
	for _, s := range sensitiveKeys {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// Scrub removes secret-looking substrings from a string value.
func Scrub(s string) string {
	s = bearerRE.ReplaceAllString(s, "$1 "+Redacted)
	s = jwtRE.ReplaceAllString(s, Redacted)
	s = dsnRE.ReplaceAllString(s, Redacted+"@$1(")
	s = phoneRE.ReplaceAllStringFunc(s, func(m string) string {
		digits := 0
		for _, c := range m {
			if c >= '0' && c <= '9' {
				digits++
			}
		}
		if digits < 10 { // dates, versions, short ids are not phone numbers
			return m
		}
		// keep the leading separator character, drop the number
		if len(m) > 0 && !(m[0] == '+' || (m[0] >= '0' && m[0] <= '9')) {
			return m[:1] + Redacted
		}
		return Redacted
	})
	return s
}

func redactAttr(a slog.Attr) slog.Attr {
	a.Value = a.Value.Resolve()
	if sensitiveKey(a.Key) {
		return slog.String(a.Key, Redacted)
	}
	switch a.Value.Kind() {
	case slog.KindGroup:
		g := a.Value.Group()
		out := make([]slog.Attr, len(g))
		for i, x := range g {
			out[i] = redactAttr(x)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(out...)}
	case slog.KindString:
		return slog.String(a.Key, Scrub(a.Value.String()))
	case slog.KindAny:
		switch v := a.Value.Any().(type) {
		case error:
			return slog.String(a.Key, Scrub(v.Error()))
		case []byte:
			return slog.String(a.Key, Redacted)
		default:
			return slog.String(a.Key, Scrub(strings.TrimSpace(sprint(v))))
		}
	}
	return a
}

type ctxKey struct{}

// WithTrace stores the trace id in the context; the handler adds it to every record.
func WithTrace(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// Trace returns the trace id or "".
func Trace(ctx context.Context) string { s, _ := ctx.Value(ctxKey{}).(string); return s }

type handler struct{ inner slog.Handler }

func (h handler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }
func (h handler) Handle(ctx context.Context, r slog.Record) error {
	nr := slog.NewRecord(r.Time, r.Level, Scrub(r.Message), r.PC)
	if id := Trace(ctx); id != "" {
		nr.AddAttrs(slog.String("trace_id", id))
	}
	r.Attrs(func(a slog.Attr) bool { nr.AddAttrs(redactAttr(a)); return true })
	return h.inner.Handle(ctx, nr)
}
func (h handler) WithAttrs(as []slog.Attr) slog.Handler {
	out := make([]slog.Attr, len(as))
	for i, a := range as {
		out[i] = redactAttr(a)
	}
	return handler{h.inner.WithAttrs(out)}
}
func (h handler) WithGroup(n string) slog.Handler { return handler{h.inner.WithGroup(n)} }

// New returns a JSON logger writing to w with redaction and trace ids.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(handler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

// Middleware stores X-Request-Id as the trace id in the request context and writes one access-log line per request. It logs the path
// but never the query string or headers (which may carry tokens).
func Middleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithTrace(r.Context(), r.Header.Get("X-Request-Id"))
			start := time.Now()
			sw := &sw{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r.WithContext(ctx))
			log.LogAttrs(ctx, slog.LevelInfo, "http_request", slog.String("method", r.Method), slog.String("path", r.URL.Path),
				slog.Int("status", sw.status), slog.Duration("duration", time.Since(start)))
		})
	}
}

type sw struct {
	http.ResponseWriter
	status int
}

func (s *sw) WriteHeader(c int) { s.status = c; s.ResponseWriter.WriteHeader(c) }
