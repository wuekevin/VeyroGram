package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"veyrogram/api/internal/admin/domain"
	"veyrogram/api/internal/platform/config"
)

// runAdmin implements `veyrogram-api admin bootstrap -username U`. The password is never accepted as an argument (it would land in shell history and the
// process list): it comes from VG_ADMIN_BOOTSTRAP_PASSWORD or, with -password-stdin, from standard input.
func runAdmin(args []string, getenv func(string) string, cfg config.Config, pool *sql.DB, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "bootstrap" {
		fmt.Fprintln(stderr, "usage: admin bootstrap -username U [-password-stdin]   (or VG_ADMIN_BOOTSTRAP_PASSWORD)")
		return 2
	}
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
	fs.SetOutput(stderr)
	username := fs.String("username", "", "login name of the first administrator")
	stdin := fs.Bool("password-stdin", false, "read the password from standard input")
	if err := fs.Parse(args[1:]); err != nil || *username == "" {
		fmt.Fprintln(stderr, "-username is required")
		return 2
	}
	password, err := bootstrapPassword(getenv, *stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	return bootstrapAdmin(cfg, pool, *username, password, stderr)
}

// bootstrapPassword reads the first administrator's password: from standard input, or else from VG_ADMIN_BOOTSTRAP_PASSWORD.
func bootstrapPassword(getenv func(string) string, fromStdin bool) (string, error) {
	if !fromStdin {
		return getenv("VG_ADMIN_BOOTSTRAP_PASSWORD"), nil
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", errors.New("no password on standard input")
	}
	return strings.TrimRight(line, "\r\n"), nil
}

// bootstrapAdmin creates the first administrator and prints the one-time authenticator secret. Shared by `admin bootstrap` and the install wizard.
func bootstrapAdmin(cfg config.Config, pool *sql.DB, username, password string, stderr io.Writer) int {
	if len(cfg.AdminTOTPKey) < 32 {
		fmt.Fprintln(stderr, "VG_ADMIN_TOTP_KEY (at least 32 characters) is required: it encrypts the administrator's TOTP secret")
		return exitConfig
	}
	svc, err := adminService(cfg, pool)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return exitConfig
	}
	prov, err := svc.Bootstrap(context.Background(), username, password)
	var weak *domain.WeakPasswordError
	switch {
	case errors.As(err, &weak):
		fmt.Fprintln(stderr, "refused: the password does not meet the policy:")
		for _, p := range weak.Problems {
			fmt.Fprintln(stderr, "  -", p)
		}
		return 1
	case errors.Is(err, domain.ErrAlreadyInitialized):
		fmt.Fprintln(stderr, "refused: this installation already has its first administrator (initialisation works once; the decision is stored in the database, not in a file)")
		return 1
	case errors.Is(err, domain.ErrInvalidInput):
		fmt.Fprintln(stderr, "refused: the username must be 3-64 characters of letters, digits, dot, dash or underscore")
		return 1
	case err != nil:
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	fmt.Printf("first administrator %q created (role owner).\n", username)
	fmt.Println("Add this to an authenticator app NOW; it is shown only once and is required for the first sign-in:")
	fmt.Println("  secret:", prov.Secret)
	fmt.Println("  uri:   ", prov.URI)
	return 0
}
