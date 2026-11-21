// Package backupcrypt encrypts backups so that the host that makes them cannot read them back (Story 1.22).
//
// A backup is encrypted to an RSA public key kept on the host; the private key lives off the host, with the person who restores. A compromised host
// therefore leaks neither the backups it already shipped nor the ones it will make. Each file gets its own random AES-256 key, wrapped with RSA-OAEP
// (SHA-256) into the file header; the body is a chunked AES-GCM stream (the STREAM construction): every chunk is authenticated, its position is bound
// into its nonce, and the last chunk is flagged, so flipped bits, reordered chunks, a truncated file or an appended tail all fail instead of
// producing quiet garbage. The header (magic, wrapped key, nonce prefix, chunk size) is bound into every chunk as associated data.
//
// Format: "VGBK1\n" | uint16 len(wrappedKey) | wrappedKey | 7-byte nonce prefix | uint32 chunkSize | chunks...
// chunk i: GCM-seal(key, nonce = prefix || uint32(i) || lastFlag, plaintext <= chunkSize, aad = header). Every chunk but the last is exactly chunkSize bytes.
package backupcrypt

import (
	"bufio"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
)

const (
	magic        = "VGBK1\n"
	prefixLen    = 7
	ChunkSize    = 1 << 20 // 1 MiB of plaintext per chunk
	maxChunkSize = 16 << 20
	tagLen       = 16
	minRSABits   = 3072
)

// ErrCorrupt is returned for any authentication failure: wrong key, tampering, truncation, or a file that is not a backup.
var ErrCorrupt = errors.New("backupcrypt: the file is corrupt, truncated, tampered with, or was encrypted for a different key")

// GenerateKey makes a 4096-bit RSA key pair and returns PEM for the private (PKCS#8) and public (PKIX) halves. Run it on the machine that will restore,
// never on the backup host.
func GenerateKey() (privPEM, pubPEM []byte, err error) {
	k, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, nil, err
	}
	pk, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, nil, err
	}
	pub, err := x509.MarshalPKIXPublicKey(&k.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pk}), pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), nil
}

// ParsePublicKey reads a PKIX public key and refuses anything weaker than 3072-bit RSA.
func ParsePublicKey(p []byte) (*rsa.PublicKey, error) {
	b, _ := pem.Decode(p)
	if b == nil {
		return nil, errors.New("backupcrypt: not a PEM public key")
	}
	k, err := x509.ParsePKIXPublicKey(b.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := k.(*rsa.PublicKey)
	if !ok || pub.N.BitLen() < minRSABits {
		return nil, fmt.Errorf("backupcrypt: the public key must be RSA of at least %d bits", minRSABits)
	}
	return pub, nil
}

// ParsePrivateKey reads a PKCS#8 (or PKCS#1) RSA private key.
func ParsePrivateKey(p []byte) (*rsa.PrivateKey, error) {
	b, _ := pem.Decode(p)
	if b == nil {
		return nil, errors.New("backupcrypt: not a PEM private key")
	}
	if k, err := x509.ParsePKCS8PrivateKey(b.Bytes); err == nil {
		if rk, ok := k.(*rsa.PrivateKey); ok {
			return rk, nil
		}
		return nil, errors.New("backupcrypt: the private key is not RSA")
	}
	return x509.ParsePKCS1PrivateKey(b.Bytes)
}

func nonce(prefix []byte, i uint32, last bool) []byte {
	n := make([]byte, 12)
	copy(n, prefix)
	binary.BigEndian.PutUint32(n[prefixLen:], i)
	if last {
		n[11] = 1
	}
	return n
}

// Encrypt reads plaintext from r and writes the encrypted stream to w. Memory use is one chunk.
func Encrypt(w io.Writer, r io.Reader, pub *rsa.PublicKey) error {
	return encrypt(w, r, pub, ChunkSize)
}

func encrypt(w io.Writer, r io.Reader, pub *rsa.PublicKey, chunk int) error {
	key := make([]byte, 32)
	prefix := make([]byte, prefixLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return err
	}
	if _, err := io.ReadFull(rand.Reader, prefix); err != nil {
		return err
	}
	wrapped, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, key, []byte("veyrogram-backup"))
	if err != nil {
		return err
	}
	var hdr bytes.Buffer
	hdr.WriteString(magic)
	_ = binary.Write(&hdr, binary.BigEndian, uint16(len(wrapped)))
	hdr.Write(wrapped)
	hdr.Write(prefix)
	_ = binary.Write(&hdr, binary.BigEndian, uint32(chunk))
	if _, err := w.Write(hdr.Bytes()); err != nil {
		return err
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return err
	}
	br := bufio.NewReaderSize(r, chunk+1)
	buf := make([]byte, chunk)
	for i := uint32(0); ; i++ {
		n, err := io.ReadFull(br, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		// the chunk is last when nothing follows it (an empty input is one empty last chunk)
		_, peekErr := br.Peek(1)
		last := err == io.EOF || err == io.ErrUnexpectedEOF || peekErr == io.EOF
		if _, werr := w.Write(gcm.Seal(nil, nonce(prefix, i, last), buf[:n], hdr.Bytes())); werr != nil {
			return werr
		}
		if last {
			return nil
		}
		if i == ^uint32(0) {
			return errors.New("backupcrypt: input too large")
		}
	}
}

// Decrypt reads an encrypted stream and writes the plaintext. Plaintext already written is NOT trustworthy if an error is returned (a later chunk may
// fail): callers decrypt to a temporary file and use it only after success.
func Decrypt(w io.Writer, r io.Reader, priv *rsa.PrivateKey) error {
	br := bufio.NewReader(r)
	var hdr bytes.Buffer
	read := func(n int) ([]byte, error) {
		b := make([]byte, n)
		if _, err := io.ReadFull(br, b); err != nil {
			return nil, ErrCorrupt
		}
		hdr.Write(b)
		return b, nil
	}
	m, err := read(len(magic))
	if err != nil || string(m) != magic {
		return ErrCorrupt
	}
	lb, err := read(2)
	if err != nil {
		return err
	}
	wl := int(binary.BigEndian.Uint16(lb))
	if wl == 0 || wl > 1024 {
		return ErrCorrupt
	}
	wrapped, err := read(wl)
	if err != nil {
		return err
	}
	prefix, err := read(prefixLen)
	if err != nil {
		return err
	}
	cb, err := read(4)
	if err != nil {
		return err
	}
	chunk := int(binary.BigEndian.Uint32(cb))
	if chunk < 1 || chunk > maxChunkSize {
		return ErrCorrupt
	}
	key, err := rsa.DecryptOAEP(sha256.New(), nil, priv, wrapped, []byte("veyrogram-backup"))
	if err != nil {
		return ErrCorrupt
	}
	blk, err := aes.NewCipher(key)
	if err != nil {
		return ErrCorrupt
	}
	gcm, err := cipher.NewGCM(blk)
	if err != nil {
		return ErrCorrupt
	}
	buf := make([]byte, chunk+tagLen)
	for i := uint32(0); ; i++ {
		n, err := io.ReadFull(br, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		if n < tagLen {
			return ErrCorrupt // no last chunk: the file was cut
		}
		// A full-size chunk is the last one only if the stream ends right after it.
		_, peekErr := br.Peek(1)
		last := n < len(buf) || peekErr == io.EOF
		pt, oerr := gcm.Open(nil, nonce(prefix, i, last), buf[:n], hdr.Bytes())
		if oerr != nil {
			return ErrCorrupt
		}
		if _, werr := w.Write(pt); werr != nil {
			return werr
		}
		if last {
			return nil
		}
	}
}
