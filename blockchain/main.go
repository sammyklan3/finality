package main

import (
	"fmt"
	"log"

	"github.com/sammyklan3/finality/blockchain/core"
)

func main() {
	bc := core.NewBlockchain()

	sender, _ := core.NewKeyPair()
	receiver, _ := core.NewKeyPair()

	tx := core.Transaction{
		From:   core.PublicKeyToHex(sender.PublicKey),
		To:     core.PublicKeyToHex(receiver.PublicKey),
		Amount: 10,
	}
	if err := tx.SignTransaction(sender); err != nil {
		log.Fatal(err)
	}

	if err := bc.AddBlock([]core.Transaction{tx}, sender, 3); err != nil {
		log.Fatal(err)
	}

	for _, block := range bc.Blocks {
		fmt.Printf("\nBlock #%d\nHash: %s\nPrevHash: %s\n", block.Index, block.Hash, block.PrevHash)
		for _, t := range block.Transactions {
			fmt.Printf("TX from %s... to %s... amount %.2f\n", t.From[:10], t.To[:10], t.Amount)
		}
	}

	fmt.Println("\nBlockchain valid?", bc.IsValid())
}
