package preflight

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"testing"
)

// fakeHost is a healthy production host; each test breaks one thing.
type fakeHost struct {
	goos      string
	uid       int
	users     map[string]bool
	paths     map[string]PathInfo
	busy      map[string]bool
	cmds      map[string]string
	disk      uint64
	mysqlInfo MySQLInfo
	mysqlErr  error
	redisInfo RedisInfo
	redisErr  error
}

func healthy() *fakeHost {
	return &fakeHost{
		goos:  "linux",
		uid:   0,
		users: map[string]bool{"veyrogram": true},
		paths: map[string]PathInfo{
			"/run/systemd/system":    {Exists: true},
			"/etc/veyrogram/api.env": {Exists: true, Mode: 0o640, UID: 0},
		},
		busy: map[string]bool{},
		cmds: map[string]string{"nginx -v": "nginx version: nginx/1.24.0", "timedatectl show -p NTPSynchronized --value": "yes\n"},
		disk: 100 * gib,
		mysqlInfo: MySQLInfo{Version: "8.0.36", Comment: "MySQL Community Server - GPL", SQLMode: "ONLY_FULL_GROUP_BY,STRICT_TRANS_TABLES", Charset: "utf8mb4",
			MaxConns: 500, LogBin: true, BinlogFormat: "ROW"},
		redisInfo: RedisInfo{Version: "7.2.4", Policy: "noeviction", AOF: true},
	}
}

func (h *fakeHost) GOOS() string                    { return h.goos }
func (h *fakeHost) UID() int                        { return h.uid }
func (h *fakeHost) UserExists(n string) bool        { return h.users[n] }
func (h *fakeHost) Stat(p string) PathInfo          { return h.paths[p] }
func (h *fakeHost) DiskFree(string) (uint64, error) { return h.disk, nil }
func (h *fakeHost) CanListen(a string) error {
	if h.busy[a] {
		return errors.New("address already in use")
	}
	return nil
}
func (h *fakeHost) Exec(_ context.Context, name string, args ...string) (string, error) {
	name = name[strings.LastIndex(name, "/")+1:]
	out, ok := h.cmds[strings.Join(append([]string{name}, args...), " ")]
	if !ok {
		return "", errors.New("not found")
	}
	return out, nil
}
func (h *fakeHost) LookPath(name string) (string, bool) {
	for k := range h.cmds {
		if strings.HasPrefix(k, name+" ") {
			return "/usr/sbin/" + name, true
		}
	}
	return "", false
}
func (h *fakeHost) MySQL(context.Context, string) (MySQLInfo, error) { return h.mysqlInfo, h.mysqlErr }
func (h *fakeHost) Redis(context.Context, string, string) (RedisInfo, error) {
	return h.redisInfo, h.redisErr
}

func prodOpts() Options {
	return Options{Strict: true, Install: true, Fresh: true, ServiceUser: "veyrogram", EnvFile: "/etc/veyrogram/api.env",
		DiskPaths: []string{"/opt/veyrogram"}, Ports: []Port{{"api", "127.0.0.1:8080"}, {"manager", "127.0.0.1:9103"}},
		MySQLDSN: "u:pw@tcp(127.0.0.1:3306)/vg", RedisAddr: "127.0.0.1:6379", RedisPass: "secret-redis-pass", RequireRedis: true}
}

func find(fs []Finding, check string) *Finding {
	for i := range fs {
		if fs[i].Check == check {
			return &fs[i]
		}
	}
	return nil
}

func TestHealthyProductionHostHasNoBlockersOrWarnings(t *testing.T) {
	fs := Run(context.Background(), healthy(), prodOpts())
	for _, f := range fs {
		if f.Sev != OK {
			t.Errorf("unexpected %s on a healthy host: %+v", f.Sev, f)
		}
	}
	if len(fs) < 10 {
		t.Errorf("too few checks ran: %d", len(fs))
	}
}

// Each case breaks exactly one thing and must produce a blocker with an actionable fix.
func TestEachBlockingConditionIsReported(t *testing.T) {
	cases := []struct {
		name   string
		check  string
		break_ func(*fakeHost, *Options)
	}{
		{"not root", "privileges", func(h *fakeHost, _ *Options) { h.uid = 1000 }},
		{"no systemd", "systemd", func(h *fakeHost, _ *Options) { delete(h.paths, "/run/systemd/system") }},
		{"no service user", "service-user", func(h *fakeHost, _ *Options) { h.users = map[string]bool{} }},
		{"env file world readable", "env-file", func(h *fakeHost, _ *Options) { h.paths["/etc/veyrogram/api.env"] = PathInfo{Exists: true, Mode: 0o644} }},
		{"env file group writable", "env-file", func(h *fakeHost, _ *Options) { h.paths["/etc/veyrogram/api.env"] = PathInfo{Exists: true, Mode: 0o660} }},
		{"port taken", "port:api", func(h *fakeHost, _ *Options) { h.busy["127.0.0.1:8080"] = true }},
		{"disk nearly full", "disk:/opt/veyrogram", func(h *fakeHost, _ *Options) { h.disk = gib }},
		{"nginx missing", "nginx", func(h *fakeHost, _ *Options) { delete(h.cmds, "nginx -v") }},
		{"nginx too old", "nginx", func(h *fakeHost, _ *Options) { h.cmds["nginx -v"] = "nginx version: nginx/1.14.2" }},
		{"mysql unreachable", "mysql", func(h *fakeHost, _ *Options) { h.mysqlErr = errors.New("connection refused") }},
		{"mysql 5.7", "mysql-version", func(h *fakeHost, _ *Options) { h.mysqlInfo.Version = "5.7.44" }},
		{"mariadb", "mysql-version", func(h *fakeHost, _ *Options) { h.mysqlInfo.Version = "10.11.6-MariaDB-0ubuntu0.22.04.1" }},
		{"mysql not strict", "mysql-sql-mode", func(h *fakeHost, _ *Options) { h.mysqlInfo.SQLMode = "NO_ENGINE_SUBSTITUTION" }},
		{"mysql dsn missing", "mysql", func(_ *fakeHost, o *Options) { o.MySQLDSN = "" }},
		{"redis unreachable", "redis", func(h *fakeHost, _ *Options) { h.redisErr = errors.New("connection refused") }},
		{"redis 6", "redis-version", func(h *fakeHost, _ *Options) { h.redisInfo.Version = "6.2.14" }},
		{"redis open", "redis-auth", func(h *fakeHost, _ *Options) { h.redisInfo.OpenWithoutAuth = true }},
		{"redis evicts", "redis-eviction", func(h *fakeHost, _ *Options) { h.redisInfo.Policy = "allkeys-lru" }},
		{"redis volatile eviction", "redis-eviction", func(h *fakeHost, _ *Options) { h.redisInfo.Policy = "volatile-lru" }},
		{"redis missing but required", "redis", func(_ *fakeHost, o *Options) { o.RedisAddr = "" }},
		{"not linux in production", "platform", func(h *fakeHost, _ *Options) { h.goos = "darwin" }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h, o := healthy(), prodOpts()
			c.break_(h, &o)
			f := find(Run(context.Background(), h, o), c.check)
			if f == nil || f.Sev != Block {
				t.Fatalf("expected a blocker for %q, got %+v", c.check, f)
			}
			if f.Detail == "" {
				t.Error("a blocker must say what is wrong")
			}
		})
	}
}

// The same weak host is only warned about in development: a laptop is not production.
func TestDevelopmentOnlyWarns(t *testing.T) {
	h, o := healthy(), prodOpts()
	o.Strict, o.Install = false, false
	h.goos, h.redisInfo.OpenWithoutAuth, h.redisInfo.Policy = "darwin", true, "allkeys-lru"
	h.mysqlInfo.SQLMode = ""
	delete(h.cmds, "nginx -v")
	fs := Run(context.Background(), h, o)
	if b := Blockers(fs); len(b) != 0 {
		t.Fatalf("development must not block on hardening: %+v", b)
	}
	for _, c := range []string{"redis-auth", "redis-eviction", "mysql-sql-mode", "nginx"} {
		if f := find(fs, c); f == nil || f.Sev != Warn {
			t.Errorf("%s should warn in development, got %+v", c, f)
		}
	}
	// ...but a MySQL that is too old stays a blocker everywhere.
	h.mysqlInfo.Version = "5.7.1"
	if f := find(Run(context.Background(), h, o), "mysql-version"); f == nil || f.Sev != Block {
		t.Errorf("old MySQL must block in every environment: %+v", f)
	}
}

func TestBoundPortsAreIgnoredOnAnInstalledHost(t *testing.T) {
	h, o := healthy(), prodOpts()
	o.Fresh = false
	h.busy["127.0.0.1:8080"] = true
	if f := find(Run(context.Background(), h, o), "port:api"); f != nil {
		t.Errorf("our own running service must not be reported as a conflict: %+v", f)
	}
}

func TestWarningsDoNotBlock(t *testing.T) {
	h, o := healthy(), prodOpts()
	h.mysqlInfo.LogBin = false
	h.mysqlInfo.Charset = "latin1"
	h.redisInfo.AOF = false
	h.disk = 5 * gib
	h.cmds["timedatectl show -p NTPSynchronized --value"] = "no\n"
	fs := Run(context.Background(), h, o)
	if b := Blockers(fs); len(b) != 0 {
		t.Fatalf("these are warnings, not blockers: %+v", b)
	}
	for _, c := range []string{"mysql-binlog", "mysql-charset", "redis-persistence", "disk:/opt/veyrogram", "clock"} {
		if f := find(fs, c); f == nil || f.Sev != Warn {
			t.Errorf("%s should be a warning, got %+v", c, f)
		}
	}
}

func TestErrorsDoNotLeakSecrets(t *testing.T) {
	h, o := healthy(), prodOpts()
	h.redisErr = errors.New("dial failed using password secret-redis-pass")
	f := find(Run(context.Background(), h, o), "redis")
	if f == nil || strings.Contains(f.Detail, "secret-redis-pass") {
		t.Fatalf("the Redis password leaked into a finding: %+v", f)
	}
}

func TestParseVersionAndDiskParent(t *testing.T) {
	for in, want := range map[string][2]int{"8.0.36": {8, 0}, "8.4.1-commercial": {8, 4}, "7.2.4": {7, 2}} {
		a, b, ok := parseVersion(in)
		if !ok || a != want[0] || b != want[1] {
			t.Errorf("parseVersion(%q) = %d.%d %v", in, a, b, ok)
		}
	}
	if _, _, ok := parseVersion("garbage"); ok {
		t.Error("garbage must not parse")
	}
	if parentDir("/opt/veyrogram/api") != "/opt/veyrogram" || parentDir("/opt") != "/" || parentDir("rel") != "." {
		t.Error("parentDir")
	}
	_ = fs.ModePerm
}
