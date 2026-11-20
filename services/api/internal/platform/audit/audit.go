// Package audit is the append-only, hash-chained audit log (Story 1.4, ARCHITECTURE AD-15). Each row's hash covers the previous hash and the
// row's fields, so editing or deleting history breaks verification; the application's database account has INSERT/SELECT only on audit_log
// (infrastructure/mysql/app-grants.sql.tmpl) and the chain head is anchored daily to storage off the host, so even an attacker who controls the
// database cannot silently rewrite the past.
package audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"veyrogram/api/internal/platform/logx"
)

var genesis [32]byte // prev_hash of the first row

// Entry is one audit event. Detail must never contain secrets or message bodies (it is not redacted here).
type Entry struct {
	TenantID uint64
	Actor    string
	Action   string
	Target   string
	Detail   string
}

type Log struct {
	DB  *sql.DB
	Now func() time.Time
}

func (l *Log) now() time.Time {
	n := time.Now
	if l.Now != nil {
		n = l.Now
	}
	return n().UTC().Truncate(time.Millisecond)
}

func field(h interface{ Write([]byte) (int, error) }, s string) {
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(s)))
	h.Write(n[:])
	h.Write([]byte(s))
}

func compute(prev [32]byte, tenant uint64, actor, action, target, detail, trace string, at time.Time) [32]byte {
	h := sha256.New()
	h.Write(prev[:])
	var b [16]byte
	binary.BigEndian.PutUint64(b[:8], tenant)
	binary.BigEndian.PutUint64(b[8:], uint64(at.UnixMilli()))
	h.Write(b[:])
	for _, s := range []string{actor, action, target, detail, trace} {
		field(h, s)
	}
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// Append writes an entry and extends the chain. Appends are serialised by a row lock on audit_head. The trace id comes from ctx.
func (l *Log) Append(ctx context.Context, e Entry) error {
	if e.Actor == "" || e.Action == "" {
		return errors.New("audit: actor and action are required")
	}
	if e.TenantID == 0 {
		e.TenantID = 1
	}
	tx, err := l.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var prev []byte
	if err := tx.QueryRowContext(ctx, "SELECT hash FROM audit_head WHERE id=1 FOR UPDATE").Scan(&prev); err != nil {
		return err
	}
	var p [32]byte
	copy(p[:], prev)
	at, trace := l.now(), logx.Trace(ctx)
	h := compute(p, e.TenantID, e.Actor, e.Action, e.Target, e.Detail, trace, at)
	var detail any
	if e.Detail != "" {
		detail = e.Detail
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO audit_log (tenant_id, actor, action, target, detail, trace_id, created_at, prev_hash, hash) VALUES (?,?,?,?,?,?,?,?,?)`,
		e.TenantID, e.Actor, e.Action, e.Target, detail, trace, at, p[:], h[:])
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	if _, err := tx.ExecContext(ctx, "UPDATE audit_head SET last_id=?, hash=? WHERE id=1", id, h[:]); err != nil {
		return err
	}
	return tx.Commit()
}

// ErrTampered means the stored chain does not match its own hashes.
var ErrTampered = errors.New("audit: hash chain verification failed")

// Head describes the end of the verified chain.
type Head struct {
	ID   uint64
	Hash string // hex
	Rows int
}

// Verify recomputes the whole chain from audit_log (never trusting audit_head).
func (l *Log) Verify(ctx context.Context) (Head, error) {
	rows, err := l.DB.QueryContext(ctx, `SELECT id, tenant_id, actor, action, target, COALESCE(detail,''), trace_id, created_at, prev_hash, hash FROM audit_log ORDER BY id`)
	if err != nil {
		return Head{}, err
	}
	defer rows.Close()
	prev := genesis
	var head Head
	for rows.Next() {
		var id, tenant uint64
		var actor, action, target, detail, trace string
		var at time.Time
		var ph, hh []byte
		if err := rows.Scan(&id, &tenant, &actor, &action, &target, &detail, &trace, &at, &ph, &hh); err != nil {
			return head, err
		}
		if !bytes.Equal(ph, prev[:]) {
			return head, fmt.Errorf("%w: row %d does not link to the previous row (deleted or reordered rows)", ErrTampered, id)
		}
		want := compute(prev, tenant, actor, action, target, detail, trace, at.UTC())
		if !bytes.Equal(hh, want[:]) {
			return head, fmt.Errorf("%w: row %d was modified", ErrTampered, id)
		}
		prev = want
		head = Head{ID: id, Hash: hex.EncodeToString(want[:]), Rows: head.Rows + 1}
	}
	return head, rows.Err()
}

// Anchor is what gets stored off-host.
type Anchor struct {
	HeadID     uint64    `json:"head_id"`
	HeadHash   string    `json:"head_hash"`
	Rows       int       `json:"rows"`
	AnchoredAt time.Time `json:"anchored_at"`
}

// Sink stores anchors outside the database host (object storage with versioning/WORM, or a separate log host).
type Sink interface {
	Put(ctx context.Context, key string, body []byte) error
}

// AnchorHead verifies the chain and, only if it is intact, writes today's anchor to the sink. A broken chain is never anchored.
func (l *Log) AnchorHead(ctx context.Context, s Sink) (Anchor, error) {
	h, err := l.Verify(ctx)
	if err != nil {
		return Anchor{}, err
	}
	a := Anchor{HeadID: h.ID, HeadHash: h.Hash, Rows: h.Rows, AnchoredAt: l.now()}
	body, _ := json.Marshal(a)
	return a, s.Put(ctx, "audit-anchor/"+a.AnchoredAt.Format("2006-01-02")+".json", body)
}

// CheckAnchor proves the current log still contains the anchored history: the row at the anchored id must carry the anchored hash.
// This detects a full rewrite of the chain by someone who controls the database, which Verify alone cannot.
func (l *Log) CheckAnchor(ctx context.Context, a Anchor) error {
	if _, err := l.Verify(ctx); err != nil {
		return err
	}
	var hh []byte
	err := l.DB.QueryRowContext(ctx, "SELECT hash FROM audit_log WHERE id=?", a.HeadID).Scan(&hh)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: anchored row %d no longer exists", ErrTampered, a.HeadID)
	}
	if err != nil {
		return err
	}
	if hex.EncodeToString(hh) != a.HeadHash {
		return fmt.Errorf("%w: row %d differs from the off-host anchor", ErrTampered, a.HeadID)
	}
	return nil
}

// HTTPSink PUTs anchors to base+key (an S3-compatible bucket with versioning/object lock, or a log host). https is required unless the host is loopback.
type HTTPSink struct {
	Base   string
	Token  string
	Client *http.Client
}

// url validates the base and returns the object URL.
func (s HTTPSink) url(key string) (string, error) {
	u, err := url.Parse(strings.TrimRight(s.Base, "/") + "/" + key)
	if err != nil || u.Host == "" {
		return "", errors.New("audit: bad anchor URL")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1")) {
		return "", errors.New("audit: anchor URL must be https (http only for loopback tests)")
	}
	return u.String(), nil
}

func (s HTTPSink) Put(ctx context.Context, key string, body []byte) error {
	u, err := s.url(key)
	if err != nil {
		return err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("audit: anchor store answered HTTP %d", resp.StatusCode)
	}
	return nil
}

// Fetcher reads anchors back from the off-host store. found=false means the object does not exist (not an error).
type Fetcher interface {
	Get(ctx context.Context, key string) (body []byte, found bool, err error)
}

func (s HTTPSink) Get(ctx context.Context, key string) ([]byte, bool, error) {
	u, err := s.url(key)
	if err != nil {
		return nil, false, err
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if s.Token != "" {
		req.Header.Set("Authorization", "Bearer "+s.Token)
	}
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode/100 != 2 {
		return nil, false, fmt.Errorf("audit: anchor store answered HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return b, true, err
}

// AnchorReport is the outcome of comparing the current log with the last N days of off-host anchors.
type AnchorReport struct {
	Checked int
	Missing []string // dates with no anchor (the daily job did not run, or someone deleted it)
}

// CheckAnchors compares the log with the anchor of each of the last `days` days (today included). Any anchor that contradicts the log is
// ErrTampered. Missing days are reported, not failed, because the store may not have existed that long; callers decide how strict to be.
func (l *Log) CheckAnchors(ctx context.Context, f Fetcher, days int) (AnchorReport, error) {
	var r AnchorReport
	if _, err := l.Verify(ctx); err != nil {
		return r, err
	}
	for i := 0; i < days; i++ {
		day := l.now().AddDate(0, 0, -i).Format("2006-01-02")
		body, found, err := f.Get(ctx, "audit-anchor/"+day+".json")
		if err != nil {
			return r, fmt.Errorf("fetch anchor %s: %w", day, err)
		}
		if !found {
			r.Missing = append(r.Missing, day)
			continue
		}
		var a Anchor
		if err := json.Unmarshal(body, &a); err != nil || a.HeadHash == "" {
			return r, fmt.Errorf("%w: anchor %s is not valid", ErrTampered, day)
		}
		if err := l.CheckAnchor(ctx, a); err != nil {
			return r, fmt.Errorf("anchor %s: %w", day, err)
		}
		r.Checked++
	}
	return r, nil
}
