package blockchain

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/sammyklan3/finality/blockchain/utils/mcrypto"
)

type Wallet struct {
	Owner           string
	PrivateKeyBytes []byte // We export the key as PEM encoded bytes

	privateKey *ecdsa.PrivateKey
}

func NewWallet(owner string) (*Wallet, error) {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return nil, fmt.Errorf("Wallet must have an owner")
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("Error generating ECDSA key pair; %v", err)
	}

	privateKeyBytes, err := mcrypto.EncodeECDSAPrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("Error encoding ECDSA private key; %v", err)
	}

	return &Wallet{
		Owner:           strings.ToUpper(owner),
		PrivateKeyBytes: privateKeyBytes,
		privateKey:      privateKey,
	}, nil
}

func (w *Wallet) PrivateKey() (*ecdsa.PrivateKey, error) {
	privateKey := w.privateKey
	if privateKey != nil {
		return privateKey, nil
	}

	// Parse private key from private key bytes
	privateKey, err := mcrypto.DecodeECDSAPrivateKey(w.PrivateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Error decoding ECDSA private key; %v", err)
	}
	w.privateKey = privateKey
	return privateKey, nil
}

func (w Wallet) Sign(digest []byte) (string, error) {
	signature, err := w.privateKey.Sign(rand.Reader, digest, crypto.MD5)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}
