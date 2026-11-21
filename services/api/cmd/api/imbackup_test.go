package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"veyrogram/api/internal/platform/imgw"
	"veyrogram/api/internal/platform/testim"
)

// Story 1.22 AC2: the documented WuKongIM snapshot procedure (stop -> local copy -> start -> encrypt and ship) restores to a working IM with every message.
// Real IM process, real backup.sh / restore.sh, real encryption. Only the "stop" quiesce is exercised, because it is the only one verified; nothing here
// claims that copying a RUNNING IM's files is safe.
func TestIMDataDirSnapshotRestoresEveryMessage(t *testing.T) {
	bin := build(t)
	root := filepath.Join("..", "..", "..", "..")
	backup, restore := filepath.Join(root, "infrastructure", "backup", "backup.sh"), filepath.Join(root, "infrastructure", "backup", "restore.sh")
	if _, err := os.Stat(backup); err != nil {
		t.Skip("backup scripts not found")
	}
	keys := t.TempDir()
	if out, err := exec.Command(bin, "backup", "keygen", "-out-dir", filepath.Join(keys, "k")).CombinedOutput(); err != nil {
		t.Fatalf("keygen: %v\n%s", err, out)
	}

	a := testim.Start(t, "")
	c, err := imgw.New(a.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const n = 25
	for i := 0; i < n; i++ {
		if _, err := c.Send(ctx, "bk_u1", "bk_u2", 1, fmt.Sprintf("cmn-%03d", i), []byte(fmt.Sprintf(`{"type":1,"schema_version":3,"text":"snapshot-marker-%03d"}`, i))); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}
	before, err := c.ChannelMessageSync(ctx, "bk_u2", "bk_u1", 0, 100, true)
	if err != nil || len(before.Messages) != n {
		t.Fatalf("messages before backup: %d %v", len(before.Messages), err)
	}

	// The "service manager": the test owns the IM process, so the stop/start commands are recorded rather than executed. They must run in order, around the copy.
	work := t.TempDir()
	trace := filepath.Join(work, "trace")
	a.Stop() // stand-in for `systemctl stop veyrogram-wukongim` (the harness stops it with SIGKILL, i.e. the files are what a hard stop leaves behind)
	env := append(os.Environ(),
		"VG_BACKUP_ENV="+filepath.Join(work, "none.env"), "VG_BACKUP_DIR="+filepath.Join(work, "store"), "VG_BACKUP_BIN="+bin,
		"VG_BACKUP_PUBKEY="+filepath.Join(keys, "k", "public.pem"), "VG_BACKUP_REQUIRE_OFFHOST=0",
		"VG_IM_DATA_DIR="+a.DataDir(), "VG_IM_QUIESCE=stop",
		"VG_IM_STOP_CMD=echo stop >>"+trace, "VG_IM_START_CMD=echo start >>"+trace, "VG_BACKUP_METRICS="+filepath.Join(work, "b.prom"))
	cmd := exec.Command("bash", backup, "im")
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("backup.sh im: %v\n%s", err, out)
	}
	if b, _ := os.ReadFile(trace); strings.TrimSpace(string(b)) != "stop\nstart" {
		t.Fatalf("the IM must be stopped, then started again: %q", b)
	}
	arts, _ := filepath.Glob(filepath.Join(work, "store", "im-*.vgbk"))
	if len(arts) != 1 {
		t.Fatalf("artifact: %v", arts)
	}
	raw, _ := os.ReadFile(arts[0])
	if strings.Contains(string(raw), "snapshot-marker") || strings.Contains(string(raw), "bk_u1") {
		t.Fatal("the encrypted IM snapshot contains message plaintext")
	}
	if fi, _ := os.Stat(arts[0]); fi.Mode().Perm() != 0o600 {
		t.Errorf("artifact mode %v", fi.Mode().Perm())
	}
	man, _ := os.ReadFile(arts[0] + ".manifest")
	if !strings.Contains(string(man), "quiesce=stop") || !strings.Contains(string(man), "downtime_seconds=") {
		t.Errorf("manifest should record the quiesce method and the downtime: %s", man)
	}

	// Restore on "another machine" with the private key, into an empty directory; refuses a non-empty one and a wrong key.
	dest := filepath.Join(work, "restored")
	r := exec.Command("bash", restore, "im", "-f", arts[0], "-k", filepath.Join(keys, "k", "private.pem"), "-o", dest)
	r.Env = append(os.Environ(), "VG_BACKUP_BIN="+bin)
	if out, err := r.CombinedOutput(); err != nil {
		t.Fatalf("restore.sh im: %v\n%s", err, out)
	}
	if out, err := exec.Command("bash", restore, "im", "-f", arts[0], "-k", filepath.Join(keys, "k", "private.pem"), "-o", dest).CombinedOutput(); err == nil {
		t.Errorf("restoring into a non-empty directory must be refused:\n%s", out)
	}

	b := testim.StartOpts(t, testim.Options{DataDir: filepath.Join(dest, "data")})
	cb, err := imgw.New(b.APIURL)
	if err != nil {
		t.Fatal(err)
	}
	var after imgw.ChannelMessages
	deadline := time.Now().Add(20 * time.Second)
	for {
		after, err = cb.ChannelMessageSync(ctx, "bk_u2", "bk_u1", 0, 100, true)
		if err == nil && len(after.Messages) == n || time.Now().After(deadline) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if err != nil || len(after.Messages) != n {
		t.Fatalf("the restored IM serves %d of %d messages (%v)", len(after.Messages), n, err)
	}
	for i := range before.Messages {
		if before.Messages[i].MessageSeq != after.Messages[i].MessageSeq || string(before.Messages[i].Payload) != string(after.Messages[i].Payload) {
			t.Fatalf("message %d differs after restore:\n%+v\n%+v", i, before.Messages[i], after.Messages[i])
		}
	}
	// and it keeps working: a new message gets the next sequence number
	if _, err := cb.Send(ctx, "bk_u1", "bk_u2", 1, "cmn-after", []byte(`{"type":1,"schema_version":3,"text":"after restore"}`)); err != nil {
		t.Fatalf("send after restore: %v", err)
	}
	more, err := cb.ChannelMessageSync(ctx, "bk_u2", "bk_u1", 0, 100, true)
	if err != nil || len(more.Messages) != n+1 || more.Messages[n].MessageSeq != before.Messages[n-1].MessageSeq+1 {
		t.Fatalf("after restore the sequence must continue: %d %v", len(more.Messages), err)
	}
}
