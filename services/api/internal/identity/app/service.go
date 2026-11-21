// Package app implements phone/SMS registration and login, rotating refresh tokens and the login risk controls (Story 1.12).
package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"veyrogram/api/internal/identity/domain"
	"veyrogram/api/internal/identity/ports"
	"veyrogram/api/internal/platform/token"
)

// Service is safe for concurrent use.
type Service struct {
	Store   ports.Store
	SMS     ports.SMSSender
	Keys    *token.Ring
	HashKey []byte
	Policy  domain.Policy
	Tenant  uint64
	Now     func() time.Time
	Observe func(kind, result string) // metrics hook; must not block
}

// Tokens is the credential bundle returned by login and refresh.
type Tokens struct {
	Access    string
	ExpiresIn int
	Refresh   string
	UserUID   string
	DeviceID  uint64
	IsNew     bool
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) tenant() uint64 {
	if s.Tenant == 0 {
		return 1
	}
	return s.Tenant
}

func (s *Service) observe(kind, result string) {
	if s.Observe != nil {
		s.Observe(kind, result)
	}
}

func (s *Service) mac(parts ...string) string {
	m := hmac.New(sha256.New, s.HashKey)
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}
	return hex.EncodeToString(m.Sum(nil))
}

func (s *Service) phoneHash(p domain.Phone) string { return s.mac("phone", string(p)) }
func (s *Service) codeHash(ph, code string) string { return s.mac("code", ph, code) }

func (s *Service) event(ctx context.Context, e ports.Event) {
	e.Tenant, e.At = s.tenant(), s.now()
	if err := s.Store.AddEvent(ctx, e); err != nil {
		s.observe(e.Kind, "event_write_error")
		return
	}
	s.observe(e.Kind, e.Result)
}

// RequestInput identifies one anonymous call; IP must already be resolved through the trusted-proxy rules (AD-16).
type RequestInput struct {
	Phone     string
	DeviceKey string
	IP        string
	TraceID   string
}

// RequestCode sends a fresh random one-time code. The outcome never depends on whether the number is registered (no enumeration).
func (s *Service) RequestCode(ctx context.Context, in RequestInput) error {
	phone, err := domain.ParsePhone(in.Phone)
	if err != nil || !domain.ValidDeviceKey(in.DeviceKey) {
		return domain.ErrInvalidInput
	}
	ph, now, p := s.phoneHash(phone), s.now(), s.Policy
	ev := ports.Event{Kind: "sms_send", PhoneHash: ph, IP: in.IP, DeviceKey: in.DeviceKey, TraceID: in.TraceID}
	if wait, limited, err := s.sendLimited(ctx, ph, in, now); err != nil {
		return err
	} else if limited {
		ev.Result = "rate_limited"
		s.event(ctx, ev)
		return &domain.LimitError{RetryAfter: wait}
	}
	code, err := randomCode()
	if err != nil {
		return err
	}
	if err := s.Store.NewCode(ctx, s.tenant(), ph, s.codeHash(ph, code), now.Add(p.CodeTTL), now); err != nil {
		return err
	}
	msg := fmt.Sprintf(p.MessageTemplate, code, int(p.CodeTTL/time.Minute))
	if err := s.SMS.Send(ctx, phone, msg); err != nil {
		ev.Result = "send_error"
		s.event(ctx, ev)
		return domain.ErrSMSUnavailable
	}
	ev.Result = "ok"
	s.event(ctx, ev)
	return nil
}

func randomCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func (s *Service) sendLimited(ctx context.Context, ph string, in RequestInput, now time.Time) (time.Duration, bool, error) {
	p := s.Policy
	if last, ok, err := s.Store.LastEvent(ctx, "sms_send", "ok", ports.DimPhone, ph); err != nil {
		return 0, false, err
	} else if ok && now.Sub(last) < p.SendCooldown {
		return p.SendCooldown - now.Sub(last), true, nil
	}
	since := now.Add(-p.SendWindow)
	for _, c := range []struct {
		d     ports.Dim
		v     string
		limit int
	}{{ports.DimPhone, ph, p.SendPerPhone}, {ports.DimIP, in.IP, p.SendPerIP}, {ports.DimDevice, in.DeviceKey, p.SendPerDevice}, {ports.DimAll, "", p.SendGlobal}} {
		n, err := s.Store.CountEvents(ctx, "sms_send", "ok", c.d, c.v, since)
		if err != nil {
			return 0, false, err
		}
		if n >= c.limit {
			return p.SendWindow, true, nil
		}
	}
	return 0, false, nil
}

// LoginInput is one login attempt.
type LoginInput struct {
	Phone   string
	Code    string
	Device  domain.Device
	IP      string
	TraceID string
}

// Login verifies the code and, for a correct one, registers the user on first use and issues tokens.
// Wrong, expired, replayed and never-requested codes are indistinguishable to the caller (domain.ErrInvalidCode).
func (s *Service) Login(ctx context.Context, in LoginInput) (Tokens, error) {
	phone, err := domain.ParsePhone(in.Phone)
	if err != nil || len(in.Code) != 6 || !digits(in.Code) {
		return Tokens{}, domain.ErrInvalidInput
	}
	ph, now := s.phoneHash(phone), s.now()
	ev := ports.Event{Kind: "login", PhoneHash: ph, IP: in.IP, DeviceKey: in.Device.Key, TraceID: in.TraceID}
	if wait, locked, err := s.locked(ctx, ph, in, now); err != nil {
		return Tokens{}, err
	} else if locked {
		ev.Result = "locked"
		s.event(ctx, ev)
		return Tokens{}, &domain.LimitError{Locked: true, RetryAfter: wait}
	}
	fail := func() (Tokens, error) {
		ev.Result = "fail"
		s.event(ctx, ev)
		return Tokens{}, domain.ErrInvalidCode
	}
	c, ok, err := s.Store.LatestCode(ctx, s.tenant(), ph, now)
	if err != nil {
		return Tokens{}, err
	}
	if !ok || c.Attempts >= s.Policy.MaxCodeAttempts {
		return fail()
	}
	if subtle.ConstantTimeCompare([]byte(c.Hash), []byte(s.codeHash(ph, in.Code))) != 1 {
		if err := s.Store.BumpAttempt(ctx, c.ID); err != nil {
			return Tokens{}, err
		}
		return fail()
	}
	refresh, rhash, err := newRefresh()
	if err != nil {
		return Tokens{}, err
	}
	uid, err := randomHex(16)
	if err != nil {
		return Tokens{}, err
	}
	sess, err := s.Store.Login(ctx, ports.LoginTx{
		Tenant: s.tenant(), CodeID: c.ID, PhoneHash: ph, Last4: phone.Last4(), NewUID: uid, Device: in.Device,
		RefreshHash: rhash, RefreshExpires: now.Add(s.Policy.RefreshTTL), Now: now,
	})
	switch {
	case errors.Is(err, domain.ErrCodeConsumed):
		return fail() // replay (or a concurrent use of the same code)
	case errors.Is(err, domain.ErrDisabled):
		ev.Result = "disabled"
		s.event(ctx, ev)
		return Tokens{}, domain.ErrDisabled
	case err != nil:
		return Tokens{}, err
	}
	ev.Result, ev.UserID = "ok", sess.UserID
	s.event(ctx, ev)
	return s.bundle(sess, refresh, now)
}

func digits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *Service) locked(ctx context.Context, ph string, in LoginInput, now time.Time) (time.Duration, bool, error) {
	p := s.Policy
	since := now.Add(-p.FailWindow)
	for _, c := range []struct {
		d     ports.Dim
		v     string
		limit int
	}{{ports.DimPhone, ph, p.FailPerPhone}, {ports.DimIP, in.IP, p.FailPerIP}, {ports.DimDevice, in.Device.Key, p.FailPerDevice}} {
		n, err := s.Store.CountEvents(ctx, "login", "fail", c.d, c.v, since)
		if err != nil {
			return 0, false, err
		}
		if n >= c.limit {
			return p.FailWindow, true, nil
		}
	}
	return 0, false, nil
}

// Refresh rotates a refresh token. Presenting an already-rotated token revokes the device (reuse detection, AD-9).
func (s *Service) Refresh(ctx context.Context, refreshToken, ip, traceID string) (Tokens, error) {
	if len(refreshToken) < 20 || len(refreshToken) > 200 {
		return Tokens{}, domain.ErrInvalidRefresh
	}
	now := s.now()
	next, nhash, err := newRefresh()
	if err != nil {
		return Tokens{}, err
	}
	sess, res, err := s.Store.Rotate(ctx, s.tenant(), hashRefresh(refreshToken), nhash, now.Add(s.Policy.RefreshTTL), now)
	if err != nil {
		return Tokens{}, err
	}
	ev := ports.Event{Kind: "refresh", IP: ip, TraceID: traceID, UserID: sess.UserID}
	switch res {
	case ports.RotateOK:
		ev.Result = "ok"
		s.event(ctx, ev)
		return s.bundle(sess, next, now)
	case ports.RotateReuse:
		ev.Result = "reuse"
		s.event(ctx, ev)
	default:
		ev.Result = "invalid"
		s.event(ctx, ev)
	}
	return Tokens{}, domain.ErrInvalidRefresh
}

func (s *Service) bundle(sess ports.Session, refresh string, now time.Time) (Tokens, error) {
	jti, err := randomHex(8)
	if err != nil {
		return Tokens{}, err
	}
	ttl := s.Policy.AccessTTL
	access, err := s.Keys.Sign(token.Claims{
		Subject: sess.UID, Device: strconv.FormatUint(sess.DeviceID, 10), Class: string(sess.Class), Tenant: s.tenant(),
		ID: jti, IssuedAt: now.Unix(), Expires: now.Add(ttl).Unix(),
	})
	if err != nil {
		return Tokens{}, err
	}
	return Tokens{Access: access, ExpiresIn: int(ttl / time.Second), Refresh: refresh, UserUID: sess.UID, DeviceID: sess.DeviceID, IsNew: sess.IsNew}, nil
}

func newRefresh() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	token = "rt_" + base64.RawURLEncoding.EncodeToString(b)
	return token, hashRefresh(token), nil
}

func hashRefresh(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
