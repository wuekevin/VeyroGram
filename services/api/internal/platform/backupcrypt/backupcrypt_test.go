package backupcrypt

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"io"
	"strings"
	"testing"
)

func keys(t *testing.T) (*rsa.PrivateKey, *rsa.PublicKey) {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatal(err)
	}
	return k, &k.PublicKey
}

func roundtrip(t *testing.T, priv *rsa.PrivateKey, pub *rsa.PublicKey, data []byte, chunk int) []byte {
	t.Helper()
	var enc bytes.Buffer
	if err := encrypt(&enc, bytes.NewReader(data), pub, chunk); err != nil {
		t.Fatal(err)
	}
	ciphertext := append([]byte(nil), enc.Bytes()...)
	var dec bytes.Buffer
	if err := Decrypt(&dec, &enc, priv); err != nil {
		t.Fatalf("decrypt (%d bytes, chunk %d): %v", len(data), chunk, err)
	}
	if !bytes.Equal(dec.Bytes(), data) {
		t.Fatalf("round trip differs for %d bytes, chunk %d", len(data), chunk)
	}
	return ciphertext
}

// Sizes around the chunk boundary are where STREAM implementations go wrong: empty, one byte, exactly one chunk, exactly two, one over.
func TestRoundTripAtChunkBoundaries(t *testing.T) {
	priv, pub := keys(t)
	for _, n := range []int{0, 1, 15, 16, 17, 31, 32, 33, 47, 48, 49, 1000} {
		data := make([]byte, n)
		_, _ = rand.Read(data)
		roundtrip(t, priv, pub, data, 16)
	}
	data := make([]byte, 3*ChunkSize+5)
	_, _ = rand.Read(data)
	roundtrip(t, priv, pub, data, ChunkSize)
}

func TestCiphertextHidesPlaintextAndDiffersEachTime(t *testing.T) {
	priv, pub := keys(t)
	data := []byte(strings.Repeat("CREATE TABLE secret_customers (", 50))
	a := roundtrip(t, priv, pub, data, 64)
	b := roundtrip(t, priv, pub, data, 64)
	if bytes.Contains(a, []byte("secret_customers")) || bytes.Equal(a, b) {
		t.Fatal("ciphertext leaks plaintext or is deterministic (a fresh key per file is required)")
	}
}

func TestTamperingTruncationReorderAndAppendAreAllDetected(t *testing.T) {
	priv, pub := keys(t)
	data := make([]byte, 100)
	_, _ = rand.Read(data)
	var enc bytes.Buffer
	if err := encrypt(&enc, bytes.NewReader(data), pub, 16); err != nil {
		t.Fatal(err)
	}
	good := enc.Bytes()
	hdrLen := len(magic) + 2 + pub.Size() + prefixLen + 4
	chunk := 16 + tagLen
	fails := func(name string, b []byte) {
		t.Helper()
		var out bytes.Buffer
		if err := Decrypt(&out, bytes.NewReader(b), priv); !errors.Is(err, ErrCorrupt) {
			t.Errorf("%s: want ErrCorrupt, got %v", name, err)
		}
	}
	flip := func(i int) []byte { b := append([]byte(nil), good...); b[i] ^= 1; return b }
	fails("flipped bit in the body", flip(hdrLen+3))
	fails("flipped bit in the header", flip(len(magic)+5))
	fails("flipped bit in the last tag", flip(len(good)-1))
	fails("truncated to a chunk boundary", good[:hdrLen+2*chunk])
	fails("truncated mid-chunk", good[:len(good)-3])
	fails("empty", nil)
	fails("appended tail", append(append([]byte(nil), good...), 1, 2, 3))
	swapped := append([]byte(nil), good...)
	copy(swapped[hdrLen:hdrLen+chunk], good[hdrLen+chunk:hdrLen+2*chunk])
	copy(swapped[hdrLen+chunk:hdrLen+2*chunk], good[hdrLen:hdrLen+chunk])
	fails("two chunks swapped", swapped)
	fails("not a backup", []byte("just some text, definitely not a backup file"))
	other, _ := keys(t)
	var out bytes.Buffer
	if err := Decrypt(&out, bytes.NewReader(good), other); !errors.Is(err, ErrCorrupt) {
		t.Errorf("wrong key: %v", err)
	}
	// a chunk cut out of the middle
	cut := append(append([]byte(nil), good[:hdrLen+chunk]...), good[hdrLen+2*chunk:]...)
	fails("a chunk removed", cut)
}

func TestKeyGenerationAndStrength(t *testing.T) {
	if testing.Short() {
		t.Skip("4096-bit generation is slow")
	}
	priv, pub, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	pk, err := ParsePrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	pb, err := ParsePublicKey(pub)
	if err != nil || pb.N.BitLen() != 4096 {
		t.Fatalf("public key: %v", err)
	}
	var enc, dec bytes.Buffer
	if err := Encrypt(&enc, strings.NewReader("hello"), pb); err != nil {
		t.Fatal(err)
	}
	if err := Decrypt(&dec, &enc, pk); err != nil || dec.String() != "hello" {
		t.Fatalf("%v %q", err, dec.String())
	}
	weak, _ := rsa.GenerateKey(rand.Reader, 2048)
	if _, err := ParsePublicKey(pemPub(t, &weak.PublicKey)); err == nil {
		t.Error("a 2048-bit key must be refused")
	}
	if _, err := ParsePublicKey([]byte("nope")); err == nil {
		t.Error("garbage must be refused")
	}
}

type countingWriter struct{ n int }

func (c *countingWriter) Write(p []byte) (int, error) { c.n += len(p); return len(p), nil }

func TestStreamsLargeInputWithoutBuffering(t *testing.T) {
	priv, pub := keys(t)
	pr, pw := io.Pipe()
	go func() {
		blk := make([]byte, 1<<20)
		for i := 0; i < 20; i++ {
			_, _ = pw.Write(blk)
		}
		pw.Close()
	}()
	er, ew := io.Pipe()
	go func() { er2 := Encrypt(ew, pr, pub); ew.CloseWithError(er2) }()
	var cw countingWriter
	if err := Decrypt(&cw, er, priv); err != nil || cw.n != 20<<20 {
		t.Fatalf("%v %d", err, cw.n)
	}
}

func pemPub(t *testing.T, k *rsa.PublicKey) []byte {
	t.Helper()
	b, err := x509MarshalPKIX(k)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
