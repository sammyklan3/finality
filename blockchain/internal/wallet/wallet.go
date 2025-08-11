package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"crypto/sha256"
	"fmt"
	"math/big"
)

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

func hashData(data []byte) []byte {
    h := sha256.Sum256(data)
    return h[:]
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
    pubKeyBytes := append(w.PublicKey.X.Bytes(), w.PublicKey.Y.Bytes()...)
    hashed := sha256.Sum256(pubKeyBytes)
    return fmt.Sprintf("%x", hashed[:20]) // 20 byte address
}


func (w *Wallet) Sign(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data to sign cannot be empty")
	}

	hashedData := hashData(data)
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, hashedData)
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
	hashedData := hashData(data)
	return ecdsa.Verify(w.PublicKey, hashedData, &r, &s)
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

func ImportPrivateKey(pemBytes []byte) (*Wallet, error) {
   block, _ := pem.Decode(pemBytes)
   if block == nil || block.Type != "PRIVATE KEY" {
       return nil, fmt.Errorf("invalid PEM format")
   }
   key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
   if err != nil {
       return nil, err
   }
   priv, ok := key.(*ecdsa.PrivateKey)
   if !ok {
       return nil, fmt.Errorf("not an ECDSA private key")
   }
   return &Wallet{PrivateKey: priv, PublicKey: &priv.PublicKey}, nil
}
