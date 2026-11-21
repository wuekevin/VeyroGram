// Package preflight checks that a host can run VeyroGram before anything is installed or changed (Story 1.21).
//
// Every check is a pure function of what the Host reports, so each blocking rule is unit-tested without a real MySQL, Redis or Nginx, and the real
// Host (OSHost) is exercised against the real services in the integration test. A check never changes the host.
//
// A finding is a blocker (the install must not continue), a warning (worth fixing, does not stop the install) or ok. What blocks depends on the
// environment: staging and production are strict, development and test only report.
package preflight

import (
	"context"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"
	"strings"
)

type Severity string

const (
	OK    Severity = "ok"
	Warn  Severity = "warn"
	Block Severity = "block"
)

// Finding is one check result. Fix says what to do about it, in a sentence an operator can act on.
type Finding struct {
	Check  string   `json:"check"`
	Sev    Severity `json:"severity"`
	Detail string   `json:"detail"`
	Fix    string   `json:"fix,omitempty"`
}

// MySQLInfo and RedisInfo are what the Host learned from the servers; the checks never talk to them directly.
type MySQLInfo struct {
	Version      string // SELECT VERSION()
	Comment      string // @@version_comment
	SQLMode      string
	Charset      string
	MaxConns     int
	LogBin       bool
	BinlogFormat string
	LocalInfile  bool
}

type RedisInfo struct {
	Version         string
	OpenWithoutAuth bool   // answered PING with no password
	Policy          string // maxmemory-policy; "" when CONFIG GET is not allowed
	AOF             bool
}

// PathInfo is what Stat reports about a path.
type PathInfo struct {
	Exists bool
	Mode   fs.FileMode
	UID    int
}

// Host is everything the checks need from the machine. OSHost is the real one; tests use a fake.
type Host interface {
	GOOS() string
	UID() int
	UserExists(name string) bool
	Stat(path string) PathInfo
	// CanListen reports whether the address can be bound right now (and releases it at once).
	CanListen(addr string) error
	// Exec runs a command and returns its combined output.
	Exec(ctx context.Context, name string, args ...string) (string, error)
	LookPath(name string) (string, bool)
	DiskFree(path string) (uint64, error)
	MySQL(ctx context.Context, dsn string) (MySQLInfo, error)
	Redis(ctx context.Context, addr, password string) (RedisInfo, error)
}

// Port is a listener the installation will need.
type Port struct{ Name, Addr string }

type Options struct {
	Strict       bool // staging / production
	Install      bool // run by the installer on the target host (root, systemd, service user)
	Fresh        bool // nothing of ours is running yet, so a bound port is somebody else's
	ServiceUser  string
	EnvFile      string
	DiskPaths    []string
	Ports        []Port
	MySQLDSN     string
	RedisAddr    string
	RedisPass    string
	RequireRedis bool
}

const (
	gib           = uint64(1) << 30
	minDiskBlock  = 2 * gib
	minDiskWarn   = 10 * gib
	minNginxMajor = 1
	minNginxMinor = 18
)

// Run executes every check and returns the findings in a stable order.
func Run(ctx context.Context, h Host, o Options) []Finding {
	var fs []Finding
	fs = append(fs, checkPlatform(h, o)...)
	fs = append(fs, checkEnvFile(h, o)...)
	fs = append(fs, checkPorts(h, o)...)
	fs = append(fs, checkDisk(h, o)...)
	fs = append(fs, checkNginx(ctx, h, o)...)
	fs = append(fs, checkMySQL(ctx, h, o)...)
	fs = append(fs, checkRedis(ctx, h, o)...)
	fs = append(fs, checkClock(ctx, h, o)...)
	return fs
}

// Blockers filters the findings that stop an install.
func Blockers(fs []Finding) []Finding {
	var b []Finding
	for _, f := range fs {
		if f.Sev == Block {
			b = append(b, f)
		}
	}
	return b
}

// strictBlock is a blocker in staging/production and a warning elsewhere.
func (o Options) severe() Severity {
	if o.Strict {
		return Block
	}
	return Warn
}

func ok(check, detail string) Finding { return Finding{Check: check, Sev: OK, Detail: detail} }

func checkPlatform(h Host, o Options) []Finding {
	var out []Finding
	if h.GOOS() != "linux" {
		out = append(out, Finding{"platform", o.severe(), "running on " + h.GOOS() + "; the production target is Linux with systemd", "install on a Linux host (this check is advisory for development machines)"})
		return out
	}
	out = append(out, ok("platform", "linux"))
	if o.Install {
		if h.Stat("/run/systemd/system").Exists {
			out = append(out, ok("systemd", "present"))
		} else {
			out = append(out, Finding{"systemd", Block, "systemd is not the running init system", "VeyroGram is deployed as systemd services (no Docker); use a systemd-based host"})
		}
		if h.UID() == 0 {
			out = append(out, ok("privileges", "running as root"))
		} else {
			out = append(out, Finding{"privileges", Block, fmt.Sprintf("running as uid %d; installing users, directories and units needs root", h.UID()), "re-run with sudo"})
		}
	}
	if o.ServiceUser != "" {
		if h.UserExists(o.ServiceUser) {
			out = append(out, ok("service-user", o.ServiceUser))
		} else {
			out = append(out, Finding{"service-user", Block, "service account " + o.ServiceUser + " does not exist", "useradd --system --home-dir /nonexistent --shell /usr/sbin/nologin " + o.ServiceUser})
		}
	}
	return out
}

// checkEnvFile enforces that secrets are not readable by other users and that a non-root user cannot rewrite them.
func checkEnvFile(h Host, o Options) []Finding {
	if o.EnvFile == "" {
		return nil
	}
	p := h.Stat(o.EnvFile)
	if !p.Exists {
		return []Finding{{"env-file", Warn, o.EnvFile + " does not exist yet", "the installer creates it from the environment template"}}
	}
	switch {
	case p.Mode.Perm()&0o007 != 0:
		return []Finding{{"env-file", Block, fmt.Sprintf("%s is readable by every user (mode %04o); it holds secrets", o.EnvFile, p.Mode.Perm()), "chmod 0640 and chown root:veyrogram"}}
	case p.Mode.Perm()&0o022 != 0:
		return []Finding{{"env-file", Block, fmt.Sprintf("%s is writable by group or others (mode %04o)", o.EnvFile, p.Mode.Perm()), "chmod 0640"}}
	case p.UID != 0 && h.GOOS() == "linux":
		return []Finding{{"env-file", o.severe(), fmt.Sprintf("%s is owned by uid %d, not root; the service account must not be able to change its own configuration", o.EnvFile, p.UID), "chown root:veyrogram " + o.EnvFile}}
	}
	return []Finding{ok("env-file", fmt.Sprintf("%s mode %04o", o.EnvFile, p.Mode.Perm()))}
}

func checkPorts(h Host, o Options) []Finding {
	var out []Finding
	for _, p := range o.Ports {
		if p.Addr == "" {
			continue
		}
		if !o.Fresh {
			continue // on an installed host our own services hold these; a probe would only cry wolf
		}
		if err := h.CanListen(p.Addr); err != nil {
			out = append(out, Finding{"port:" + p.Name, Block, p.Addr + " is already in use or cannot be bound: " + err.Error(), "stop the process using it or change the address for " + p.Name})
		} else {
			out = append(out, ok("port:"+p.Name, p.Addr+" free"))
		}
	}
	return out
}

func checkDisk(h Host, o Options) []Finding {
	var out []Finding
	for _, p := range o.DiskPaths {
		free, err := h.DiskFree(p)
		switch {
		case err != nil:
			out = append(out, Finding{"disk:" + p, Warn, "cannot read free space: " + err.Error(), ""})
		case free < minDiskBlock:
			out = append(out, Finding{"disk:" + p, Block, fmt.Sprintf("only %.1f GiB free", float64(free)/float64(gib)), "free space or mount a larger volume (need at least 2 GiB, 10 GiB recommended)"})
		case free < minDiskWarn:
			out = append(out, Finding{"disk:" + p, Warn, fmt.Sprintf("%.1f GiB free; 10 GiB or more is recommended for logs, binlogs and backups", float64(free)/float64(gib)), ""})
		default:
			out = append(out, ok("disk:"+p, fmt.Sprintf("%.0f GiB free", float64(free)/float64(gib))))
		}
	}
	return out
}

var nginxVer = regexp.MustCompile(`nginx/(\d+)\.(\d+)\.(\d+)`)

func checkNginx(ctx context.Context, h Host, o Options) []Finding {
	path, found := h.LookPath("nginx")
	if !found {
		// 宝塔 installs Nginx outside PATH.
		for _, c := range []string{"/www/server/nginx/sbin/nginx", "/usr/sbin/nginx"} {
			if h.Stat(c).Exists {
				path, found = c, true
				break
			}
		}
	}
	if !found {
		return []Finding{{"nginx", o.severe(), "nginx not found", "install Nginx (宝塔 or the distribution package); it terminates TLS in front of the API"}}
	}
	out, err := h.Exec(ctx, path, "-v")
	m := nginxVer.FindStringSubmatch(out)
	if err != nil && m == nil {
		return []Finding{{"nginx", Warn, "found " + path + " but could not read its version", ""}}
	}
	if m == nil {
		return []Finding{{"nginx", Warn, "unrecognised nginx -v output", ""}}
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < minNginxMajor || (major == minNginxMajor && minor < minNginxMinor) {
		return []Finding{{"nginx", o.severe(), fmt.Sprintf("nginx %s.%s.%s is older than 1.18", m[1], m[2], m[3]), "upgrade Nginx"}}
	}
	return []Finding{ok("nginx", fmt.Sprintf("%s (%s.%s.%s)", path, m[1], m[2], m[3]))}
}

var verNum = regexp.MustCompile(`^(\d+)\.(\d+)`)

func parseVersion(v string) (major, minor int, ok bool) {
	m := verNum.FindStringSubmatch(v)
	if m == nil {
		return 0, 0, false
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	return major, minor, true
}

func checkMySQL(ctx context.Context, h Host, o Options) []Finding {
	if o.MySQLDSN == "" {
		return []Finding{{"mysql", Block, "no MySQL connection given (VG_MYSQL_ADMIN_DSN or VG_MYSQL_DSN)", "set the DSN of an account that can read server variables"}}
	}
	info, err := h.MySQL(ctx, o.MySQLDSN)
	if err != nil {
		return []Finding{{"mysql", Block, "cannot connect: " + redactErr(err.Error(), o.MySQLDSN), "check that MySQL is running and the DSN is right"}}
	}
	var out []Finding
	major, minor, parsed := parseVersion(info.Version)
	switch {
	case strings.Contains(strings.ToLower(info.Version+" "+info.Comment), "mariadb"):
		out = append(out, Finding{"mysql-version", Block, "this is MariaDB (" + info.Version + "); VeyroGram is built and tested for MySQL 8.0", "install MySQL 8.0"})
	case !parsed:
		out = append(out, Finding{"mysql-version", Block, "cannot parse server version " + strconv.Quote(info.Version), ""})
	case major < 8:
		out = append(out, Finding{"mysql-version", Block, "MySQL " + info.Version + " is older than 8.0", "upgrade to MySQL 8.0"})
	default:
		out = append(out, ok("mysql-version", info.Version))
		_ = minor
	}
	if strings.Contains(info.SQLMode, "STRICT_TRANS_TABLES") || strings.Contains(info.SQLMode, "STRICT_ALL_TABLES") {
		out = append(out, ok("mysql-sql-mode", "strict"))
	} else {
		out = append(out, Finding{"mysql-sql-mode", o.severe(), "sql_mode is not strict (" + strconv.Quote(info.SQLMode) + "); bad data would be silently truncated", "set sql_mode to include STRICT_TRANS_TABLES"})
	}
	if info.Charset != "utf8mb4" {
		out = append(out, Finding{"mysql-charset", Warn, "server character set is " + strconv.Quote(info.Charset) + ", not utf8mb4", "set character_set_server=utf8mb4 (the schema names utf8mb4 itself, so this only affects new databases)"})
	}
	if info.MaxConns > 0 && info.MaxConns < 100 {
		out = append(out, Finding{"mysql-connections", Warn, fmt.Sprintf("max_connections is %d", info.MaxConns), "raise it to at least 200"})
	}
	if !info.LogBin || !strings.EqualFold(info.BinlogFormat, "ROW") {
		out = append(out, Finding{"mysql-binlog", Warn, "binary logging is off or not ROW format; point-in-time recovery (Story 1.22) needs it", "set log_bin and binlog_format=ROW"})
	} else {
		out = append(out, ok("mysql-binlog", "log_bin on, ROW"))
	}
	if info.LocalInfile {
		out = append(out, Finding{"mysql-local-infile", Warn, "local_infile is on", "set local_infile=OFF"})
	}
	return out
}

func checkRedis(ctx context.Context, h Host, o Options) []Finding {
	if o.RedisAddr == "" {
		if o.RequireRedis {
			return []Finding{{"redis", o.severe(), "no Redis configured; the token deny-list and rate limits need it", "set VG_REDIS_ADDR"}}
		}
		return nil
	}
	info, err := h.Redis(ctx, o.RedisAddr, o.RedisPass)
	if err != nil {
		return []Finding{{"redis", Block, "cannot connect: " + redactErr(err.Error(), o.RedisPass), "check that Redis is running, the address and the password"}}
	}
	var out []Finding
	if major, _, parsed := parseVersion(info.Version); !parsed {
		out = append(out, Finding{"redis-version", Block, "cannot parse Redis version " + strconv.Quote(info.Version), ""})
	} else if major < 7 {
		out = append(out, Finding{"redis-version", Block, "Redis " + info.Version + " is older than 7", "upgrade to Redis 7"})
	} else {
		out = append(out, ok("redis-version", info.Version))
	}
	if info.OpenWithoutAuth {
		out = append(out, Finding{"redis-auth", o.severe(), "Redis answers commands without a password", "set requirepass (or an ACL user) and put it in VG_REDIS_PASSWORD"})
	} else {
		out = append(out, ok("redis-auth", "password required"))
	}
	// The token deny-list lives here: if Redis evicts a key under memory pressure, a revoked token becomes valid again.
	switch {
	case info.Policy == "":
		out = append(out, Finding{"redis-eviction", Warn, "cannot read maxmemory-policy (CONFIG GET is disabled)", "verify by hand that maxmemory-policy is noeviction"})
	case info.Policy != "noeviction":
		out = append(out, Finding{"redis-eviction", o.severe(), "maxmemory-policy is " + info.Policy + "; Redis may evict revoked-token entries and un-revoke them", "set maxmemory-policy noeviction"})
	default:
		out = append(out, ok("redis-eviction", "noeviction"))
	}
	if !info.AOF {
		out = append(out, Finding{"redis-persistence", Warn, "append-only persistence is off; a Redis restart forgets revoked tokens until they expire", "set appendonly yes"})
	}
	return out
}

func checkClock(ctx context.Context, h Host, o Options) []Finding {
	if h.GOOS() != "linux" {
		return nil
	}
	if _, found := h.LookPath("timedatectl"); !found {
		return nil
	}
	out, err := h.Exec(ctx, "timedatectl", "show", "-p", "NTPSynchronized", "--value")
	if err != nil {
		return nil
	}
	if strings.TrimSpace(out) == "yes" {
		return []Finding{ok("clock", "synchronised")}
	}
	return []Finding{{"clock", Warn, "the clock is not NTP-synchronised; administrator authenticator codes and token expiry depend on accurate time", "enable NTP (systemd-timesyncd or chrony)"}}
}

// redactErr removes a secret that a driver error might echo back.
func redactErr(msg string, secrets ...string) string {
	for _, s := range secrets {
		if len(s) >= 6 {
			msg = strings.ReplaceAll(msg, s, "***")
		}
	}
	return msg
}
