package p2p

import (
    "encoding/json"
    "log"
    "math/rand"
    "net"
    "sync"
    "time"
)

type Packet struct {
    Type       string   `json:"type"`
    NodeID     string   `json:"node_id"`
    Address    string   `json:"address"`
    KnownPeers []string `json:"known_peers"`
}

type Discovery struct {
    nodeID    string
    udpAddr   *net.UDPAddr
    conn      *net.UDPConn
    peers     map[string]Peer
    peersLock sync.Mutex
    bootstrap []string
    stopCh    chan struct{}
}

// NewDiscovery creates discovery instance
func NewDiscovery(nodeID, listenAddr string, bootstrap []string) (*Discovery, error) {
    udpAddr, err := net.ResolveUDPAddr("udp", listenAddr)
    if err != nil {
        return nil, err
    }
    conn, err := net.ListenUDP("udp", udpAddr)
    if err != nil {
        return nil, err
    }
    return &Discovery{
        nodeID:    nodeID,
        udpAddr:   udpAddr,
        conn:      conn,
        peers:     make(map[string]Peer),
        bootstrap: bootstrap,
        stopCh:    make(chan struct{}),
    }, nil
}

// Start launches listener & bootstrap routine
func (d *Discovery) Start() {
    go d.listen()
    go d.bootstrapPeers()
    go d.periodicPing()
}

// Stop stops discovery
func (d *Discovery) Stop() {
    close(d.stopCh)
    d.conn.Close()
}

// listen for incoming UDP packets
func (d *Discovery) listen() {
    buf := make([]byte, 2048)
    for {
        select {
        case <-d.stopCh:
            return
        default:
            d.conn.SetReadDeadline(time.Now().Add(time.Second * 2))
            n, addr, err := d.conn.ReadFromUDP(buf)
            if err != nil {
                if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
                    continue // no packet in time
                }
                log.Printf("UDP read error: %v", err)
                continue
            }
            go d.handlePacket(buf[:n], addr)
        }
    }
}

// handlePacket processes ping/pong messages
func (d *Discovery) handlePacket(data []byte, addr *net.UDPAddr) {
    var pkt Packet
    err := json.Unmarshal(data, &pkt)
    if err != nil {
        log.Printf("Invalid packet from %v: %v", addr, err)
        return
    }
    switch pkt.Type {
    case "ping":
        d.handlePing(pkt, addr)
    case "pong":
        d.handlePong(pkt)
    default:
        log.Printf("Unknown packet type %s from %v", pkt.Type, addr)
    }
}

func (d *Discovery) handlePing(pkt Packet, addr *net.UDPAddr) {
    d.peersLock.Lock()
    defer d.peersLock.Unlock()

    _, exists := d.peers[pkt.NodeID]
    if !exists {
        log.Printf("[discovery] New peer discovered via PING: %s @ %s", pkt.NodeID, pkt.Address)
    }

    d.peers[pkt.NodeID] = Peer{NodeID: pkt.NodeID, Address: pkt.Address}

    // Send pong response
    var known []string
    for _, p := range d.peers {
        known = append(known, p.Address)
    }

    pong := Packet{
        Type:       "pong",
        NodeID:     d.nodeID,
        Address:    d.udpAddr.String(),
        KnownPeers: known,
    }
    d.sendPacket(pong, addr)
}

func (d *Discovery) handlePong(pkt Packet) {
    d.peersLock.Lock()
    defer d.peersLock.Unlock()

    _, exists := d.peers[pkt.NodeID]
    if !exists {
        log.Printf("[discovery] New peer discovered via PONG: %s @ %s", pkt.NodeID, pkt.Address)
    }

    d.peers[pkt.NodeID] = Peer{NodeID: pkt.NodeID, Address: pkt.Address}

    for _, addr := range pkt.KnownPeers {
        if addr == d.udpAddr.String() {
            continue
        }
        // We don’t know NodeID here, so use address as key to avoid duplicates
        if _, exists := d.peers[addr]; !exists {
            log.Printf("[discovery] Adding peer from known peers list: %s", addr)
            d.peers[addr] = Peer{Address: addr}
        }
    }
}

// sendPacket encodes & sends UDP packet
func (d *Discovery) sendPacket(pkt Packet, addr *net.UDPAddr) {
    data, err := json.Marshal(pkt)
    if err != nil {
        log.Printf("Failed to marshal packet: %v", err)
        return
    }
    _, err = d.conn.WriteToUDP(data, addr)
    if err != nil {
        log.Printf("Failed to send packet to %v: %v", addr, err)
    }
}

// bootstrapPeers sends initial ping to bootstrap nodes
func (d *Discovery) bootstrapPeers() {
    for _, addrStr := range d.bootstrap {
        udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
        if err != nil {
            log.Printf("Invalid bootstrap address %s: %v", addrStr, err)
            continue
        }
        d.sendPing(udpAddr)
    }
}

// sendPing sends ping packet to a peer
func (d *Discovery) sendPing(addr *net.UDPAddr) {
    ping := Packet{
        Type:       "ping",
        NodeID:     d.nodeID,
        Address:    d.udpAddr.String(),
        KnownPeers: nil,
    }
    d.sendPacket(ping, addr)
}

// periodicPing pings random peers to keep connections fresh
func (d *Discovery) periodicPing() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    for {
        select {
        case <-d.stopCh:
            return
        case <-ticker.C:
            d.peersLock.Lock()
            peerAddrs := []string{}
            for _, p := range d.peers {
                peerAddrs = append(peerAddrs, p.Address)
            }
            d.peersLock.Unlock()

            if len(peerAddrs) == 0 {
                continue
            }
            // Ping a random peer
            addrStr := peerAddrs[rand.Intn(len(peerAddrs))]
            udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
            if err != nil {
                continue
            }
            d.sendPing(udpAddr)
        }
    }
}

// ListPeers returns a snapshot of known peers
func (d *Discovery) ListPeers() []Peer {
    d.peersLock.Lock()
    defer d.peersLock.Unlock()
    peers := make([]Peer, 0, len(d.peers))
    for _, p := range d.peers {
        peers = append(peers, p)
    }
    return peers
}