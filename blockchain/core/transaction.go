package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

type Transaction struct {
	From   string  // hex public key
	To     string  // hex public key
	Amount float64
	R      string // signature R
	S      string // signature S
}

func (tx *Transaction) Hash() string {
	data := fmt.Sprintf("%s%s%f", tx.From, tx.To, tx.Amount)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (tx *Transaction) SignTransaction(kp *KeyPair) error {
	r, s, err := kp.Sign(tx.Hash())
	if err != nil {
		return err
	}
	tx.R = r
	tx.S = s
	return nil
}

func (tx *Transaction) Verify() bool {
	pubBytes, err := hex.DecodeString(tx.From)
	if err != nil || len(pubBytes) == 0 {
		return false
	}

	half := len(pubBytes) / 2
	x := new(big.Int).SetBytes(pubBytes[:half])
	y := new(big.Int).SetBytes(pubBytes[half:])
	pubKey := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	return VerifySignature(pubKey, tx.Hash(), tx.R, tx.S)
}
