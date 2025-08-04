package core

import (
	"bytes"
	"encoding/gob"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

type Block struct {
	Index        int
	Timestamp    int64
	Transactions []Transaction
	PrevHash     string
	Hash         string
	Nonce        int
}

// CalculateHash computes the hash of the block based on its contents.
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

// NewBlock creates a new block with the given parameters and calculates its hash.
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
