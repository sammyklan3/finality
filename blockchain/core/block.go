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
