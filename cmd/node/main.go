package main

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "finality/internal/p2p"
)

func main() {
    // Command-line flags
    udpPort := flag.Int("udp-port", 30303, "UDP port for peer discovery")
    tcpPort := flag.Int("tcp-port", 30304, "TCP port for node connections")
    bootstrapPeers := flag.String("bootnodes", "", "Comma-separated list of bootstrap peers (ip:port)")
    flag.Parse()

    // Generate ECDSA private key for the node
    privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        log.Fatalf("Failed to generate node key: %v", err)
    }

    // Derive node ID as sha256(pubkeyBytes)
    pubKeyBytes := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
    nodeID := sha256.Sum256(pubKeyBytes)
    nodeIDHex := hex.EncodeToString(nodeID[:])

    fmt.Printf("Node ID: %s\n", nodeIDHex)

    // Parse bootstrap peers into a slice
    var bootnodes []string
    if *bootstrapPeers != "" {
        bootnodes = splitAndTrim(*bootstrapPeers)
    }

    // Start UDP discovery with generated NodeID
    listenAddr := fmt.Sprintf("0.0.0.0:%d", *udpPort)
    discovery, err := p2p.NewDiscovery(nodeIDHex, listenAddr, bootnodes)
    if err != nil {
        log.Fatalf("Failed to start discovery: %v", err)
    }
    discovery.Start()
    defer discovery.Stop()

    fmt.Printf("Finality node started.\nUDP discovery listening on %s\nTCP connections will listen on 0.0.0.0:%d\n", listenAddr, *tcpPort)

    // TODO: Start TCP server on tcpPort, consensus, etc.

    // Graceful shutdown on Ctrl+C
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
    <-sigs
    fmt.Println("\nShutting down node...")
}

// Helper to split comma-separated bootstrap peers and trim spaces
func splitAndTrim(s string) []string {
    var res []string
    for _, p := range strings.Split(s, ",") {
        res = append(res, strings.TrimSpace(p))
    }
    return res
}
