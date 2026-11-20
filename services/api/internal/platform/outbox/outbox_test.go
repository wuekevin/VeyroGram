package outbox_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/logx"
	"veyrogram/api/internal/platform/migrate"
	"veyrogram/api/internal/platform/outbox"
	"veyrogram/api/internal/platform/testdb"
	"veyrogram/api/migrations"
)

func setup(t *testing.T) *sql.DB {
	t.Helper()
	db := testdb.New(t)
	ms, err := migrate.Load(migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (&migrate.Runner{DB: db, Migrations: ms}).Up(context.Background()); err != nil {
		t.Fatal(err)
	}
	return db
}

func enqueue(t *testing.T, db *sql.DB, ctx context.Context, topic, key, payload string) string {
	t.Helper()
	id, err := outbox.Enqueue(ctx, db, time.Now(), 1, topic, key, []byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func statusOf(t *testing.T, db *sql.DB, id string) (string, int) {
	t.Helper()
	var s string
	var a int
	if err := db.QueryRow("SELECT status, attempts FROM outbox WHERE event_id=?", id).Scan(&s, &a); err != nil {
		t.Fatal(err)
	}
	return s, a
}

func TestEnqueueIsAtomicWithTheCallersTransaction(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	tx, _ := db.Begin()
	if _, err := outbox.Enqueue(ctx, tx, time.Now(), 1, "t", "k", []byte("x")); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	n, _ := outbox.Pending(ctx, db)
	if n != 0 {
		t.Fatalf("a rolled-back transaction must leave no event, found %d", n)
	}
	tx, _ = db.Begin()
	_, _ = outbox.Enqueue(ctx, tx, time.Now(), 1, "t", "k", []byte("x"))
	tx.Commit()
	if n, _ := outbox.Pending(ctx, db); n != 1 {
		t.Fatalf("a committed transaction must leave exactly one event, found %d", n)
	}
	if _, err := outbox.Enqueue(ctx, db, time.Now(), 1, "", "k", nil); err == nil {
		t.Fatal("empty topic must be refused")
	}
}

func TestDeliveryCarriesPayloadAndTraceAndMarksDone(t *testing.T) {
	db := setup(t)
	ctx := logx.WithTrace(context.Background(), "deadbeefcafe0042")
	id := enqueue(t, db, ctx, "greet", "u1", "hello")
	var got outbox.Event
	w := &outbox.Worker{DB: db, Handlers: map[string]outbox.Handler{"greet": func(c context.Context, e outbox.Event) error {
		got = e
		if logx.Trace(c) != "deadbeefcafe0042" {
			t.Errorf("handler context must carry the producer's trace id, got %q", logx.Trace(c))
		}
		return nil
	}}}
	if n, err := w.RunOnce(context.Background()); err != nil || n != 1 {
		t.Fatalf("run: %d %v", n, err)
	}
	if got.ID != id || string(got.Payload) != "hello" || got.Topic != "greet" || got.Attempt != 1 || got.TraceID != "deadbeefcafe0042" {
		t.Fatalf("event wrong: %+v", got)
	}
	if s, _ := statusOf(t, db, id); s != "done" {
		t.Fatalf("status %s", s)
	}
	if n, _ := w.RunOnce(context.Background()); n != 0 {
		t.Fatal("a done event must not be claimed again")
	}
}

func TestTraceIDReachesTheIMThroughAPIOutboxWorkerImgw(t *testing.T) {
	db := setup(t)
	var rid, tp atomic.Value
	im := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rid.Store(r.Header.Get("X-Request-Id"))
		tp.Store(r.Header.Get("traceparent"))
		w.Write([]byte(`{"status":200}`))
	}))
	defer im.Close()
	c, _ := imgw.New(im.URL)
	// "API" side: the request trace id is in the context when the event is enqueued
	enqueue(t, db, logx.WithTrace(context.Background(), "feedfacecafe0001"), "im.kick", "u1", "u1")
	w := &outbox.Worker{DB: db, Handlers: map[string]outbox.Handler{"im.kick": func(ctx context.Context, e outbox.Event) error {
		return c.DeviceQuit(ctx, string(e.Payload), 255)
	}}}
	if _, err := w.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if rid.Load() != "feedfacecafe0001" || tp.Load() != "00-0000000000000000feedfacecafe0001-0000000000000001-01" {
		t.Fatalf("trace id did not travel API -> outbox -> worker -> imgw -> IM: %v %v", rid.Load(), tp.Load())
	}
}

func TestRedeliveredEventIsHandledOnce(t *testing.T) {
	db := setup(t)
	id := enqueue(t, db, context.Background(), "t", "k", "x")
	var runs atomic.Int32
	var results []string
	w := &outbox.Worker{DB: db, Handlers: map[string]outbox.Handler{"t": func(context.Context, outbox.Event) error { runs.Add(1); return nil }},
		OnResult: func(_, r string) { results = append(results, r) }}
	w.RunOnce(context.Background())
	// simulate a duplicate delivery (lost ack, replay tooling, lease race): the same event becomes claimable again
	if _, err := db.Exec("UPDATE outbox SET status='pending' WHERE event_id=?", id); err != nil {
		t.Fatal(err)
	}
	w.RunOnce(context.Background())
	if runs.Load() != 1 || len(results) != 2 || results[0] != "done" || results[1] != "duplicate" {
		t.Fatalf("handler must run once; runs=%d results=%v", runs.Load(), results)
	}
	if s, _ := statusOf(t, db, id); s != "done" {
		t.Fatalf("status %s", s)
	}
	// the same event id can't be enqueued twice either
	if _, err := db.Exec(`INSERT INTO outbox (event_id, topic, ordering_key, payload, available_at, created_at) VALUES (?, 't','k','x',NOW(),NOW())`, id); err == nil {
		t.Fatal("event_id must be unique")
	}
}

func TestFailuresBackOffThenSucceed(t *testing.T) {
	db := setup(t)
	id := enqueue(t, db, context.Background(), "t", "k", "x")
	now := time.Now().UTC()
	var calls int
	w := &outbox.Worker{DB: db, Now: func() time.Time { return now }, Backoff: func(n int) time.Duration { return time.Duration(n) * time.Minute },
		Handlers: map[string]outbox.Handler{"t": func(context.Context, outbox.Event) error {
			calls++
			if calls < 3 {
				return errors.New("temporary")
			}
			return nil
		}}}
	ctx := context.Background()
	w.RunOnce(ctx)
	if s, a := statusOf(t, db, id); s != "pending" || a != 1 || calls != 1 {
		t.Fatalf("after failure 1: %s attempts=%d calls=%d", s, a, calls)
	}
	if n, _ := w.RunOnce(ctx); n != 0 {
		t.Fatal("must not retry before the backoff elapsed")
	}
	now = now.Add(61 * time.Second)
	w.RunOnce(ctx)
	if s, a := statusOf(t, db, id); s != "pending" || a != 2 {
		t.Fatalf("after failure 2: %s attempts=%d", s, a)
	}
	now = now.Add(121 * time.Second)
	w.RunOnce(ctx)
	if s, a := statusOf(t, db, id); s != "done" || a != 3 || calls != 3 {
		t.Fatalf("after success: %s attempts=%d calls=%d", s, a, calls)
	}
}

func TestExhaustedEventsGoToDeadLetterAndCanBeReplayed(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	id := enqueue(t, db, ctx, "t", "k", "x")
	now := time.Now().UTC()
	var fail atomic.Bool
	fail.Store(true)
	var results []string
	w := &outbox.Worker{DB: db, MaxAttempts: 3, Now: func() time.Time { return now }, Backoff: func(int) time.Duration { return time.Second },
		OnResult: func(_, r string) { results = append(results, r) },
		Handlers: map[string]outbox.Handler{"t": func(context.Context, outbox.Event) error {
			if fail.Load() {
				return errors.New("boom")
			}
			return nil
		}}}
	for i := 0; i < 3; i++ {
		w.RunOnce(ctx)
		now = now.Add(time.Minute)
	}
	if s, a := statusOf(t, db, id); s != "dead" || a != 3 {
		t.Fatalf("expected dead after 3 attempts, got %s/%d", s, a)
	}
	if fmt.Sprint(results) != "[retry retry dead]" {
		t.Fatalf("results %v", results)
	}
	var dlID uint64
	var topic, lastErr string
	if err := db.QueryRow("SELECT id, topic, last_error FROM dead_letter WHERE event_id=?", id).Scan(&dlID, &topic, &lastErr); err != nil || topic != "t" || lastErr != "boom" {
		t.Fatalf("dead letter row: %v %q %q", err, topic, lastErr)
	}
	if n, _ := w.RunOnce(ctx); n != 0 {
		t.Fatal("dead events must not be claimed")
	}
	fail.Store(false)
	if err := outbox.Replay(ctx, db, now, dlID); err != nil {
		t.Fatal(err)
	}
	if err := outbox.Replay(ctx, db, now, dlID); err == nil {
		t.Fatal("replaying the same dead letter twice must fail")
	}
	if err := outbox.Replay(ctx, db, now, 99999); err == nil {
		t.Fatal("unknown dead letter must fail")
	}
	w.RunOnce(ctx)
	if s, a := statusOf(t, db, id); s != "done" || a != 1 {
		t.Fatalf("replayed event must be delivered afresh: %s/%d", s, a)
	}
}

func TestPanicsAndMissingHandlersAreFailuresNotCrashes(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	a := enqueue(t, db, ctx, "panics", "k1", "x")
	b := enqueue(t, db, ctx, "nobody", "k2", "x")
	w := &outbox.Worker{DB: db, Handlers: map[string]outbox.Handler{"panics": func(context.Context, outbox.Event) error { panic("kaboom") }}}
	if n, err := w.RunOnce(ctx); err != nil || n != 2 {
		t.Fatalf("worker must survive: %d %v", n, err)
	}
	for _, id := range []string{a, b} {
		if s, at := statusOf(t, db, id); s != "pending" || at != 1 {
			t.Fatalf("%s: %s/%d", id, s, at)
		}
	}
	var msg string
	db.QueryRow("SELECT last_error FROM outbox WHERE event_id=?", a).Scan(&msg)
	if msg != "handler panic: kaboom" {
		t.Fatalf("error recorded: %q", msg)
	}
}

func TestCrashedWorkersLeaseExpiresAndAnotherWorkerTakesOver(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	id := enqueue(t, db, ctx, "t", "k", "x")
	now := time.Now().UTC()
	stuck := &outbox.Worker{DB: db, ID: "w1", Now: func() time.Time { return now }, Lease: time.Minute, Handlers: map[string]outbox.Handler{"t": func(context.Context, outbox.Event) error { return nil }}}
	// simulate a worker that claimed the event and died: row is processing with a lease in the future
	db.Exec("UPDATE outbox SET status='processing', locked_by='dead-worker', locked_until=?, attempts=1 WHERE event_id=?", now.Add(time.Minute), id)
	if n, _ := stuck.RunOnce(ctx); n != 0 {
		t.Fatal("an unexpired lease must be respected")
	}
	now = now.Add(2 * time.Minute)
	if n, _ := stuck.RunOnce(ctx); n != 1 {
		t.Fatal("an expired lease must be reclaimed")
	}
	if s, a := statusOf(t, db, id); s != "done" || a != 2 {
		t.Fatalf("%s/%d", s, a)
	}
}

func TestManyWorkersNeverProcessAnEventTwice(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	const total = 300
	for i := 0; i < total; i++ {
		enqueue(t, db, ctx, "t", fmt.Sprintf("key-%d", i), fmt.Sprint(i))
	}
	var mu sync.Mutex
	seen := map[string]int{}
	h := func(_ context.Context, e outbox.Event) error {
		mu.Lock()
		seen[e.ID]++
		mu.Unlock()
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := &outbox.Worker{DB: db, ID: fmt.Sprintf("w%d", i), Batch: 8, Handlers: map[string]outbox.Handler{"t": h}}
			for {
				n, err := w.RunOnce(ctx)
				if err != nil {
					t.Error(err)
					return
				}
				if n == 0 {
					return
				}
			}
		}(i)
	}
	wg.Wait()
	if len(seen) != total {
		t.Fatalf("every event must be delivered: got %d of %d", len(seen), total)
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("event %s handled %d times by concurrent workers", id, n)
		}
	}
}

func TestEventsWithTheSameKeyAreDeliveredInOrderAndNeverConcurrently(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	const keys, per = 5, 20
	for i := 0; i < per; i++ {
		for k := 0; k < keys; k++ {
			enqueue(t, db, ctx, "t", fmt.Sprintf("agg-%d", k), fmt.Sprintf("%d:%d", k, i))
		}
	}
	var mu sync.Mutex
	last := map[int]int{}
	busy := map[int]bool{}
	var violations atomic.Int32
	h := func(_ context.Context, e outbox.Event) error {
		var k, i int
		fmt.Sscanf(string(e.Payload), "%d:%d", &k, &i)
		mu.Lock()
		if busy[k] {
			violations.Add(1)
		}
		busy[k] = true
		if prev, ok := last[k]; ok && i != prev+1 {
			violations.Add(1)
		}
		last[k] = i
		mu.Unlock()
		time.Sleep(time.Millisecond)
		mu.Lock()
		busy[k] = false
		mu.Unlock()
		return nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			w := &outbox.Worker{DB: db, ID: fmt.Sprintf("w%d", i), Handlers: map[string]outbox.Handler{"t": h}}
			for idle := 0; idle < 20; {
				if n, err := w.RunOnce(ctx); err != nil {
					t.Error(err)
					return
				} else if n == 0 {
					idle++
					time.Sleep(5 * time.Millisecond)
				}
			}
		}(i)
	}
	wg.Wait()
	for k := 0; k < keys; k++ {
		if last[k] != per-1 {
			t.Fatalf("key %d stopped at %d", k, last[k])
		}
	}
	if violations.Load() != 0 {
		t.Fatalf("%d ordering/concurrency violations within an ordering key", violations.Load())
	}
}

func TestAFailingEventBlocksLaterEventsOfTheSameKeyOnly(t *testing.T) {
	db := setup(t)
	ctx := context.Background()
	a1 := enqueue(t, db, ctx, "t", "A", "fail")
	a2 := enqueue(t, db, ctx, "t", "A", "ok")
	b1 := enqueue(t, db, ctx, "t", "B", "ok")
	w := &outbox.Worker{DB: db, Backoff: func(int) time.Duration { return time.Hour }, Handlers: map[string]outbox.Handler{"t": func(_ context.Context, e outbox.Event) error {
		if string(e.Payload) == "fail" {
			return errors.New("x")
		}
		return nil
	}}}
	w.RunOnce(ctx)
	w.RunOnce(ctx)
	sa1, _ := statusOf(t, db, a1)
	sa2, _ := statusOf(t, db, a2)
	sb1, _ := statusOf(t, db, b1)
	if sa1 != "pending" || sa2 != "pending" || sb1 != "done" {
		t.Fatalf("order within a key must hold while an earlier event is waiting to retry: a1=%s a2=%s b1=%s", sa1, sa2, sb1)
	}
}
