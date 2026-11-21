package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"veyrogram/api/internal/platform/backupcrypt"
)

// runBackup implements the encryption side of the backup tooling (infrastructure/backup/*.sh call it):
//
//	backup keygen -out-dir D            make the key pair (run on YOUR machine, never on the backup host); private.pem is 0600
//	backup encrypt -pub P [-out F]      stdin -> encrypted file (default stdout); needs only the PUBLIC key
//	backup decrypt -key K [-in F] [-out F]   encrypted file -> plaintext; with -out the file appears only after the whole stream has authenticated
func runBackup(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: backup keygen -out-dir D | encrypt -pub P [-out F] | decrypt -key K [-in F] [-out F]")
		return 2
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	switch args[0] {
	case "keygen":
		dir := fs.String("out-dir", "", "directory for private.pem and public.pem")
		if fs.Parse(args[1:]) != nil || *dir == "" {
			fmt.Fprintln(stderr, "-out-dir is required")
			return 2
		}
		priv, pub, err := backupcrypt.GenerateKey()
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		if err := os.MkdirAll(*dir, 0o700); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		for _, f := range []struct {
			name string
			data []byte
			mode os.FileMode
		}{{"private.pem", priv, 0o600}, {"public.pem", pub, 0o644}} {
			p := filepath.Join(*dir, f.name)
			if _, err := os.Stat(p); err == nil {
				fmt.Fprintln(stderr, "refused: "+p+" already exists; will not overwrite a key")
				return 1
			}
			if err := os.WriteFile(p, f.data, f.mode); err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
		}
		fmt.Fprintf(stdout, "wrote %s/private.pem (keep it OFF the backup host, in at least two safe places: without it no backup can be read) and %s/public.pem (copy this one to the host)\n", *dir, *dir)
		return 0
	case "encrypt":
		pubPath := fs.String("pub", "", "public key (PEM)")
		out := fs.String("out", "", "output file (default stdout)")
		if fs.Parse(args[1:]) != nil || *pubPath == "" {
			fmt.Fprintln(stderr, "-pub is required")
			return 2
		}
		raw, err := os.ReadFile(*pubPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		pub, err := backupcrypt.ParsePublicKey(raw)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return writeResult(*out, stdout, stderr, func(w io.Writer) error { return backupcrypt.Encrypt(w, stdin, pub) })
	case "decrypt":
		keyPath := fs.String("key", "", "private key (PEM)")
		in := fs.String("in", "", "encrypted file (default stdin)")
		out := fs.String("out", "", "plaintext output file (default stdout; prefer -out: it is only created after the whole file verified)")
		if fs.Parse(args[1:]) != nil || *keyPath == "" {
			fmt.Fprintln(stderr, "-key is required")
			return 2
		}
		raw, err := os.ReadFile(*keyPath)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		priv, err := backupcrypt.ParsePrivateKey(raw)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		src := stdin
		if *in != "" {
			f, err := os.Open(*in)
			if err != nil {
				fmt.Fprintln(stderr, "error:", err)
				return 1
			}
			defer f.Close()
			src = f
		}
		return writeResult(*out, stdout, stderr, func(w io.Writer) error { return backupcrypt.Decrypt(w, src, priv) })
	}
	fmt.Fprintln(stderr, "unknown backup command")
	return 2
}

// writeResult runs fill into a temporary file next to out (mode 0600) and renames it into place only on success, so a failed or interrupted run never leaves
// a file that looks complete. With no out it streams to stdout.
func writeResult(out string, stdout, stderr io.Writer, fill func(io.Writer) error) int {
	if out == "" {
		if err := fill(stdout); err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 1
		}
		return 0
	}
	tmp, err := os.CreateTemp(filepath.Dir(out), ".vgbk-*")
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	defer os.Remove(tmp.Name())
	if err := fill(tmp); err != nil {
		tmp.Close()
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err := tmp.Close(); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	if err := os.Rename(tmp.Name(), out); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
}
