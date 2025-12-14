package p2p

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"finality/internal/core"
)

type Node struct {
	privKey *ecdsa.PrivateKey
	NodeID  string
	ChainID string

	listenAddr string
	listener   net.Listener

	peers      map[string]*Peer
	peersMutex sync.Mutex

	Blockchain *core.Blockchain
	Mempool    *core.Mempool
}

func NewNode(privKey *ecdsa.PrivateKey, chainID, listenAddr string, bc *core.Blockchain, mp *core.Mempool) *Node {
	pubKeyBytes := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)
	nodeID := fmt.Sprintf("%x", pubKeyBytes) // TODO: Replace with shorter, secure ID

	return &Node{
		privKey:    privKey,
		NodeID:     nodeID,
		ChainID:    chainID,
		listenAddr: listenAddr,
		peers:      make(map[string]*Peer),
		Blockchain: bc,
		Mempool:    mp,
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
	defer conn.Close()

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

	// Listen for incoming gossip messages
	go n.listenPeerMessages(peer)

	peer.ReadLoop()

	// Remove peer on disconnect
	n.peersMutex.Lock()
	delete(n.peers, peerNodeID)
	n.peersMutex.Unlock()
}

// GossipTransaction sends a transaction to all connected peers
func (n *Node) GossipTransaction(tx core.Transaction) {
	msg := MessageType{
		Type: "tx",
		Data: mustJSON(tx),
	}
	n.broadcast(msg)
}

// GossipBlock sends a block to all connected peers
func (n *Node) GossipBlock(block core.Block) {
	msg := Message{
		Type: "block",
		Data: mustJSON(block),
	}
	n.broadcast(msg)
}

func (n *Node) broadcast(msg Message) {
	n.peersMutex.Lock()
	defer n.peersMutex.Unlock()
	for _, peer := range n.peers {
		peer.Send(msg)
	}
}

func (n *Node) listenPeerMessages(peer *Peer) {
	for {
		msg, ok := peer.ReadMessage()
		if !ok {
			return
		}

		switch msg.Type {
		case "tx":
			var tx core.Transaction
			if err := json.Unmarshal(msg.Data, &tx); err == nil {
				if n.Mempool.AddTransaction(tx) {
					log.Printf("TX %s added to mempool from peer %s", tx.Hash(), peer.NodeID)
					n.GossipTransaction(tx) // forward gossip
				}
			}
		case "block":
			var block core.Block
			if err := json.Unmarshal(msg.Data, &block); err == nil {
				if !n.Blockchain.HasBlock(block.Hash()) {
					err := n.Blockchain.AddBlock(block.Transactions, nil, n.Blockchain.Difficulty())
					if err == nil {
						log.Printf("Block %s added from peer %s", block.Hash(), peer.NodeID)
						n.Mempool.RemoveTransactions(block.Transactions)
						n.GossipBlock(block) // forward gossip
					}
				}
			}
		}
	}
}

func mustJSON(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// runHandshake performs the TCP handshake
func (n *Node) runHandshake(conn net.Conn, outbound bool) (string, error) {
	localMsg := HandshakeMessage{
		NodeID:          n.NodeID,
		ProtocolVersion: ProtocolVersion,
		ChainID:         n.ChainID,
	}

	if outbound {
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
