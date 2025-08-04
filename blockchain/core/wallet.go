package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
)

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

// NewWallet creates a new Wallet with a generated ECDSA key pair.
func NewWallet() *Wallet {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return &Wallet{
		PrivateKey: priv,
		PublicKey:  &priv.PublicKey,
	}
}

func (w *Wallet) GetAddress() string {
	return fmt.Sprintf("%x", w.PublicKey.X)
}

func (w *Wallet) Sign(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to sign cannot be empty")
	}
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, data)
	if err != nil {
		return nil, fmt.Errorf("failed to sign data: %v", err)
	}
	return append(r.Bytes(), s.Bytes()...), nil
}

func (w *Wallet) Verify(data []byte, sig []byte) bool {
	if len(sig) != 64 {
		return false
	}
	r := big.Int{}
	s := big.Int{}
	r.SetBytes(sig[:32])
	s.SetBytes(sig[32:])
	return ecdsa.Verify(w.PublicKey, data, &r, &s)
}

func (w *Wallet) ExportPrivateKey() ([]byte, error) {
	privBytes, err := x509.MarshalPKCS8PrivateKey(w.PrivateKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}), nil
}

func (w *Wallet) ExportPublicKey() ([]byte, error) {
	pubBytes, err := x509.MarshalPKIXPublicKey(w.PublicKey)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}), nil
}
