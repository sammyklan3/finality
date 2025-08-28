package blockchain

import (
	"crypto/ecdsa"
	"fmt"
	"log"
)

type SignedBlock struct {
	*Block
	Hash      []byte `json:"hash"`
	Signature []byte `json:"signature"`
}

func NewSignedBlock(block *Block, privateKey *ecdsa.PrivateKey) (*SignedBlock, error) {
	if privateKey == nil {
		return nil, fmt.Errorf("NIL private key")
	}
	hash, signature, err := block.signBlock(privateKey)
	if err != nil {
		return nil, err
	}

	signedBlock := SignedBlock{
		Block:     block,
		Hash:      hash,
		Signature: signature,
	}
	log.Printf("%s\n", signedBlock)

	return &signedBlock, nil
}

func (b SignedBlock) String() string {
	return fmt.Sprintf("\n\n==========\nSignedBlock<hash=%x, sig=%x>\n==========\n", b.Hash, b.Signature)
}
