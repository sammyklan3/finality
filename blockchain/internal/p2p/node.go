package p2p

import (
    "crypto/ecdsa"
    "fmt"
    "log"
    "net"
    "sync"
    "time"
)

type Node struct {
    privKey   *ecdsa.PrivateKey
    NodeID    string
    ChainID   string

    listenAddr string
    listener   net.Listener

    peers      map[string]*Peer
    peersMutex sync.Mutex
}

func NewNode(privKey *ecdsa.PrivateKey, chainID, listenAddr string) *Node {
    pubKeyBytes := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
    nodeID := fmt.Sprintf("%x", pubKeyBytes) // Simple hex encoding, improve if needed

    return &Node{
        privKey:    privKey,
        NodeID:     nodeID,
        ChainID:    chainID,
        listenAddr: listenAddr,
        peers:      make(map[string]*Peer),
    }
}

// Start TCP server to accept inbound connections
func (n *Node) Start() error {
    ln, err := net.Listen("tcp", n.listenAddr)
    if err != nil {
        return err
    }
    n.listener = ln
    log.Printf("TCP server listening on %s", n.listenAddr)

    go n.acceptLoop()
    return nil
}

func (n *Node) acceptLoop() {
    for {
        conn, err := n.listener.Accept()
        if err != nil {
            log.Printf("Listener accept error: %v", err)
            continue
        }
        go n.handleConnection(conn, false)
    }
}

// Connect to a peer (outbound)
func (n *Node) Connect(addr string) error {
    conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
    if err != nil {
        return err
    }
    go n.handleConnection(conn, true)
    return nil
}

// Handle a new TCP connection (inbound or outbound)
func (n *Node) handleConnection(conn net.Conn, outbound bool) {
    defer func() {
        conn.Close()
    }()

    // Run handshake
    peerNodeID, err := n.runHandshake(conn, outbound)
    if err != nil {
        log.Printf("Handshake failed with %s: %v", conn.RemoteAddr(), err)
        return
    }
    log.Printf("Handshake successful with peer %s", peerNodeID)

    peer := NewPeer(conn)
    peer.NodeID = peerNodeID

    n.peersMutex.Lock()
    n.peers[peerNodeID] = peer
    n.peersMutex.Unlock()

    peer.ReadLoop()

    // Remove peer on disconnect
    n.peersMutex.Lock()
    delete(n.peers, peerNodeID)
    n.peersMutex.Unlock()
}

// runHandshake performs the TCP handshake
func (n *Node) runHandshake(conn net.Conn, outbound bool) (string, error) {
    localMsg := HandshakeMessage{
        NodeID:          n.NodeID,
        ProtocolVersion: ProtocolVersion,
        ChainID:         n.ChainID,
    }

    if outbound {
        // Outbound: send first, then read
        if err := WriteHandshake(conn, localMsg); err != nil {
            return "", err
        }
        remoteMsg, err := ReadHandshake(conn)
        if err != nil {
            return "", err
        }
        if err := ValidateHandshake(n.ChainID, remoteMsg); err != nil {
            return "", err
        }
        return remoteMsg.NodeID, nil
    } else {
        // Inbound: read first, then send
        remoteMsg, err := ReadHandshake(conn)
        if err != nil {
            return "", err
        }
        if err := ValidateHandshake(n.ChainID, remoteMsg); err != nil {
            return "", err
        }
        if err := WriteHandshake(conn, localMsg); err != nil {
            return "", err
        }
        return remoteMsg.NodeID, nil
    }
}

// Disconnect peer by nodeID
func (n *Node) Disconnect(peerID string) {
    n.peersMutex.Lock()
    defer n.peersMutex.Unlock()
    if peer, ok := n.peers[peerID]; ok {
        peer.Close()
        delete(n.peers, peerID)
    }
}
