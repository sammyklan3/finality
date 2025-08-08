package main

import (
    "flag"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
	"strings"

    "finality/internal/p2p"
)

func main() {
    // Command-line flags
    udpPort := flag.Int("udp-port", 30303, "UDP port for peer discovery")
    tcpPort := flag.Int("tcp-port", 30304, "TCP port for node connections")
    bootstrapPeers := flag.String("bootnodes", "", "Comma-separated list of bootstrap peers (ip:port)")
    nodeID := flag.String("node-id", "node123", "Unique node ID") // ideally generate properly later

    flag.Parse()

    // Parse bootstrap peers into a slice
    var bootnodes []string
    if *bootstrapPeers != "" {
        bootnodes = splitAndTrim(*bootstrapPeers)
    }

    // Start UDP discovery
    listenAddr := fmt.Sprintf("0.0.0.0:%d", *udpPort)
    discovery, err := p2p.NewDiscovery(*nodeID, listenAddr, bootnodes)
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
