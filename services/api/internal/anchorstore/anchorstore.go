// Package anchorstore is a tiny write-once HTTP store for audit anchors. Run it on a host that the application/database host cannot administer
// (a different machine or provider): even someone who fully controls the main database and the veyrogram account can then not rewrite history,
// because an existing anchor can never be replaced or deleted through this API.
//
//	PUT /audit-anchor/YYYY-MM-DD.json   write-once; the same bytes again is 200 (idempotent), different bytes is 409
//	GET /audit-anchor/YYYY-MM-DD.json   read
//
// PUT needs the write token, GET the read token (separate secrets). TLS is terminated by the reverse proxy in front.
package anchorstore

import (
	"bytes"
	"crypto/subtle"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maxBody = 4 << 10

var keyRE = regexp.MustCompile(`^/audit-anchor/(\d{4}-\d{2}-\d{2})\.json$`)

type Config struct {
	Dir        string // anchors are stored as files here, mode 0444
	WriteToken string
	ReadToken  string
}

func bearer(r *http.Request, want string) bool {
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return want != "" && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

// Handler serves the store. An empty token disables the corresponding method (never open).
func Handler(c Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := keyRE.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.NotFound(w, r)
			return
		}
		file := filepath.Join(c.Dir, m[1]+".json")
		switch r.Method {
		case http.MethodGet:
			if !bearer(r, c.ReadToken) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			b, err := os.ReadFile(file)
			if errors.Is(err, os.ErrNotExist) {
				http.NotFound(w, r)
				return
			}
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(b)
		case http.MethodPut:
			if !bearer(r, c.WriteToken) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
			if err != nil || len(body) == 0 || len(body) > maxBody {
				http.Error(w, "bad body", http.StatusRequestEntityTooLarge)
				return
			}
			// O_EXCL makes "first writer wins" atomic; there is deliberately no overwrite or delete path.
			f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o444)
			if errors.Is(err, os.ErrExist) {
				old, rerr := os.ReadFile(file)
				if rerr == nil && bytes.Equal(old, body) {
					w.WriteHeader(http.StatusOK)
					return
				}
				http.Error(w, "anchor already exists with different content", http.StatusConflict)
				return
			}
			if err != nil {
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			_, werr := f.Write(body)
			if cerr := f.Close(); werr != nil || cerr != nil {
				_ = os.Remove(file)
				http.Error(w, "error", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusCreated)
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed) // DELETE/POST/PATCH never exist
		}
	})
}
