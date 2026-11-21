package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"veyrogram/api/internal/platform/config"
)

// runProbe implements `veyrogram-api probe [-textfile FILE]`: is each part of the installation answering? It exits 1 when anything is down.
// Addresses come from the same VG_* variables the services read, so the probe cannot drift from them.
//
//	api     GET <internal listener>/readyz   200 (readiness includes MySQL)
//	worker  TCP connect to the worker's internal listener
//	im      GET <IM API>/healthz             200
//	canary  GET $VG_PROBE_CANARY_URL         200, only when that variable is set (optional: put it in the environment file while a canary runs)
//
// With -textfile it writes veyrogram_probe_up{target=...} and veyrogram_probe_timestamp_seconds atomically for node_exporter's textfile collector; a stopped
// probe shows as a timestamp that stops moving (alert VeyroGramProbeStale).
func runProbe(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	textfile := fs.String("textfile", "", "write Prometheus text metrics to this file (atomic rename)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	cfg, _ := config.Load(getenv) // configuration problems are the service's business; the probe only needs the addresses
	type target struct {
		name string
		up   func(context.Context) bool
	}
	ts := []target{
		{"api", httpOK("http://" + cfg.InternalAddr + "/readyz")},
		{"worker", tcpOK(cfg.WorkerInternalAddr)},
		{"im", httpOK(strings.TrimRight(cfg.IMAPIURL, "/") + "/healthz")},
	}
	if u := getenv("VG_PROBE_CANARY_URL"); u != "" {
		ts = append(ts, target{"canary", httpOK(u)})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	down := false
	var text strings.Builder
	text.WriteString("# TYPE veyrogram_probe_up gauge\n")
	for _, t := range ts {
		up := t.up(ctx)
		v, word := 0, "DOWN  "
		if up {
			v, word = 1, "ok    "
		} else {
			down = true
		}
		fmt.Fprintf(stdout, "%s %s\n", word, t.name)
		fmt.Fprintf(&text, "veyrogram_probe_up{target=%q} %d\n", t.name, v)
	}
	fmt.Fprintf(&text, "# TYPE veyrogram_probe_timestamp_seconds gauge\nveyrogram_probe_timestamp_seconds %d\n", time.Now().Unix())
	if *textfile != "" {
		if err := writeAtomic(*textfile, text.String()); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
	}
	if down {
		return 1
	}
	return 0
}

func httpOK(url string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return false
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return resp.StatusCode == http.StatusOK
	}
}

func tcpOK(addr string) func(context.Context) bool {
	return func(ctx context.Context) bool {
		var d net.Dialer
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		c, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return false
		}
		return c.Close() == nil
	}
}

func writeAtomic(path, content string) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".probe.*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
