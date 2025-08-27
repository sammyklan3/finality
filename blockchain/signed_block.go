package blockchain

import (
	"crypto/ecdsa"
	"fmt"
)

type SignedBlock struct {
	Block
	Hash      []byte `json:"hash"`
	Signature []byte `json:"signature"`
}

func NewSignedBlock(block Block, privateKey *ecdsa.PrivateKey) (*SignedBlock, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("NIL private key")
	}

	hash, err := block.Hash()
	if err != nil {
		return nil, err
	}
	signature, err := block.SignBlock(privateKey)
	if err != nil {
		return nil, err
	}
	return &SignedBlock{
		Block:     block,
		Hash:      hash,
		Signature: signature,
	}, nil
}
