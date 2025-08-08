package core

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"crypto/sha256"
	"fmt"
	"math/big"
)

type Transaction struct {
	From      string
	To        string
	Amount    float64
	R         string
	S         string
}

// Hash returns the hash of the transaction
func (tx *Transaction) Hash() string {
	data := fmt.Sprintf("%s%s%f", tx.From, tx.To, tx.Amount)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// SignTransaction signs the transaction using the sender's private key
func (tx *Transaction) SignTransaction(kp *KeyPair) error {
	r, s, err := kp.Sign(tx.Hash())
	if err != nil {
		return err
	}
	tx.R = r
	tx.S = s
	return nil
}

// Verify checks if the transaction's signature is valid
func (tx *Transaction) Verify() bool {
	// Decode public key from hex
	pubBytes, err := hex.DecodeString(tx.From)
	if err != nil {
		return false
	}
	x := new(big.Int).SetBytes(pubBytes[:len(pubBytes)/2])
	y := new(big.Int).SetBytes(pubBytes[len(pubBytes)/2:])
	pubKey := ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}

	return VerifySignature(pubKey, tx.Hash(), tx.R, tx.S)
}
