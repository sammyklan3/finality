package blockchain

import (
	"fmt"
	"strings"
	"sync"
)

var mu sync.RWMutex = sync.RWMutex{}

type Blockchain struct {
	Name         string        `json:"name"`
	Blocks       []SignedBlock `json:"blocks"`
	CurrentBlock *Block        `json:"current_block"`
}

func NewBlockchain(owner string) (*Blockchain, error) {
	isEmpty := strings.TrimSpace(owner) == ""
	if isEmpty {
		return nil, fmt.Errorf("Missing blockchain owner")
	}

	// TODO: Implement loading of blockchain from file

	return &Blockchain{
		Name:   owner,
		Blocks: []SignedBlock{},
	}, nil
}

func (b *Blockchain) AddBlock(signedBlock *SignedBlock, pubKey []byte, signature []byte) error {
	err := signedBlock.Block.VerifyBlock(pubKey, signature)
	if err != nil {
		return err
	}

	// TODO: Check if block already exists b4 appending it

	mu.Lock()
	b.Blocks = append(b.Blocks, *signedBlock)
	mu.Unlock()

	return nil
}
