package p2p

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
)

// MessageType represents the kind of P2P message
type MessageType byte

const (
	MsgNewTx     MessageType = 0x01 // Gossip that a new tx hash is available
	MsgGetTx     MessageType = 0x02 // Request full tx by hash
	MsgTx        MessageType = 0x03 // Full transaction data
	MsgNewBlock  MessageType = 0x04 // Gossip that a new block hash is available
	MsgGetBlock  MessageType = 0x05 // Request full block by hash
	MsgBlock     MessageType = 0x06 // Full block data
)

// P2PMessage is the basic wire format
type P2PMessage struct {
	Type    MessageType `json:"type"`
	Payload []byte      `json:"payload"`
}

// TxMessage represents the payload for tx-related messages
type TxMessage struct {
	Hash string `json:"hash,omitempty"`
	Tx   []byte `json:"tx,omitempty"` // Raw encoded tx
}

// BlockMessage represents the payload for block-related messages
type BlockMessage struct {
	Hash  string `json:"hash,omitempty"`
	Block []byte `json:"block,omitempty"` // Raw encoded block
}

// EncodeMessage converts a P2PMessage to bytes with a length prefix
func EncodeMessage(msg P2PMessage) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, uint32(len(data))); err != nil {
		return nil, err
	}
	if _, err := buf.Write(data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DecodeMessage reads and decodes a message from a TCP stream
func DecodeMessage(r io.Reader) (P2PMessage, error) {
	var msg P2PMessage
	var length uint32

	if err := binary.Read(r, binary.BigEndian, &length); err != nil {
		return msg, err
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return msg, err
	}
	err := json.Unmarshal(data, &msg)
	return msg, err
}

// EncodeTxPayload helper for transaction-related messages
func EncodeTxPayload(hash string, txData []byte) ([]byte, error) {
	return json.Marshal(TxMessage{
		Hash: hash,
		Tx:   txData,
	})
}

// DecodeTxPayload helper for transaction-related messages
func DecodeTxPayload(data []byte) (TxMessage, error) {
	var tm TxMessage
	err := json.Unmarshal(data, &tm)
	return tm, err
}

// EncodeBlockPayload helper for block-related messages
func EncodeBlockPayload(hash string, blockData []byte) ([]byte, error) {
	return json.Marshal(BlockMessage{
		Hash:  hash,
		Block: blockData,
	})
}

// DecodeBlockPayload helper for block-related messages
func DecodeBlockPayload(data []byte) (BlockMessage, error) {
	var bm BlockMessage
	err := json.Unmarshal(data, &bm)
	return bm, err
}
