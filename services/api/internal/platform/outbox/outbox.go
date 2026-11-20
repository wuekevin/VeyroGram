// Package outbox implements the transactional outbox and its worker (ARCHITECTURE AD-7). Producers enqueue inside the same MySQL transaction as
// their state change; workers claim rows with FOR UPDATE SKIP LOCKED, deliver at-least-once, keep order per ordering key, de-duplicate per
// consumer by event id, back off on failure, and move exhausted events to dead_letter, from where an operator can replay them.
// Handlers MUST still be idempotent by event id: a crash after a side effect but before the commit redelivers the event.
package outbox

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"veyrogram/api/internal/platform/logx"
)

// Event is what handlers receive.
type Event struct {
	ID          string // event_id (32 hex)
	TenantID    uint64
	Topic       string
	OrderingKey string
	Payload     []byte
	TraceID     string
	Attempt     int // 1 on first delivery
}

// Execer is satisfied by *sql.Tx and *sql.DB.
type Execer interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Enqueue stores an event. Pass the caller's *sql.Tx so the event commits atomically with the state change. The trace id is taken from ctx.
func Enqueue(ctx context.Context, x Execer, now time.Time, tenant uint64, topic, orderingKey string, payload []byte) (string, error) {
	if topic == "" || orderingKey == "" {
		return "", errors.New("outbox: topic and ordering key are required")
	}
	if tenant == 0 {
		tenant = 1
	}
	id := newID()
	at := now.UTC().Truncate(time.Millisecond)
	_, err := x.ExecContext(ctx, `INSERT INTO outbox (event_id, tenant_id, topic, ordering_key, payload, trace_id, status, available_at, created_at)
		VALUES (?,?,?,?,?,?, 'pending', ?, ?)`, id, tenant, topic, orderingKey, payload, logx.Trace(ctx), at, at)
	return id, err
}

// Handler processes one event; returning an error schedules a retry.
type Handler func(ctx context.Context, e Event) error

// Worker claims and delivers events.
type Worker struct {
	DB          *sql.DB
	ID          string                          // worker identity (default random)
	Handlers    map[string]Handler              // keyed by topic; the topic is also the consumer name for dedupe
	Batch       int                             // default 16
	Lease       time.Duration                   // default 1m: how long a claimed row stays invisible
	MaxAttempts int                             // default 5
	Backoff     func(attempt int) time.Duration // default 2^attempt seconds, capped at 5m
	Now         func() time.Time
	OnResult    func(topic, result string) // "done" | "retry" | "dead" | "duplicate"
	Log         func(msg string, kv ...any)
}

func (w *Worker) now() time.Time {
	if w.Now != nil {
		return w.Now().UTC().Truncate(time.Millisecond)
	}
	return time.Now().UTC().Truncate(time.Millisecond)
}
func (w *Worker) batch() int {
	if w.Batch > 0 {
		return w.Batch
	}
	return 16
}
func (w *Worker) lease() time.Duration {
	if w.Lease > 0 {
		return w.Lease
	}
	return time.Minute
}
func (w *Worker) maxAttempts() int {
	if w.MaxAttempts > 0 {
		return w.MaxAttempts
	}
	return 5
}
func (w *Worker) backoff(n int) time.Duration {
	if w.Backoff != nil {
		return w.Backoff(n)
	}
	d := time.Second << min(n, 8)
	return min(d, 5*time.Minute)
}
func (w *Worker) result(topic, r string) {
	if w.OnResult != nil {
		w.OnResult(topic, r)
	}
}
func (w *Worker) logf(msg string, kv ...any) {
	if w.Log != nil {
		w.Log(msg, kv...)
	}
}

type claimed struct {
	id    uint64
	event Event
}

// claim atomically reserves up to Batch events. Only the oldest unfinished event of each ordering key is eligible, so events with the same key
// are delivered strictly one after another, across all workers. Rows whose lease expired (crashed worker) are eligible again.
func (w *Worker) claim(ctx context.Context) ([]claimed, error) {
	if w.ID == "" {
		w.ID = newID()[:16]
	}
	now := w.now()
	tx, err := w.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT id, event_id, tenant_id, topic, ordering_key, payload, trace_id, attempts FROM outbox o
		WHERE ((o.status='pending' AND o.available_at <= ?) OR (o.status='processing' AND o.locked_until <= ?))
		AND NOT EXISTS (SELECT 1 FROM outbox p WHERE p.ordering_key=o.ordering_key AND p.id < o.id AND p.status IN ('pending','processing'))
		ORDER BY o.id LIMIT ? FOR UPDATE SKIP LOCKED`, now, now, w.batch())
	if err != nil {
		return nil, err
	}
	var out []claimed
	for rows.Next() {
		var c claimed
		var attempts int
		if err := rows.Scan(&c.id, &c.event.ID, &c.event.TenantID, &c.event.Topic, &c.event.OrderingKey, &c.event.Payload, &c.event.TraceID, &attempts); err != nil {
			rows.Close()
			return nil, err
		}
		c.event.Attempt = attempts + 1
		out = append(out, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, c := range out {
		if _, err := tx.ExecContext(ctx, `UPDATE outbox SET status='processing', locked_by=?, locked_until=?, attempts=attempts+1 WHERE id=?`, w.ID, now.Add(w.lease()), c.id); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit()
}

// RunOnce claims one batch and delivers it; it returns how many events were handled (any outcome).
func (w *Worker) RunOnce(ctx context.Context) (int, error) {
	cl, err := w.claim(ctx)
	if err != nil {
		return 0, err
	}
	for _, c := range cl {
		if err := w.deliver(ctx, c); err != nil {
			return 0, err
		}
	}
	return len(cl), nil
}

func (w *Worker) deliver(ctx context.Context, c claimed) error {
	e := c.event
	ctx = logx.WithTrace(ctx, e.TraceID)
	var dup int
	if err := w.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox_consumed WHERE consumer=? AND event_id=?", e.Topic, e.ID).Scan(&dup); err != nil {
		return err
	}
	if dup > 0 { // already handled (redelivery after a lost commit, or a manual replay): acknowledge without running the handler
		w.result(e.Topic, "duplicate")
		return w.finish(ctx, c, nil, false)
	}
	h := w.Handlers[e.Topic]
	var herr error
	if h == nil {
		herr = fmt.Errorf("no handler registered for topic %q", e.Topic)
	} else {
		herr = safeCall(ctx, h, e)
	}
	return w.finish(ctx, c, herr, true)
}

func safeCall(ctx context.Context, h Handler, e Event) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("handler panic: %v", r)
		}
	}()
	return h(ctx, e)
}

func (w *Worker) finish(ctx context.Context, c claimed, herr error, record bool) error {
	now := w.now()
	// completion must survive a cancelled ctx: the side effect already happened
	ctx = context.WithoutCancel(ctx)
	e := c.event
	if herr == nil {
		tx, err := w.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if record {
			if _, err := tx.ExecContext(ctx, "INSERT IGNORE INTO outbox_consumed (consumer, event_id, consumed_at) VALUES (?,?,?)", e.Topic, e.ID, now); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, "UPDATE outbox SET status='done', locked_until=NULL, last_error='' WHERE id=?", c.id); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		if record {
			w.result(e.Topic, "done")
		}
		return nil
	}
	msg := herr.Error()
	if len(msg) > 480 {
		msg = msg[:480]
	}
	if e.Attempt >= w.maxAttempts() {
		tx, err := w.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, "UPDATE outbox SET status='dead', locked_until=NULL, last_error=? WHERE id=?", msg, c.id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO dead_letter (outbox_id, event_id, topic, attempts, last_error, failed_at) VALUES (?,?,?,?,?,?)", c.id, e.ID, e.Topic, e.Attempt, msg, now); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		w.result(e.Topic, "dead")
		w.logf("outbox event moved to dead letter", "topic", e.Topic, "event_id", e.ID, "attempts", e.Attempt)
		return nil
	}
	_, err := w.DB.ExecContext(ctx, "UPDATE outbox SET status='pending', locked_until=NULL, available_at=?, last_error=? WHERE id=?", now.Add(w.backoff(e.Attempt)), msg, c.id)
	if err == nil {
		w.result(e.Topic, "retry")
	}
	return err
}

// Run loops until ctx is cancelled, sleeping `idle` when there was nothing to do.
func (w *Worker) Run(ctx context.Context, idle time.Duration) error {
	if idle <= 0 {
		idle = time.Second
	}
	for ctx.Err() == nil {
		n, err := w.RunOnce(ctx)
		if err != nil && ctx.Err() == nil {
			w.logf("outbox worker error", "err", err.Error())
			n = 0
		}
		if n == 0 {
			select {
			case <-ctx.Done():
			case <-time.After(idle):
			}
		}
	}
	return nil
}

// Replay puts a dead-lettered event back in the queue (attempts reset) and marks the dead letter replayed. It also forgets the consumer
// dedupe record so the handler really runs again.
func Replay(ctx context.Context, db *sql.DB, now time.Time, deadLetterID uint64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var outboxID uint64
	var topic, eventID string
	var replayed sql.NullTime
	err = tx.QueryRowContext(ctx, "SELECT outbox_id, topic, event_id, replayed_at FROM dead_letter WHERE id=? FOR UPDATE", deadLetterID).Scan(&outboxID, &topic, &eventID, &replayed)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("outbox: dead letter %d not found", deadLetterID)
	}
	if err != nil {
		return err
	}
	if replayed.Valid {
		return fmt.Errorf("outbox: dead letter %d was already replayed", deadLetterID)
	}
	at := now.UTC().Truncate(time.Millisecond)
	if _, err := tx.ExecContext(ctx, "UPDATE outbox SET status='pending', attempts=0, available_at=?, locked_until=NULL, last_error='' WHERE id=? AND status='dead'", at, outboxID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM outbox_consumed WHERE consumer=? AND event_id=?", topic, eventID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE dead_letter SET replayed_at=? WHERE id=?", at, deadLetterID); err != nil {
		return err
	}
	return tx.Commit()
}

// Pending counts events not yet finished (for the vg_outbox_pending gauge).
func Pending(ctx context.Context, db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM outbox WHERE status IN ('pending','processing')").Scan(&n)
	return n, err
}
