// Command anchor-store runs the write-once audit-anchor store (internal/anchorstore) on a separate host.
//
//	ANCHOR_STORE_DIR=/var/lib/anchors ANCHOR_STORE_WRITE_TOKEN=... ANCHOR_STORE_READ_TOKEN=... ANCHOR_STORE_ADDR=127.0.0.1:9200 anchor-store
//
// Put it behind an HTTPS reverse proxy; both tokens must be >= 32 chars and differ.
package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"veyrogram/api/internal/anchorstore"
)

func main() {
	dir, addr := os.Getenv("ANCHOR_STORE_DIR"), os.Getenv("ANCHOR_STORE_ADDR")
	w, r := os.Getenv("ANCHOR_STORE_WRITE_TOKEN"), os.Getenv("ANCHOR_STORE_READ_TOKEN")
	if addr == "" {
		addr = "127.0.0.1:9200"
	}
	if dir == "" || len(w) < 32 || len(r) < 32 || w == r {
		fmt.Fprintln(os.Stderr, "ANCHOR_STORE_DIR is required; ANCHOR_STORE_WRITE_TOKEN and ANCHOR_STORE_READ_TOKEN must be >= 32 chars and different")
		os.Exit(78)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	srv := &http.Server{Addr: addr, Handler: anchorstore.Handler(anchorstore.Config{Dir: dir, WriteToken: w, ReadToken: r}), ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintln(os.Stderr, "anchor-store listening on", addr)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
