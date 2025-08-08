package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"math/big"
	"encoding/hex"
)

type KeyPair struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  *ecdsa.PublicKey
}

func NewKeyPair() (*KeyPair, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pub := &priv.PublicKey
	return &KeyPair{PrivateKey: priv, PublicKey: pub}, nil
}

func (kp *KeyPair) Sign(data string) (string, string, error) {
	hash := sha256.Sum256([]byte(data))
	r, s, err := ecdsa.Sign(rand.Reader, kp.PrivateKey, hash[:])
	if err != nil {
		return "", "", err
	}

	return r.Text(16), s.Text(16), nil
}

func VerifySignature(pub ecdsa.PublicKey, data, rText, sText string) bool {
	hash := sha256.Sum256([]byte(data))
	r := new(big.Int)
	s := new(big.Int)

	r.SetString(rText, 16)
	s.SetString(sText, 16)

	valid := ecdsa.Verify(&pub, hash[:], r, s)
	return valid
}

// PublicKeyToHex converts public key to hex string
func PublicKeyToHex(pub *ecdsa.PublicKey) string {
	return hex.EncodeToString(append(pub.X.Bytes(), pub.Y.Bytes()...))
}