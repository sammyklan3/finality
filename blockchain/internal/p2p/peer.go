package p2p

import (
    "fmt"
    "log"
    "net"
    "sync"
)

type Peer struct {
    Conn   net.Conn
    NodeID string
    Address string

    sendLock sync.Mutex
    // Add more fields as needed: message queues, status, etc.
}

func NewPeer(conn net.Conn) *Peer {
    return &Peer{
        Conn: conn,
    }
}

func (p *Peer) Close() {
    p.Conn.Close()
}

func (p *Peer) Send(data []byte) error {
    p.sendLock.Lock()
    defer p.sendLock.Unlock()
    // Send length-prefixed messages
    length := uint32(len(data))
    lenBuf := []byte{
        byte(length >> 24),
        byte(length >> 16),
        byte(length >> 8),
        byte(length),
    }
    if _, err := p.Conn.Write(lenBuf); err != nil {
        return err
    }
    if _, err := p.Conn.Write(data); err != nil {
        return err
    }
    return nil
}

// Start reading from the peer connection (can spawn goroutine here)
func (p *Peer) ReadLoop() {
    for {
        lengthBuf := make([]byte, 4)
        if _, err := p.Conn.Read(lengthBuf); err != nil {
            log.Printf("Peer %s disconnected: %v", p.NodeID, err)
            return
        }
        length := (uint32(lengthBuf[0]) << 24) | (uint32(lengthBuf[1]) << 16) | (uint32(lengthBuf[2]) << 8) | uint32(lengthBuf[3])
        data := make([]byte, length)
        if _, err := p.Conn.Read(data); err != nil {
            log.Printf("Peer %s disconnected during read: %v", p.NodeID, err)
            return
        }

        // TODO: handle incoming message data here
        fmt.Printf("Received message from %s: %x\n", p.NodeID, data)
    }
}
