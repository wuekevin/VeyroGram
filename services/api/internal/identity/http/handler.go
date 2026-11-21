// Package identityhttp serves /v1/auth/*. It is mounted only when the auth.sms_login flag is on.
package identityhttp

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"veyrogram/api/internal/identity/app"
	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/platform/authn"
	"veyrogram/api/internal/platform/clientip"
	"veyrogram/api/internal/platform/httpx"
	"veyrogram/api/internal/platform/token"
)

type Handler struct {
	Svc     *app.Service
	Trusted []*net.IPNet // AD-16
	Log     *slog.Logger
	Auth    func(http.Handler) http.Handler // authn.Require; the device/IM routes are mounted only when set
}

// Mount registers the routes on mux.
func (h *Handler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/auth/sms/code", h.requestCode)
	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/refresh", h.refresh)
	if h.Auth != nil {
		mux.Handle("POST /v1/auth/logout", h.Auth(http.HandlerFunc(h.logout)))
		mux.Handle("POST /v1/auth/logout-all", h.Auth(http.HandlerFunc(h.logoutAll)))
		mux.Handle("GET /v1/devices", h.Auth(http.HandlerFunc(h.listDevices)))
		mux.Handle("DELETE /v1/devices/{id}", h.Auth(http.HandlerFunc(h.revokeDevice)))
		mux.Handle("GET /v1/im/token", h.Auth(http.HandlerFunc(h.imToken)))
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil || dec.More() {
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "malformed request body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store") // responses may carry credentials
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	var le *domain.LimitError
	switch {
	case errors.As(err, &le):
		secs := int(le.RetryAfter.Seconds())
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
		w.Header().Set("RateLimit-Remaining", "0")
		w.Header().Set("RateLimit-Reset", strconv.Itoa(secs))
		code, msg := "rate_limited", "too many requests, try again later"
		if le.Locked {
			code, msg = "too_many_attempts", "too many failed attempts, try again later"
		}
		httpx.WriteError(w, r, http.StatusTooManyRequests, code, msg)
	case errors.Is(err, domain.ErrNotFound):
		httpx.WriteError(w, r, http.StatusNotFound, "not_found", "no such resource")
	case errors.Is(err, domain.ErrIMUnavailable):
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "im_unavailable", "the messaging service is unavailable, try again later")
	case errors.Is(err, domain.ErrInvalidInput):
		httpx.WriteError(w, r, http.StatusBadRequest, "invalid_request", "invalid request")
	case errors.Is(err, domain.ErrInvalidCode):
		httpx.WriteError(w, r, http.StatusUnauthorized, "invalid_code", "the verification code is invalid or expired")
	case errors.Is(err, domain.ErrInvalidRefresh):
		httpx.WriteError(w, r, http.StatusUnauthorized, "invalid_refresh_token", "the refresh token is invalid, expired or revoked")
	case errors.Is(err, domain.ErrDisabled):
		httpx.WriteError(w, r, http.StatusForbidden, "account_disabled", "this account is disabled")
	case errors.Is(err, domain.ErrSMSUnavailable):
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "sms_unavailable", "could not send the verification code, try again later")
	default:
		if h.Log != nil {
			h.Log.Error("auth request failed", "err", err.Error(), "trace_id", r.Header.Get("X-Request-Id"))
		}
		httpx.WriteError(w, r, http.StatusInternalServerError, "internal", "internal error")
	}
}

func (h *Handler) ip(r *http.Request) string { return clientip.From(r, h.Trusted) }

// POST /v1/auth/sms/code {phone, device_id} -> 202. The answer is identical for registered and unregistered numbers.
func (h *Handler) requestCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone    string `json:"phone"`
		DeviceID string `json:"device_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	err := h.Svc.RequestCode(r.Context(), app.RequestInput{Phone: in.Phone, DeviceKey: in.DeviceID, IP: h.ip(r), TraceID: r.Header.Get("X-Request-Id")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p := h.Svc.Policy
	writeJSON(w, http.StatusAccepted, map[string]any{"status": "sent", "expires_in": int(p.CodeTTL.Seconds()), "resend_after": int(p.SendCooldown.Seconds())})
}

type tokensOut struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	UserID       string `json:"user_id"`
	DeviceID     string `json:"device_id"`
	IsNewUser    bool   `json:"is_new_user"`
}

func out(t app.Tokens) tokensOut {
	return tokensOut{t.Access, "Bearer", t.ExpiresIn, t.Refresh, t.UserUID, strconv.FormatUint(t.DeviceID, 10), t.IsNew}
}

// POST /v1/auth/login {phone, code, device:{id,class,name}}
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Phone  string `json:"phone"`
		Code   string `json:"code"`
		Device struct {
			ID    string `json:"id"`
			Class string `json:"class"`
			Name  string `json:"name"`
		} `json:"device"`
	}
	if !decode(w, r, &in) {
		return
	}
	dev, err := domain.ParseDevice(in.Device.ID, in.Device.Class, in.Device.Name)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.Svc.Login(r.Context(), app.LoginInput{Phone: in.Phone, Code: in.Code, Device: dev, IP: h.ip(r), TraceID: r.Header.Get("X-Request-Id")})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out(t))
}

// POST /v1/auth/refresh {refresh_token}
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &in) {
		return
	}
	t, err := h.Svc.Refresh(r.Context(), in.RefreshToken, h.ip(r), r.Header.Get("X-Request-Id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out(t))
}

func (h *Handler) claims(r *http.Request) token.Claims {
	c, _ := authn.FromContext(r.Context())
	return c
}

// POST /v1/auth/logout: revokes the calling device (its refresh tokens, access tokens and IM token).
func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	if err := h.Svc.Logout(r.Context(), h.claims(r), h.ip(r), r.Header.Get("X-Request-Id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/auth/logout-all: revokes every device of the account (refresh tokens, access tokens, IM sessions).
func (h *Handler) logoutAll(w http.ResponseWriter, r *http.Request) {
	n, err := h.Svc.LogoutAll(r.Context(), h.claims(r), h.ip(r), r.Header.Get("X-Request-Id"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"revoked_devices": n})
}

type deviceOut struct {
	ID         string `json:"id"`
	Class      string `json:"class"`
	Name       string `json:"name"`
	CreatedAt  string `json:"created_at"`
	LastSeenAt string `json:"last_seen_at"`
	Current    bool   `json:"current"`
}

// GET /v1/devices
func (h *Handler) listDevices(w http.ResponseWriter, r *http.Request) {
	ds, err := h.Svc.Devices(r.Context(), h.claims(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items := make([]deviceOut, len(ds))
	for i, d := range ds {
		items[i] = deviceOut{strconv.FormatUint(d.ID, 10), string(d.Class), d.Name, d.CreatedAt.UTC().Format(time.RFC3339), d.LastSeenAt.UTC().Format(time.RFC3339), d.Current}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// DELETE /v1/devices/{id}
func (h *Handler) revokeDevice(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
	if err != nil {
		h.fail(w, r, domain.ErrNotFound)
		return
	}
	if err := h.Svc.RevokeDevice(r.Context(), h.claims(r), id, h.ip(r), r.Header.Get("X-Request-Id")); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/im/token
func (h *Handler) imToken(w http.ResponseWriter, r *http.Request) {
	c, err := h.Svc.IMToken(r.Context(), h.claims(r))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"uid": c.UID, "token": c.Token, "device_flag": c.Flag})
}
