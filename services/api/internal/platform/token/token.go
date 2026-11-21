// Package token issues and verifies short-lived access tokens (ARCHITECTURE AD-9): compact JWS signed with Ed25519, a `kid` header
// for key rotation, an algorithm allow-list of exactly one entry (never "none"), and standard-library cryptography only.
// The first key in the ring signs; every key in the ring verifies, so rotation = add the new key first, retire the old one after the TTL.
package token

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Claims carried by an access token. IDs are opaque strings; no phone numbers or other PII.
type Claims struct {
	Subject  string `json:"sub"` // user uid
	Device   string `json:"did"` // device id (decimal string)
	Class    string `json:"cls"` // app|web|pc
	Tenant   uint64 `json:"tid"`
	ID       string `json:"jti"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
	Issuer   string `json:"iss"`
}

var (
	ErrInvalid = errors.New("token: invalid")
	ErrExpired = errors.New("token: expired")
)

const issuer = "veyrogram"

// Ring is a set of Ed25519 keys addressed by kid.
type Ring struct {
	signKid string
	priv    map[string]ed25519.PrivateKey
}

// ParseRing parses "kid:base64url(32-byte seed)[,kid:seed...]". The first entry signs.
func ParseRing(spec string) (*Ring, error) {
	r := &Ring{priv: map[string]ed25519.PrivateKey{}}
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kid, b64, ok := strings.Cut(part, ":")
		if !ok || kid == "" || len(kid) > 32 || strings.ContainsAny(kid, ". \"\\") {
			return nil, fmt.Errorf("token: key entries look like kid:base64url-seed")
		}
		seed, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(b64, "="))
		if err != nil || len(seed) != ed25519.SeedSize {
			return nil, fmt.Errorf("token: key %q must be a base64url-encoded 32-byte seed", kid)
		}
		if allSame(seed) {
			return nil, fmt.Errorf("token: key %q is a trivial seed", kid)
		}
		if _, dup := r.priv[kid]; dup {
			return nil, fmt.Errorf("token: duplicate kid %q", kid)
		}
		r.priv[kid] = ed25519.NewKeyFromSeed(seed)
		if r.signKid == "" {
			r.signKid = kid
		}
	}
	if r.signKid == "" {
		return nil, errors.New("token: no signing key configured")
	}
	return r, nil
}

func allSame(b []byte) bool {
	for _, c := range b {
		if c != b[0] {
			return false
		}
	}
	return true
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

var enc = base64.RawURLEncoding

// Sign returns a compact JWS for c, filling Issuer.
func (r *Ring) Sign(c Claims) (string, error) {
	c.Issuer = issuer
	h, _ := json.Marshal(header{Alg: "EdDSA", Typ: "JWT", Kid: r.signKid})
	p, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	signing := enc.EncodeToString(h) + "." + enc.EncodeToString(p)
	sig := ed25519.Sign(r.priv[r.signKid], []byte(signing))
	return signing + "." + enc.EncodeToString(sig), nil
}

// Verify checks signature, algorithm, kid, issuer and expiry at now.
func (r *Ring) Verify(tok string, now time.Time) (Claims, error) {
	var zero Claims
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		return zero, ErrInvalid
	}
	hb, err := enc.DecodeString(parts[0])
	if err != nil {
		return zero, ErrInvalid
	}
	var h header
	if json.Unmarshal(hb, &h) != nil || h.Alg != "EdDSA" {
		return zero, ErrInvalid
	}
	key, ok := r.priv[h.Kid]
	if !ok {
		return zero, ErrInvalid
	}
	sig, err := enc.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key.Public().(ed25519.PublicKey), []byte(parts[0]+"."+parts[1]), sig) {
		return zero, ErrInvalid
	}
	pb, err := enc.DecodeString(parts[1])
	if err != nil {
		return zero, ErrInvalid
	}
	var c Claims
	if json.Unmarshal(pb, &c) != nil || c.Issuer != issuer || c.Subject == "" {
		return zero, ErrInvalid
	}
	if now.Unix() >= c.Expires {
		return zero, ErrExpired
	}
	return c, nil
}
