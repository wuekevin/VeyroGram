// Package app implements the admin plane use cases (Story 1.19): first-admin bootstrap, password + TOTP login, role-checked access, admin management and
// the read-only audit view. Every high-risk action records who, what, why and when in the hash-chained audit log BEFORE it takes effect.
package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/admin/ports"
	"veyrogram/api/internal/platform/audit"
)

type AuditLog interface {
	Append(ctx context.Context, e audit.Entry) error
	Verify(ctx context.Context) (audit.Head, error)
}

type Policy struct {
	ChallengeTTL time.Duration
	SessionTTL   time.Duration // absolute lifetime
	IdleTTL      time.Duration
	MaxAttempts  int
	LockFor      time.Duration
	MinReason    int
}

func DefaultPolicy() Policy {
	return Policy{ChallengeTTL: 5 * time.Minute, SessionTTL: 8 * time.Hour, IdleTTL: 30 * time.Minute, MaxAttempts: 5, LockFor: 15 * time.Minute, MinReason: 8}
}

type Service struct {
	Store  ports.Store
	Hasher ports.Hasher
	Box    ports.Box
	Audit  AuditLog
	Policy Policy
	Issuer string // shown in the authenticator app
	Tenant uint64
	Now    func() time.Time
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Service) tenant() uint64 {
	if s.Tenant == 0 {
		return 1
	}
	return s.Tenant
}

func newToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	token = hex.EncodeToString(b)
	return token, hashToken(token), nil
}

func hashToken(t string) string { s := sha256.Sum256([]byte(t)); return hex.EncodeToString(s[:]) }

func actorOf(p ports.Principal) string {
	return "admin:" + strconv.FormatUint(p.AdminID, 10) + ":" + p.Username
}

func detail(kv map[string]any) string { b, _ := json.Marshal(kv); return string(b) }

// Provisioning is what the person who owns an authenticator app needs once: the secret in text form and the otpauth URI to scan.
type Provisioning struct {
	Secret string
	URI    string
}

func (s *Service) provisioning(username string, secret []byte) Provisioning {
	return Provisioning{Secret: domain.TOTPSecretText(secret), URI: domain.OTPAuthURI(s.Issuer, username, secret)}
}

// Bootstrap creates the first administrator (role owner). It works exactly once: the decision lives in the database, in the same transaction as the admin row,
// so deleting a lock file (or any file) never re-opens initialisation. The TOTP secret is returned once and is the only way to complete the first login.
func (s *Service) Bootstrap(ctx context.Context, username, password string) (Provisioning, error) {
	if !domain.ValidUsername(username) {
		return Provisioning{}, domain.ErrInvalidInput
	}
	if err := domain.ValidatePassword(username, password); err != nil {
		return Provisioning{}, err
	}
	if done, err := s.Store.Initialized(ctx, s.tenant()); err != nil {
		return Provisioning{}, err
	} else if done {
		return Provisioning{}, domain.ErrAlreadyInitialized
	}
	hash, err := s.Hasher.Hash(password)
	if err != nil {
		return Provisioning{}, err
	}
	secret, err := domain.NewTOTPSecret()
	if err != nil {
		return Provisioning{}, err
	}
	enc, err := s.Box.Seal(secret)
	if err != nil {
		return Provisioning{}, err
	}
	id, err := s.Store.Bootstrap(ctx, s.tenant(), username, hash, enc, s.now())
	if err != nil {
		return Provisioning{}, err
	}
	if err := s.Audit.Append(ctx, audit.Entry{Actor: "cli:bootstrap", Action: "admin.bootstrap", Target: "admin:" + strconv.FormatUint(id, 10) + ":" + username,
		Detail: detail(map[string]any{"role": "owner"})}); err != nil {
		return Provisioning{}, fmt.Errorf("the first admin was created but the audit entry failed: %w", err)
	}
	return s.provisioning(username, secret), nil
}

type LoginResult struct {
	Kind  string // "mfa" (enter the current code) or "enroll" (first login: prove the authenticator app holds the secret from bootstrap)
	Token string // single-use, 5 minutes
}

func (s *Service) fail(ctx context.Context, action, target, ip string) {
	_ = s.Audit.Append(ctx, audit.Entry{Actor: "anonymous", Action: action, Target: target, Detail: detail(map[string]any{"ip": ip})})
}

// Login checks the password. It never returns a session: the second factor is always required, and an admin who has not enrolled one can only enrol.
func (s *Service) Login(ctx context.Context, username, password, ip string) (LoginResult, error) {
	a, ok, err := s.Store.AdminByUsername(ctx, s.tenant(), username)
	if err != nil {
		return LoginResult{}, err
	}
	now := s.now()
	if !ok || !a.Active {
		s.Hasher.DummyVerify(password)
		s.fail(ctx, "admin.login.failed", "unknown:"+clip(username), ip)
		return LoginResult{}, domain.ErrInvalidCredentials
	}
	if now.Before(a.LockedUntil) {
		return LoginResult{}, domain.ErrLocked
	}
	if !s.Hasher.Verify(password, a.PasswordHash) {
		locked, err := s.Store.RecordFailure(ctx, a.ID, s.Policy.MaxAttempts, s.Policy.LockFor, now)
		if err != nil {
			return LoginResult{}, err
		}
		act := "admin.login.failed"
		if locked {
			act = "admin.locked"
		}
		s.fail(ctx, act, "admin:"+strconv.FormatUint(a.ID, 10)+":"+a.Username, ip)
		return LoginResult{}, domain.ErrInvalidCredentials
	}
	kind := "mfa"
	if !a.Enrolled {
		kind = "enroll"
	}
	tok, hash, err := newToken()
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.Store.NewChallenge(ctx, a.ID, kind, hash, now.Add(s.Policy.ChallengeTTL)); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{Kind: kind, Token: tok}, nil
}

func clip(s string) string {
	if len(s) > 64 {
		return s[:64]
	}
	return s
}

type SessionResult struct {
	Token     string
	ExpiresAt time.Time
	Username  string
	Role      string
}

// Complete finishes a login (kind "mfa") or the first-time enrolment (kind "enroll") with a TOTP code and issues the session.
func (s *Service) Complete(ctx context.Context, kind, challenge, code, ip string) (SessionResult, error) {
	now := s.now()
	id, ok, err := s.Store.ConsumeChallenge(ctx, kind, hashToken(challenge), now) // single use: a wrong code needs a fresh password step
	if err != nil {
		return SessionResult{}, err
	}
	if !ok {
		return SessionResult{}, domain.ErrInvalidChallenge
	}
	a, found, err := s.Store.AdminByID(ctx, s.tenant(), id)
	if err != nil {
		return SessionResult{}, err
	}
	if !found || !a.Active {
		return SessionResult{}, domain.ErrInvalidChallenge
	}
	if now.Before(a.LockedUntil) {
		return SessionResult{}, domain.ErrLocked
	}
	secret, err := s.Box.Open(a.TOTPSecretEnc)
	if err != nil {
		return SessionResult{}, err
	}
	step, good := domain.VerifyTOTP(secret, code, now, a.TOTPLastStep)
	target := "admin:" + strconv.FormatUint(a.ID, 10) + ":" + a.Username
	if !good {
		locked, ferr := s.Store.RecordFailure(ctx, a.ID, s.Policy.MaxAttempts, s.Policy.LockFor, now)
		if ferr != nil {
			return SessionResult{}, ferr
		}
		act := "admin.mfa.failed"
		if locked {
			act = "admin.locked"
		}
		s.fail(ctx, act, target, ip)
		return SessionResult{}, domain.ErrInvalidCode
	}
	if err := s.Store.ConfirmStep(ctx, a.ID, step, kind == "enroll", now); err != nil {
		return SessionResult{}, err
	}
	if err := s.Store.ResetFailures(ctx, a.ID); err != nil {
		return SessionResult{}, err
	}
	tok, hash, err := newToken()
	if err != nil {
		return SessionResult{}, err
	}
	exp := now.Add(s.Policy.SessionTTL)
	if err := s.Store.NewSession(ctx, a.ID, hash, ip, now, exp); err != nil {
		return SessionResult{}, err
	}
	action := "admin.login"
	if kind == "enroll" {
		action = "admin.mfa.enrolled"
	}
	if err := s.Audit.Append(ctx, audit.Entry{Actor: "admin:" + strconv.FormatUint(a.ID, 10) + ":" + a.Username, Action: action, Target: target, Detail: detail(map[string]any{"ip": ip})}); err != nil {
		return SessionResult{}, err // no audit, no session
	}
	return SessionResult{Token: tok, ExpiresAt: exp, Username: a.Username, Role: domain.RoleNames[a.RoleID]}, nil
}

// Authenticate resolves a bearer token to a principal. Anything that is not a live, MFA-complete session of an active admin is domain.ErrUnauthenticated.
func (s *Service) Authenticate(ctx context.Context, token string) (ports.Principal, error) {
	if len(token) != 64 {
		return ports.Principal{}, domain.ErrUnauthenticated
	}
	p, ok, err := s.Store.Authenticate(ctx, hashToken(token), s.now(), s.Policy.IdleTTL)
	if err != nil {
		return p, err
	}
	if !ok {
		return p, domain.ErrUnauthenticated
	}
	return p, nil
}

func (s *Service) Logout(ctx context.Context, p ports.Principal) error {
	if err := s.Store.RevokeSession(ctx, p.SessionID, s.now()); err != nil {
		return err
	}
	return s.Audit.Append(ctx, audit.Entry{Actor: actorOf(p), Action: "admin.logout", Target: actorOf(p)})
}

func (s *Service) requireReason(reason string) (string, error) {
	r := strings.TrimSpace(reason)
	if len([]rune(r)) < s.Policy.MinReason {
		return "", domain.ErrReasonRequired
	}
	if len([]rune(r)) > 500 {
		return "", domain.ErrInvalidInput
	}
	return r, nil
}

// CreateAdmin (high risk). The new admin gets the initial password the creator chose and a fresh TOTP secret, returned once for hand-over; the new admin
// must enrol their authenticator on first login and cannot use any admin API before that.
func (s *Service) CreateAdmin(ctx context.Context, actor ports.Principal, username, role, password, reason string) (uint64, Provisioning, error) {
	reason, err := s.requireReason(reason)
	if err != nil {
		return 0, Provisioning{}, err
	}
	roleID, ok := domain.RoleByName(role)
	if !ok || !domain.ValidUsername(username) {
		return 0, Provisioning{}, domain.ErrInvalidInput
	}
	if err := domain.ValidatePassword(username, password); err != nil {
		return 0, Provisioning{}, err
	}
	hash, err := s.Hasher.Hash(password)
	if err != nil {
		return 0, Provisioning{}, err
	}
	secret, err := domain.NewTOTPSecret()
	if err != nil {
		return 0, Provisioning{}, err
	}
	enc, err := s.Box.Seal(secret)
	if err != nil {
		return 0, Provisioning{}, err
	}
	if err := s.Audit.Append(ctx, audit.Entry{Actor: actorOf(actor), Action: "admin.create", Target: username, Detail: detail(map[string]any{"role": role, "reason": reason})}); err != nil {
		return 0, Provisioning{}, err // the action does not happen without its audit record
	}
	id, err := s.Store.CreateAdmin(ctx, s.tenant(), username, hash, roleID, enc, actor.AdminID, s.now())
	if err != nil {
		_ = s.Audit.Append(ctx, audit.Entry{Actor: actorOf(actor), Action: "admin.create.failed", Target: username})
		return 0, Provisioning{}, err
	}
	return id, s.provisioning(username, secret), nil
}

// DisableAdmin (high risk) disables an admin and revokes every session of theirs. You cannot disable yourself, and the last active owner is protected.
func (s *Service) DisableAdmin(ctx context.Context, actor ports.Principal, id uint64, reason string) error {
	reason, err := s.requireReason(reason)
	if err != nil {
		return err
	}
	if id == actor.AdminID {
		return domain.ErrInvalidInput
	}
	target, found, err := s.Store.AdminByID(ctx, s.tenant(), id)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrNotFound
	}
	tname := "admin:" + strconv.FormatUint(id, 10) + ":" + target.Username
	if err := s.Audit.Append(ctx, audit.Entry{Actor: actorOf(actor), Action: "admin.disable", Target: tname, Detail: detail(map[string]any{"reason": reason})}); err != nil {
		return err
	}
	if err := s.Store.DisableAdmin(ctx, s.tenant(), id, s.now()); err != nil {
		_ = s.Audit.Append(ctx, audit.Entry{Actor: actorOf(actor), Action: "admin.disable.failed", Target: tname, Detail: detail(map[string]any{"error": clip(err.Error())})})
		return err
	}
	return nil
}

type AdminView struct {
	ID       uint64
	Username string
	Role     string
	Active   bool
	Enrolled bool
}

func (s *Service) ListAdmins(ctx context.Context) ([]AdminView, error) {
	as, err := s.Store.ListAdmins(ctx, s.tenant())
	if err != nil {
		return nil, err
	}
	out := make([]AdminView, len(as))
	for i, a := range as {
		out[i] = AdminView{a.ID, a.Username, domain.RoleNames[a.RoleID], a.Active, a.Enrolled}
	}
	return out, nil
}

// AuditRows lists audit rows newest first. It is read-only by construction: nothing in this module can write to or delete from the log except Append.
func (s *Service) AuditRows(ctx context.Context, f ports.AuditFilter) ([]ports.AuditRow, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 50
	}
	return s.Store.ListAudit(ctx, s.tenant(), f)
}

// VerifyAudit recomputes the whole hash chain.
func (s *Service) VerifyAudit(ctx context.Context) (audit.Head, error) { return s.Audit.Verify(ctx) }
