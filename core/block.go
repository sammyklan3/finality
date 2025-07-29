package core

import (
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
}

func (b *Block) CalculateHash() string {
	data := fmt.Sprintf("%d%d%s", b.Index, b.Timestamp, b.PrevHash)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}