package archtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const mod = "veyrogram/api"

func write(t *testing.T, root, rel, src string) {
	t.Helper()
	p := filepath.Join(root, "internal", rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func file(pkg string, imports ...string) string {
	var b strings.Builder
	b.WriteString("package " + pkg + "\n")
	for _, i := range imports {
		b.WriteString("import _ \"" + i + "\"\n")
	}
	return b.String()
}

// The real tree must always be clean — this is the CI gate for AD-1.
func TestRepositoryTreeFollowsDependencyDirection(t *testing.T) {
	vs, err := Check("../..", mod)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		t.Error(v)
	}
}

func TestRulesFireOnViolations(t *testing.T) {
	cases := []struct {
		name, rel, src, want string
	}{
		{"domain imports adapters", "chat/domain/m.go", file("domain", mod+"/internal/chat/adapters/mysql"), "domain must not import adapters"},
		{"domain imports app", "chat/domain/m.go", file("domain", mod+"/internal/chat/app"), "domain must not import app"},
		{"domain imports a database driver", "chat/domain/m.go", file("domain", "github.com/go-sql-driver/mysql"), "no drivers/frameworks"},
		{"app imports adapters", "chat/app/s.go", file("app", mod+"/internal/chat/adapters/mysql"), "app must not import adapters"},
		{"ports import app", "chat/ports/p.go", file("ports", mod+"/internal/chat/app"), "ports must not import app"},
		{"cross-module via domain", "chat/app/s.go", file("app", mod+"/internal/identity/domain"), "only through its ports"},
		{"cross-module via adapters", "chat/adapters/a.go", file("adapters", mod+"/internal/identity/adapters/mysql"), "only through its ports"},
		{"platform imports a module", "platform/x/x.go", file("x", mod+"/internal/chat/ports"), "must not import any module"},
		{"internal imports cmd", "chat/app/s.go", file("app", mod+"/cmd/api"), "must not import cmd"},
		{"http imports adapters", "chat/http/h.go", file("http", mod+"/internal/chat/adapters/mysql"), "http must not import adapters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, root, tc.rel, tc.src)
			vs, err := Check(root, mod)
			if err != nil {
				t.Fatal(err)
			}
			if len(vs) != 1 || !strings.Contains(vs[0].Rule, tc.want) {
				t.Fatalf("want one violation containing %q, got %v", tc.want, vs)
			}
		})
	}
}

func TestAllowedImportsPass(t *testing.T) {
	root := t.TempDir()
	write(t, root, "chat/domain/m.go", file("domain", "errors", mod+"/internal/platform/httpx"))
	write(t, root, "chat/ports/p.go", file("ports", mod+"/internal/chat/domain"))
	write(t, root, "chat/app/s.go", file("app", mod+"/internal/chat/domain", mod+"/internal/chat/ports", mod+"/internal/identity/ports"))
	write(t, root, "chat/adapters/mysql/a.go", file("mysql", mod+"/internal/chat/ports", "github.com/go-sql-driver/mysql"))
	write(t, root, "chat/http/h.go", file("http", mod+"/internal/chat/app", mod+"/internal/platform/health"))
	write(t, root, "platform/health/h.go", file("health", "net/http"))
	if vs, err := Check(root, mod); err != nil || len(vs) != 0 {
		t.Fatalf("expected no violations, got %v err=%v", vs, err)
	}
}

func TestMissingInternalDirIsNotAnError(t *testing.T) {
	if vs, err := Check(t.TempDir(), mod); err != nil || len(vs) != 0 {
		t.Fatalf("got %v %v", vs, err)
	}
}
