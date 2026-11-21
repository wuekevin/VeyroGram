package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/identity/ports"
)

// HTTPSender posts {"to": "+E164", "message": "..."} as JSON to a gateway with a bearer token. Any 2xx is success.
// The gateway (or a thin shim in front of the vendor chosen in OQ-8) owns vendor specifics such as templates and signatures.
// Errors never include the response body or the number.
type HTTPSender struct {
	URL, Token string
	Client     *http.Client
}

var _ ports.SMSSender = (*HTTPSender)(nil)

func (h *HTTPSender) Send(ctx context.Context, phone domain.Phone, message string) error {
	body, _ := json.Marshal(map[string]string{"to": string(phone), "message": message})
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
	c := h.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("sms gateway unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("sms gateway answered %d", resp.StatusCode)
	}
	return nil
}

// LogSender writes the message to the log. Development/test only; configuration refuses it in staging and production.
type LogSender struct{ Log *slog.Logger }

func (l LogSender) Send(_ context.Context, phone domain.Phone, message string) error {
	l.Log.Warn("DEV SMS (not sent)", "to_last4", phone.Last4(), "message", message)
	return nil
}
