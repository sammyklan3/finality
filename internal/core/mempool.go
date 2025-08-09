package core

import "sync"

type Mempool struct {
    mu           sync.Mutex
    transactions map[string]Transaction
}

func NewMempool() *Mempool {
    return &Mempool{
        transactions: make(map[string]Transaction),
    }
}

func (mp *Mempool) AddTransaction(tx Transaction) bool {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    if _, exists := mp.transactions[tx.Hash()]; exists {
        return false
    }
    if !tx.Verify() {
        return false
    }

    mp.transactions[tx.Hash()] = tx
    return true
}

func (mp *Mempool) RemoveTransactions(txs []Transaction) {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    for _, tx := range txs {
        delete(mp.transactions, tx.Hash())
    }
}

func (mp *Mempool) AllTransactions() []Transaction {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    txs := make([]Transaction, 0, len(mp.transactions))
    for _, tx := range mp.transactions {
        txs = append(txs, tx)
    }
    return txs
}

func (mp *Mempool) Has(hash string) bool {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    _, exists := mp.transactions[hash]
    return exists
}

// GetTransactionsForBlock fetches up to `max` transactions for mining.
func (mp *Mempool) GetTransactionsForBlock(max int) []Transaction {
    mp.mu.Lock()
    defer mp.mu.Unlock()

    txs := make([]Transaction, 0, max)
    for _, tx := range mp.transactions {
        txs = append(txs, tx)
        if len(txs) >= max {
            break
        }
    }
    return txs
}
