package main

import (
	"fmt"
	"log"

	"github.com/sammyklan3/finality/blockchain/core"
)

func main() {
	// Create blockchain
	bc := core.NewBlockchain()

	// Create sender & receiver keys
	sender, _ := core.NewKeyPair()
	receiver, _ := core.NewKeyPair()

	// Create and sign a transaction
	tx := core.Transaction{
		From:   core.PublicKeyToHex(sender.PublicKey),
		To:     core.PublicKeyToHex(receiver.PublicKey),
		Amount: 10,
	}
	if err := tx.SignTransaction(sender); err != nil {
		log.Fatal(err)
	}

	// Add block with transaction
	err := bc.AddBlock([]core.Transaction{tx}, sender, 3) // difficulty=3
	if err != nil {
		log.Fatal(err)
	}

	// Print chain
	for _, block := range bc.Blocks {
		fmt.Printf("\nBlock #%d\nHash: %s\nPrevHash: %s\n", block.Index, block.Hash, block.PrevHash)
		for _, t := range block.Transactions {
			fmt.Printf("TX from %s to %s amount %.2f\n", t.From[:10], t.To[:10], t.Amount)
		}
	}

	// Validate chain
	fmt.Println("\nBlockchain valid?", bc.IsValid())
}
