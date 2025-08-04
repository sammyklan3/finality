package core

import (
	"time"
)

type Blockchain struct {
	Blocks []Block
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

func (bc *Blockchain) AddBlock(txns []Transaction) {
	prev := bc.Blocks[len(bc.Blocks)-1]
	newBlock := Block{
		Index:        prev.Index + 1,
		Timestamp:    time.Now().Unix(),
		Transactions: txns,
		PrevHash:     prev.Hash,
	}
	newBlock.Hash = newBlock.CalculateHash()
	bc.Blocks = append(bc.Blocks, newBlock)
}

func (bc *Blockchain) LatestBlock() *Block {
	return &bc.Blocks[len(bc.Blocks)-1]
}
