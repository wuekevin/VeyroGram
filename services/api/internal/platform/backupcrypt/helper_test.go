package backupcrypt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
)

func x509MarshalPKIX(k *rsa.PublicKey) ([]byte, error) {
	b, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: b}), nil
}
