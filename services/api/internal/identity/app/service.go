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
	"veyrogram/api/internal/platform/denylist"
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
	Deny    denylist.List             // optional: revoked-device deny-list (AD-9); nil = revocation relies on token expiry
	IM      ports.IM                  // WuKongIM gateway for token sync and kicks
	IMKey   []byte                    // HMAC key deriving IM tokens
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
		s.denyDevice(ctx, sess.DeviceID) // the whole chain is dead: stop its access tokens too
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

// denyDevice blocks the device's access tokens until they would have expired anyway. A failure is reported, not fatal:
// the refresh tokens are already revoked in the database, so the worst case is the access-token TTL (<= 15 min).
func (s *Service) denyDevice(ctx context.Context, deviceID uint64) {
	if s.Deny == nil {
		s.observe("deny", "unconfigured")
		return
	}
	if err := s.Deny.Deny(ctx, s.tenant(), deviceID, s.Policy.AccessTTL+time.Minute); err != nil {
		s.observe("deny", "error")
		return
	}
	s.observe("deny", "ok")
}

// Device is one live device of the caller.
type Device struct {
	ID         uint64
	Class      domain.DeviceClass
	Name       string
	CreatedAt  time.Time
	LastSeenAt time.Time
	Current    bool
}

func (s *Service) Devices(ctx context.Context, c token.Claims) ([]Device, error) {
	ds, err := s.Store.ListDevices(ctx, s.tenant(), c.Subject)
	if err != nil {
		return nil, err
	}
	out := make([]Device, len(ds))
	for i, d := range ds {
		out[i] = Device{d.ID, d.Class, d.Name, d.CreatedAt, d.LastSeenAt, strconv.FormatUint(d.ID, 10) == c.Device}
	}
	return out, nil
}

// RevokeDevice revokes one of the caller's own devices (including the current one, which is logout). Revoking someone else's device or
// an unknown id answers domain.ErrNotFound. The IM side (new class token + kick) is queued in the same transaction and done by the worker.
func (s *Service) RevokeDevice(ctx context.Context, c token.Claims, deviceID uint64, ip, traceID string) error {
	rd, found, err := s.Store.RevokeDevice(ctx, s.tenant(), c.Subject, deviceID, s.now())
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrNotFound
	}
	s.denyDevice(ctx, rd.DeviceID)
	s.event(ctx, ports.Event{Kind: "device", Result: "revoked", IP: ip, TraceID: traceID, DeviceKey: strconv.FormatUint(deviceID, 10)})
	return nil
}

// Logout revokes the device the access token belongs to.
func (s *Service) Logout(ctx context.Context, c token.Claims, ip, traceID string) error {
	id, err := strconv.ParseUint(c.Device, 10, 64)
	if err != nil {
		return domain.ErrNotFound
	}
	return s.RevokeDevice(ctx, c, id, ip, traceID)
}

// IMCredentials is what a client needs to connect to WuKongIM.
type IMCredentials struct {
	UID   string
	Token string
	Flag  uint8
}

func (s *Service) imToken(uid string, class domain.DeviceClass, epoch uint64) string {
	m := hmac.New(sha256.New, s.IMKey)
	m.Write([]byte("im\x00" + uid + "\x00" + string(class) + "\x00" + strconv.FormatUint(epoch, 10)))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

// IMToken returns the connection token for the caller's device class and (re-)registers it with the IM, so a client kicked by a revocation of
// a same-class device can simply fetch it again. The database, not only the deny-list, decides whether the device is still active.
func (s *Service) IMToken(ctx context.Context, c token.Claims) (IMCredentials, error) {
	id, err := strconv.ParseUint(c.Device, 10, 64)
	if err != nil {
		return IMCredentials{}, domain.ErrNotFound
	}
	cr, err := s.Store.IMCreds(ctx, s.tenant(), c.Subject, id)
	if err != nil {
		return IMCredentials{}, err
	}
	if !cr.Active {
		return IMCredentials{}, domain.ErrNotFound
	}
	tok := s.imToken(c.Subject, cr.Class, cr.Epoch)
	if err := s.IM.UpdateToken(ctx, c.Subject, tok, cr.Class.IMFlag(), 1); err != nil {
		return IMCredentials{}, domain.ErrIMUnavailable
	}
	return IMCredentials{UID: c.Subject, Token: tok, Flag: cr.Class.IMFlag()}, nil
}

// SyncIM is the outbox handler body for TopicIMSync: kick the class's sessions, then register the current token. It is idempotent.
func (s *Service) SyncIM(ctx context.Context, uid string, class domain.DeviceClass) error {
	epoch, err := s.Store.ClassEpoch(ctx, s.tenant(), uid, class)
	if err != nil {
		return err
	}
	if err := s.IM.DeviceQuit(ctx, uid, class.IMFlag()); err != nil { // clears the stored token and closes owner-local sessions (AD-9)
		return err
	}
	return s.IM.UpdateToken(ctx, uid, s.imToken(uid, class, epoch), class.IMFlag(), 1)
}
