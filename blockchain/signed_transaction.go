package blockchain

import "fmt"

type SignedTransaction struct {
	Transaction
	Signature string `json:"signature"`
}

func (t SignedTransaction) String() string {
	return fmt.Sprintf("SignedTransaction<%s, signature=%v>", t.Transaction, t.Signature)
}
