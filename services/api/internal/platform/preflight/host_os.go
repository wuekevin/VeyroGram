//go:build unix

package preflight

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"
)

// OSHost is the Host backed by the real machine and real servers.
type OSHost struct{}

func (OSHost) GOOS() string { return runtime.GOOS }
func (OSHost) UID() int     { return os.Getuid() }

func (OSHost) UserExists(name string) bool {
	_, err := user.Lookup(name)
	return err == nil
}

func (OSHost) Stat(path string) PathInfo {
	fi, err := os.Stat(path)
	if err != nil {
		return PathInfo{}
	}
	p := PathInfo{Exists: true, Mode: fi.Mode()}
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		p.UID = int(st.Uid)
	}
	return p
}

func (OSHost) CanListen(addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return l.Close()
}

func (OSHost) Exec(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var b bytes.Buffer
	c := exec.CommandContext(ctx, name, args...)
	c.Stdout, c.Stderr = &b, &b
	err := c.Run()
	return b.String(), err
}

func (OSHost) LookPath(name string) (string, bool) {
	p, err := exec.LookPath(name)
	return p, err == nil
}

func (OSHost) DiskFree(path string) (uint64, error) {
	// A path that is not created yet is measured at its nearest existing parent.
	for p := path; ; {
		var st syscall.Statfs_t
		err := syscall.Statfs(p, &st)
		if err == nil {
			return uint64(st.Bavail) * uint64(st.Bsize), nil
		}
		if !errors.Is(err, fs.ErrNotExist) || p == "/" || p == "." {
			return 0, err
		}
		p = parentDir(p)
	}
}

func parentDir(p string) string {
	i := strings.LastIndex(strings.TrimRight(p, "/"), "/")
	switch {
	case i < 0:
		return "."
	case i == 0:
		return "/"
	}
	return p[:i]
}

func (OSHost) MySQL(ctx context.Context, dsn string) (MySQLInfo, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return MySQLInfo{}, errors.New("invalid DSN")
	}
	cfg.Timeout = 5 * time.Second
	conn, err := mysql.NewConnector(cfg)
	if err != nil {
		return MySQLInfo{}, errors.New("invalid DSN")
	}
	d := sql.OpenDB(conn)
	defer d.Close()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var info MySQLInfo
	var logBin, infile int
	var sqlMode, charset sql.NullString
	err = d.QueryRowContext(ctx, "SELECT VERSION(), @@version_comment, @@sql_mode, @@character_set_server, @@max_connections, @@log_bin, @@local_infile").
		Scan(&info.Version, &info.Comment, &sqlMode, &charset, &info.MaxConns, &logBin, &infile)
	if err != nil {
		return MySQLInfo{}, redactSecret(err, cfg.Passwd)
	}
	info.SQLMode, info.Charset, info.LogBin, info.LocalInfile = sqlMode.String, charset.String, logBin == 1, infile == 1
	var bf sql.NullString
	if err := d.QueryRowContext(ctx, "SELECT @@binlog_format").Scan(&bf); err == nil {
		info.BinlogFormat = bf.String
	}
	return info, nil
}

func (OSHost) Redis(ctx context.Context, addr, password string) (RedisInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var info RedisInfo
	// First without credentials: a server that answers is open to anything that can reach it.
	anon := redis.NewClient(&redis.Options{Addr: addr, DialTimeout: 3 * time.Second, MaxRetries: -1})
	defer anon.Close()
	if err := anon.Ping(ctx).Err(); err == nil {
		info.OpenWithoutAuth = true
	}
	c := redis.NewClient(&redis.Options{Addr: addr, Password: password, DialTimeout: 3 * time.Second, MaxRetries: -1})
	defer c.Close()
	if err := c.Ping(ctx).Err(); err != nil {
		return RedisInfo{}, redactSecret(err, password)
	}
	srv, err := c.Info(ctx, "server").Result()
	if err != nil {
		return RedisInfo{}, redactSecret(err, password)
	}
	info.Version = infoField(srv, "redis_version")
	if pol, err := c.ConfigGet(ctx, "maxmemory-policy").Result(); err == nil {
		info.Policy = pol["maxmemory-policy"]
	}
	if aof, err := c.ConfigGet(ctx, "appendonly").Result(); err == nil {
		info.AOF = aof["appendonly"] == "yes"
	}
	return info, nil
}

func infoField(info, key string) string {
	for _, line := range strings.Split(info, "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), key+":"); ok {
			return v
		}
	}
	return ""
}

func redactSecret(err error, secret string) error {
	if secret == "" {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), secret, "***"))
}
