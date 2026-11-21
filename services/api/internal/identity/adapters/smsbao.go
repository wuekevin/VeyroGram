package adapters

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/identity/ports"
)

// SMSBao sends through 短信宝 (https://www.smsbao.com): mainland numbers (+86 followed by 11 digits) use the domestic API
// GET /sms with the bare 11-digit number, every other country uses the international API GET /wsms with the "+" number URL-encoded.
// Both answer a plain "0" on success and a numeric error code otherwise. The message must already carry the registered signature
// (e.g. 【VeyroGram】) because the domestic channel requires one.
// Credentials travel in the query string by the vendor's design, so errors are built without the request URL (Go's own error would include it).
type SMSBao struct {
	User, Key string
	BaseURL   string // default https://api.smsbao.com
	Client    *http.Client
}

var _ ports.SMSSender = (*SMSBao)(nil)

// smsbaoErrors are the documented codes worth naming in logs; unknown codes are reported as numbers only.
var smsbaoErrors = map[string]string{
	"30": "bad credentials", "40": "account not found", "41": "insufficient balance", "43": "caller IP not allowed",
	"50": "content rejected (sensitive words/signature)", "51": "phone number rejected",
}

func (s *SMSBao) Send(ctx context.Context, phone domain.Phone, message string) error {
	base := strings.TrimRight(s.BaseURL, "/")
	if base == "" {
		base = "https://api.smsbao.com"
	}
	path, m := "/wsms", string(phone)
	if strings.HasPrefix(m, "+86") && len(m) == 14 { // +86 + 11 digits
		path, m = "/sms", m[3:]
	}
	q := url.Values{"u": {s.User}, "p": {s.Key}, "m": {m}, "c": {message}}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path+"?"+q.Encode(), nil)
	if err != nil {
		return fmt.Errorf("smsbao: bad request")
	}
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("smsbao: gateway unreachable")
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	code := strings.TrimSpace(string(b))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("smsbao: http %d", resp.StatusCode)
	}
	if code == "0" {
		return nil
	}
	if len(code) > 8 { // not a code: never echo arbitrary vendor text
		code = "?"
	}
	if why, ok := smsbaoErrors[code]; ok {
		return fmt.Errorf("smsbao: error %s (%s)", code, why)
	}
	return fmt.Errorf("smsbao: error %s", code)
}
