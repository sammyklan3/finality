package blockchain

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/md5"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	mrand "math/rand/v2"
	"strings"
	"time"
)

const (
	MIN_TRANSACTION_AMOUNT uint = 1 // 1 unit
)

type Transaction struct {
	Id        uint      `json:"id"`
	Sender    string    `json:"sender"`
	Receiver  string    `json:"receiver"`
	Amount    uint      `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

func NewTransaction(sender, receiver string, amount uint) (*Transaction, error) {
	isEmpty := func(value string) bool {
		return strings.TrimSpace(value) == ""
	}
	if isEmpty(sender) || isEmpty(receiver) {
		return nil, fmt.Errorf("Missing transaction sender or receiver")
	}
	if amount < MIN_TRANSACTION_AMOUNT {
		return nil, fmt.Errorf("Transaction amount cannot be less than %v", MIN_TRANSACTION_AMOUNT)
	}

	t := Transaction{
		Id:        mrand.Uint(),
		Sender:    sender,
		Receiver:  receiver,
		Amount:    amount,
		Timestamp: time.Now(),
	}
	log.Printf("%s\n", t)

	return &t, nil
}

func (t Transaction) Valid() error {
	isEmpty := func(value string) bool {
		return strings.TrimSpace(value) == ""
	}
	if isEmpty(t.Sender) || isEmpty(t.Receiver) {
		return fmt.Errorf("Missing transaction sender or receiver")
	}
	if t.Amount < MIN_TRANSACTION_AMOUNT {
		return fmt.Errorf("Transaction amount cannot be less than %v", MIN_TRANSACTION_AMOUNT)
	}
	return nil
}

func (t Transaction) Sign(privateKey ecdsa.PrivateKey) (string, error) {
	buffer := bytes.NewBuffer([]byte{})
	err := gob.NewEncoder(buffer).Encode(t)
	if err != nil {
		return "", err
	}

	// Hash block
	data := buffer.Bytes()
	digest := md5.Sum(data)

	// Sign hashed data
	signature, err := privateKey.Sign(rand.Reader, digest[:], crypto.MD5)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(signature), nil
}

func (t Transaction) String() string {
	return fmt.Sprintf("Transaction<id=%v>\n\tsender=%v\n\treceiver=%v\n\tamount=%v\n\ttimestamp=%v>", t.Id, t.Sender, t.Receiver, t.Amount, t.Timestamp)
}
