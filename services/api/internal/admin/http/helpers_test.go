package http_test

import (
	"crypto/sha256"
	"encoding/hex"
)

func sha256hex(t string) string { s := sha256.Sum256([]byte(t)); return hex.EncodeToString(s[:]) }
