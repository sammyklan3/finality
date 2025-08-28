package blockchain

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	GENESIS_BLOCK_HASH []byte = []byte("000000")
)

type Blockchain struct {
	blockId *atomic.Uint32
	mu      sync.RWMutex

	Wallet       Wallet        `json:"wallet"`
	Blocks       []SignedBlock `json:"blocks"`
	CurrentBlock *Block        `json:"current_block"`
}

func NewBlockchain(wallet Wallet) (*Blockchain, error) {
	isEmpty := strings.TrimSpace(wallet.Owner) == ""
	if isEmpty {
		return nil, fmt.Errorf("Missing blockchain owner")
	}

	// TODO: Implement loading of blockchain from file

	log.Printf("GENESIS; Block 0 - owned by %v\n", wallet.Owner)

	blockId := atomic.Uint32{}
	block, err := NewBlock(0, GENESIS_BLOCK_HASH, blockId.Add(1), wallet.Owner)
	if err != nil {
		return nil, fmt.Errorf("Error creating GENESIS block; %v", err)
	}

	return &Blockchain{
		blockId: &blockId,
		mu:      sync.RWMutex{},

		Wallet:       wallet,
		Blocks:       []SignedBlock{},
		CurrentBlock: block,
	}, nil
}

func (b *Blockchain) AddBlock(signedBlock *SignedBlock, pubKey []byte, signature []byte) error {
	err := signedBlock.Block.VerifyBlock(pubKey, signature)
	if err != nil {
		return err
	}

	// TODO: Check if block already exists b4 appending it

	b.mu.Lock()
	b.Blocks = append(b.Blocks, *signedBlock)
	b.mu.Unlock()

	return nil
}

// Adds a transaction onto the latest block in the chain. If the latest block if full,
// AddTransaction signs the block, appends it onto its own chain and sends the block to channel
// for further processing. A new block is thereafter created which now becomes the latest block
func (b *Blockchain) AddTransaction(t Transaction, signedBlocks chan<- SignedBlock) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	currentBlock := b.CurrentBlock
	if currentBlock == nil {
		return fmt.Errorf("Blockchain initialized incorrectly; Missing GENESIS block")
	}

	privateKey, err := b.Wallet.PrivateKey()
	if err != nil {
		return err
	}

	err = currentBlock.AddTransaction(t, *privateKey)
	blockIsFull := errors.Is(err, ErrBlockFull)

	if blockIsFull {
		signedBlock, err := NewSignedBlock(currentBlock, privateKey)
		if err != nil {
			return fmt.Errorf("Error signing block; %v", err)
		}

		b.Blocks = append(b.Blocks, *signedBlock)

		newBlock, err := NewBlock(signedBlock.Id, signedBlock.Hash, b.blockId.Add(1), b.Wallet.Owner)
		if err != nil {
			return fmt.Errorf("Error creating new block; %v\n", err)
		}

		signedBlocks <- *signedBlock

		// Set new block as current block
		b.CurrentBlock = newBlock
		return nil
	}

	return err
}
