package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"time"

	"github.com/sammyklan3/finality/blockchain"
)

// This file is going to be responsible for generating random transactions
// and passing it into a channel for the client to consume

var (
	organizations = []string{
		"JPMorgan Chase & Co.",
		"Goldman Sachs",
		"Bank of America",
		"Citigroup",
		"HSBC Holdings",
	}
	names = []string{
		"Alice Johnson",
		"Bob Smith",
		"Charlie Williams",
		"Diana Evans",
		"Ethan Brown",
		"Fiona Davis",
		"George Miller",
		"Hannah Wilson",
		"Ian Clark",
		"Julia Lewis",
		"Kevin Hall",
		"Lara Young",
		"Michael King",
		"Nora Scott",
		"Oliver Adams",
		"Paula Turner",
		"Quentin Baker",
		"Rachel Harris",
		"Samuel Allen",
		"Tina Martin",
	}
)

func randomChoice[T any](arr []T) *T {
	if len(arr) == 0 {
		return nil
	}
	index := rand.IntN(len(arr))
	return &arr[index]
}

func generateTransaction() (*blockchain.Transaction, error) {
	sender := randomChoice(names)
	receiver := randomChoice(names)
	if sender == nil || receiver == nil {
		return nil, fmt.Errorf("Error selecting random sender or receiver; NIL values. Please fill in the names array with valid data")
	}

	amount := rand.UintN(math.MaxUint16)

	t, err := blockchain.NewTransaction(*sender, *receiver, amount)
	if err != nil {
		return nil, fmt.Errorf("Error creating new transaction; %v", err)
	}
	return t, nil
}

// Adds random blocks onto a blockchain
func generateBlocks(
	ctx context.Context,
	b *blockchain.Blockchain,
	signedBlocks chan<- blockchain.SignedBlock,
) {
	for {
		select {
		case err := <-ctx.Done():
			log.Println("Blocks generator cancelled due to: ", err)
			return

		default:
			// Sleep for some random time, to simulate transactions coming
			// in at different intervals
			randNumber := rand.IntN(10)
			time.Sleep(time.Duration(randNumber) * time.Second)

			t, err := generateTransaction()
			if err != nil {
				log.Printf("Error generating random transaction; %v\n", err)
				continue
			}

			err = b.AddTransaction(*t, signedBlocks)
			if err != nil {
				log.Printf("Error adding transaction to blockchain; %v\n", err)
				// TODO: Retry adding failed transactions to blockchain
				// For now we are just going to ignore failed transactions
				continue
			}
		}
	}
}
