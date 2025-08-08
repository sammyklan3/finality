package p2p

import (
    "encoding/json"
    "fmt"
    "io"
    "net"
)

const ProtocolVersion = "finality/1.0"

type HandshakeMessage struct {
    NodeID          string `json:"node_id"`
    ProtocolVersion string `json:"protocol_version"`
    ChainID         string `json:"chain_id"`
}

func WriteHandshake(conn net.Conn, msg HandshakeMessage) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    // Write length-prefixed message
    length := uint32(len(data))
    lenBuf := []byte{
        byte(length >> 24),
        byte(length >> 16),
        byte(length >> 8),
        byte(length),
    }
    if _, err := conn.Write(lenBuf); err != nil {
        return err
    }
    if _, err := conn.Write(data); err != nil {
        return err
    }
    return nil
}

func ReadHandshake(conn net.Conn) (HandshakeMessage, error) {
    var msg HandshakeMessage
    lenBuf := make([]byte, 4)
    if _, err := io.ReadFull(conn, lenBuf); err != nil {
        return msg, err
    }
    length := (uint32(lenBuf[0]) << 24) | (uint32(lenBuf[1]) << 16) | (uint32(lenBuf[2]) << 8) | uint32(lenBuf[3])
    data := make([]byte, length)
    if _, err := io.ReadFull(conn, data); err != nil {
        return msg, err
    }
    if err := json.Unmarshal(data, &msg); err != nil {
        return msg, err
    }
    return msg, nil
}

// Validate handshake fields (protocol version match, chain ID, etc.)
func ValidateHandshake(localChainID string, msg HandshakeMessage) error {
    if msg.ProtocolVersion != ProtocolVersion {
        return fmt.Errorf("protocol version mismatch: got %s, want %s", msg.ProtocolVersion, ProtocolVersion)
    }
    if msg.ChainID != localChainID {
        return fmt.Errorf("chain ID mismatch: got %s, want %s", msg.ChainID, localChainID)
    }
    if msg.NodeID == "" {
        return fmt.Errorf("empty node ID in handshake")
    }
    return nil
}
