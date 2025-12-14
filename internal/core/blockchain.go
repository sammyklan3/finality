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

    // Verify transactions
    for _, tx := range txns {
        if !tx.Verify() {
            return fmt.Errorf("invalid transaction signature")
        }
    }

    prev := bc.Blocks[len(bc.Blocks)-1]
    newBlock := Block{
        Index:        prev.Index + 1,
        Timestamp:    time.Now().Unix(),
        Transactions: txns,
        PrevHash:     prev.Hash,
    }

    newBlock.MineBlock(difficulty)

    if err := newBlock.SignBlock(kp); err != nil {
        return err
    }

    // Prevent duplicates
    if bc.HasBlock(newBlock.Hash) {
        return fmt.Errorf("block already exists")
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

func (bc *Blockchain) HasBlock(hash string) bool {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    for _, b := range bc.Blocks {
        if b.Hash == hash {
            return true
        }
    }
    return false
}

func (bc *Blockchain) AddBlockFromMempool(mp *Mempool, kp *KeyPair, difficulty int, maxTx int) error {
    txns := mp.GetTransactionsForBlock(maxTx)
    if len(txns) == 0 {
        return fmt.Errorf("no transactions to mine")
    }

    if err := bc.AddBlock(txns, kp, difficulty); err != nil {
        return err
    }

    // Remove mined transactions from mempool
    mp.RemoveTransactions(txns)
    return nil
}

