package core

import (
	"bytes"
	"encoding/gob"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"crypto/ecdsa"
)

type Block struct {
	Index        int
	Timestamp    int64
	Transactions []Transaction
	PrevHash     string
	Hash         string
	Nonce        int
	R            string // block signature r
	S            string // block signature s
}

func (b *Block) CalculateHash() string {
	var txBuffer bytes.Buffer
	enc := gob.NewEncoder(&txBuffer)
	err := enc.Encode(b.Transactions)
	if err != nil {
		panic(err)
	}

	data := fmt.Sprintf("%d%d%s%x%d", b.Index, b.Timestamp, b.PrevHash, txBuffer.Bytes(), b.Nonce)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func NewBlock(index int, timestamp int64, transactions []Transaction, prevHash string) *Block {
	block := &Block{
		Index:        index,
		Timestamp:    timestamp,
		Transactions: transactions,
		PrevHash:     prevHash,
	}
	block.Hash = block.CalculateHash()
	return block
}

func (b *Block) IsValid() bool {
	return b.Hash == b.CalculateHash()
}

// SignBlock signs the block with a private key
func (b *Block) SignBlock(kp *KeyPair) error {
	r, s, err := kp.Sign(b.Hash)
	if err != nil {
		return err
	}
	b.R = r
	b.S = s
	return nil
}

// VerifyBlockSignature verifies the block's signature
func (b *Block) VerifyBlockSignature(pub ecdsa.PublicKey) bool {
	return VerifySignature(pub, b.Hash, b.R, b.S)
}

func (b *Block) MineBlock (difficulty int) {
	target := ""
	for i := 0; i < difficulty; i++ {
		target += "0"
	}

	for {
		b.Hash = b.CalculateHash()
		if b.Hash[:difficulty] == target {
			break
		}

		b.Nonce++
	}
}