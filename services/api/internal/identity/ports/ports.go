// Package ports defines what the identity module needs from the outside world.
package ports

import (
	"context"
	"time"

	"veyrogram/api/internal/identity/domain"
)

// Dim selects the counter dimension for login_event queries.
type Dim string

const (
	DimPhone  Dim = "phone"
	DimIP     Dim = "ip"
	DimDevice Dim = "device"
	DimAll    Dim = "all"
)

type Event struct {
	Tenant                                 uint64
	Kind, Result, PhoneHash, IP, DeviceKey string
	UserID                                 uint64
	TraceID                                string
	At                                     time.Time
}

type Code struct {
	ID       uint64
	Hash     string
	Attempts int
}

type LoginTx struct {
	Tenant         uint64
	CodeID         uint64
	PhoneHash      string
	Last4          string
	NewUID         string
	Device         domain.Device
	RefreshHash    string
	RefreshExpires time.Time
	Now            time.Time
}

type Session struct {
	UserID   uint64
	UID      string
	DeviceID uint64
	Class    domain.DeviceClass
	IsNew    bool
}

type RotateResult int

const (
	RotateOK RotateResult = iota
	RotateInvalid
	RotateReuse // a rotated token was replayed: the device has been revoked
)

type DeviceInfo struct {
	ID         uint64
	Class      domain.DeviceClass
	Name       string
	CreatedAt  time.Time
	LastSeenAt time.Time
}

type RevokedDevice struct {
	DeviceID uint64
	UID      string
	Class    domain.DeviceClass
}

// IMCreds are the facts needed to derive a device's IM token.
type IMCreds struct {
	Class  domain.DeviceClass
	Epoch  uint64
	Active bool // the device exists, belongs to the user and is not revoked
}

type IM interface {
	UpdateToken(ctx context.Context, uid, token string, deviceFlag, deviceLevel uint8) error
	DeviceQuit(ctx context.Context, uid string, deviceFlag uint8) error
}

// Notifier delivers an account notice to every device of uid, through the IM (stored, so offline devices get it on their next sync).
// clientMsgNo makes redelivery idempotent.
type Notifier interface {
	Notify(ctx context.Context, uid, clientMsgNo string, payload []byte) error
}

type Store interface {
	ListDevices(ctx context.Context, tenant uint64, userUID string) ([]DeviceInfo, error)
	// RevokeDevice revokes a device owned by userUID: marks it, kills its refresh tokens, bumps the IM epoch of its class and queues
	// the IM sync, all in one transaction. found=false when the device does not exist or belongs to someone else.
	RevokeDevice(ctx context.Context, tenant uint64, userUID string, deviceID uint64, now time.Time) (rd RevokedDevice, found bool, err error)
	// RevokeAllDevices does RevokeDevice for every live device of the user in one transaction (logout everywhere).
	RevokeAllDevices(ctx context.Context, tenant uint64, userUID string, now time.Time) ([]RevokedDevice, error)
	IMCreds(ctx context.Context, tenant uint64, userUID string, deviceID uint64) (IMCreds, error)
	ClassEpoch(ctx context.Context, tenant uint64, userUID string, class domain.DeviceClass) (uint64, error)
	CountEvents(ctx context.Context, kind, result string, d Dim, val string, since time.Time) (int, error)
	LastEvent(ctx context.Context, kind, result string, d Dim, val string) (time.Time, bool, error)
	AddEvent(ctx context.Context, e Event) error
	// NewCode supersedes unconsumed codes for the phone and stores the new hash.
	NewCode(ctx context.Context, tenant uint64, phoneHash, codeHash string, expires, now time.Time) error
	LatestCode(ctx context.Context, tenant uint64, phoneHash string, now time.Time) (Code, bool, error)
	BumpAttempt(ctx context.Context, codeID uint64) error
	// Login atomically consumes the code, creates or loads the user and device and stores the first refresh token.
	// It returns domain.ErrCodeConsumed when the code was already used and domain.ErrDisabled for disabled accounts.
	Login(ctx context.Context, in LoginTx) (Session, error)
	Rotate(ctx context.Context, tenant uint64, oldHash, newHash string, newExpires, now time.Time) (Session, RotateResult, error)
}

type SMSSender interface {
	Send(ctx context.Context, phone domain.Phone, message string) error
}
