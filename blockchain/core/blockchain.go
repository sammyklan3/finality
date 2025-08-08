package core

import (
	"time"
	"sync"
	"fmt"
)

type Blockchain struct {
	Blocks []Block
	mu     sync.Mutex
}

func NewBlockchain() *Blockchain {
	genesis := Block{
		Index:        0,
		Timestamp:    time.Now().Unix(),
		Transactions: []Transaction{},
		PrevHash:     "",
	}
	genesis.Hash = genesis.CalculateHash()
	return &Blockchain{Blocks: []Block{genesis}}
}

func (bc *Blockchain) AddBlock(txns []Transaction, kp *KeyPair, difficulty int) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Verify all transactions
	for _, tx := range txns {
		if !tx.Verify() {
			return fmt.Errorf("invalid transaction detected")
		}
	}

	prev := bc.Blocks[len(bc.Blocks)-1]
	newBlock := Block{
		Index:        prev.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: txns,
		PrevHash:     prev.Hash,
	}

	// Mine with Proof-of-Work
	newBlock.MineBlock(difficulty)

	// Sign block
	if err := newBlock.SignBlock(kp); err != nil {
		return err
	}

	bc.Blocks = append(bc.Blocks, newBlock)
	return nil
}


func (bc *Blockchain) LatestBlock() *Block {
	return &bc.Blocks[len(bc.Blocks)-1]
}

func (bc *Blockchain) IsValid() bool {
	for i := 1; i < len(bc.Blocks); i++ {
		current := bc.Blocks[i]
		prev := bc.Blocks[i-1]

		if current.Hash != current.CalculateHash() {
			return false
		}
		if current.PrevHash != prev.Hash {
			return false
		}
	}
	return true
}

