package main

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "finality/internal/core"
    "finality/internal/p2p"
)

func main() {
    // CLI flags
    port := flag.Int("port", 3000, "Port to run the node on")
    peer := flag.String("peer", "", "Address of a peer to connect to")
    difficulty := flag.Int("difficulty", 2, "Mining difficulty")
    mineInterval := flag.Int("mineInterval", 10, "Mining interval in seconds")
    flag.Parse()

    // Generate node's private key
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        log.Fatalf("Failed to generate node key: %v", err)
    }
    kp := &core.KeyPair{PrivateKey: privKey}

    // Initialize blockchain & mempool
    bc := core.NewBlockchain()
    mempool := core.NewMempool()

    // Initialize P2P node
    node := p2p.NewNode(*port, bc, mempool)
    if *peer != "" {
        go func() {
            if err := node.Connect(*peer); err != nil {
                log.Printf("Failed to connect to peer: %v", err)
            }
        }()
    }

    // Start P2P listener
    go func() {
        if err := node.Start(); err != nil {
            log.Fatalf("P2P node error: %v", err)
        }
    }()

    // Example: simulate adding a dummy transaction every 15s
    go func() {
        for {
            tx := core.NewTransaction("Alice", "Bob", 1, privKey)
            if mempool.AddTransaction(tx) {
                log.Printf("Transaction %s added to mempool", tx.Hash())
                node.BroadcastTransaction(tx) // send to peers
            }
            time.Sleep(15 * time.Second)
        }
    }()

    // Mining loop
    go func() {
        for {
            txs := mempool.GetTransactionsForBlock(100)
            if len(txs) > 0 {
                if err := bc.AddBlock(txs, kp, *difficulty); err != nil {
                    log.Printf("Mining error: %v", err)
                } else {
                    log.Printf("Mined new block with %d transactions", len(txs))
                    mempool.RemoveTransactions(txs)
                    node.BroadcastBlock(bc.LastBlock())
                }
            }
            time.Sleep(time.Duration(*mineInterval) * time.Second)
        }
    }()

    // Graceful shutdown handling
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
    <-sigs
    log.Println("Shutting down node...")
    node.Stop()
}
