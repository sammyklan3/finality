package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sammyklan3/finality/raft"
)

var (
	remote string
	self   *RaftServer

	// test flag
	is_leader   bool = false
	is_follower bool = false

	new_state      chan state  = make(chan state)
	new_peers_chan chan string = make(chan string)
)

func init() {
	flag.StringVar(&remote, "remote", "", "Address of the server to contact to join raft network")
	flag.BoolVar(&is_leader, "leader", false, "Start off server as leader")
	flag.BoolVar(&is_follower, "follower", false, "Start of server as follower")
	flag.Parse()

	if is_leader && is_follower {
		log.Fatalln("illogical argument; leaders can NOT also be followers")
	}

	host, port := NewAddress("localhost", 1024)
	address := fmt.Sprintf("%v:%v", host, port)
	self = NewRaftServer(address)
}

func launchRPCService() {
	err := rpc.Register(self)
	if err != nil {
		log.Fatalf("error registering raft service; %v\n", err)
	}

	listener, err := net.Listen("tcp", self.address)
	if err != nil {
		log.Fatalf("error starting raft server; %v\n", err)
	}
	defer listener.Close()

	log.Printf("started new raft server on %v\n", self.address)

	for {
		client, err := listener.Accept()
		if err != nil {
			log.Printf("error accepting client connection; %v\n", err)
			continue
		}

		// log.Printf("new client connection from %v\n", client.RemoteAddr())
		go rpc.ServeConn(client)
	}
}

func joinNetwork() ([]string, error) {
	if strings.TrimSpace(remote) == "" {
		return nil, fmt.Errorf("no remote provided")
	}

	client, err := rpc.Dial("tcp", remote)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	request := raft.JoinRequest{
		Address: self.address,
	}
	reply := raft.JoinReply{}

	log.Println("joining raft network")
	err = client.Call("RaftServer.JoinNetwork", &request, &reply)
	if err != nil {
		return nil, err
	}

	peers := reply.RaftServers
	peers = slices.DeleteFunc(peers, func(peer string) bool {
		return peer == self.address
	})

	self.mu.Lock()
	self.peers[remote] = 0
	for _, peer := range peers {
		self.peers[peer] = 0
	}
	self.mu.Unlock()

	go notifyPeersOfNewServer(peers)

	return peers, nil
}

func notifyPeersOfNewServer(peers []string) {
	self.mu.RLock()
	prev_log, _ := self.logEntries.Peek()
	self.mu.RUnlock()

	wg := sync.WaitGroup{}
	wg.Add(len(peers))

	for _, peer := range peers {
		go func(peer string) {
			defer wg.Done()

			log.Printf("notifying %v i joined the network\n", peer)
			client, err := rpc.Dial("tcp", peer)
			if err != nil {
				log.Printf("error dialing peer; %v\n", err)
				return
			}
			defer client.Close()

			request := raft.JoinRequest{
				Address:      self.address,
				PrevLogIndex: prev_log.Index,
			}
			reply := raft.JoinReply{}
			err = client.Call("RaftServer.JoinNetwork", &request, &reply)
			if err != nil {
				log.Printf("error calling RaftServer.JoinNetwork RPC func; %v\n", err)
				return
			}
		}(peer)
	}

	wg.Wait()
}

func requestVote(request raft.VoteRequest, address string, result_chan chan<- raft.VoteReply) {
	log.Printf("requesting %v for their vote 🗳️ on term %v\n", address, request.CandidatesTerm)

	client, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Printf("error requesting vote; %v\n", err)
		return
	}
	defer client.Close()

	vote_reply := raft.VoteReply{}
	err = client.Call("RaftServer.RequestVote", &request, &vote_reply)
	if err != nil {
		log.Printf("error requesting vote; %v\n", err)
		return
	}

	vote_granted := vote_reply.VoteGranted
	if vote_granted {
		log.Printf("%v granted you their voted\n", address)
	} else {
		log.Printf("%v denied you their voted\n", address)
	}

	result_chan <- vote_reply
}

func requestVotes(peers []string) (bool, error) {
	if is_follower {
		new_state <- follower
		return false, nil
	}

	if len(peers) == 0 {
		log.Println("requestVotes; waiting for peers to join network...")
		_ = <-new_peers_chan
	}

	// increment current term
	current_vote, err := self.incrementTerm()
	if err != nil {
		return false, fmt.Errorf("error incrementing current term; %v", err)
	}

	log.Printf("requesting votes for term %v\n", current_vote.Term)

	// change to candidate state
	self.mu.Lock()
	self.currentState = candidate
	my_prev_log, _ := self.logEntries.Peek()
	self.mu.Unlock()

	// vote for self
	my_vote := raft.Vote{
		Term:     current_vote.Term,
		VotedFor: self.address,
	}
	err = self.castVote(my_vote)
	if err != nil {
		return false, fmt.Errorf("error voting for self; %v", err)
	}

	vote_request := raft.VoteRequest{
		CandidatesTerm:   current_vote.Term,
		CandidateAddress: self.address,
		LastLog:          my_prev_log,
	}

	// send requestVote RPCs to all peer servers
	votes_chan := make(chan raft.VoteReply, len(peers))
	for _, peer := range peers {
		go requestVote(vote_request, peer, votes_chan)
	}

	// we also count our own vote
	votes_granted := 1
	half_peers := len(peers) / 2

	hasMajority := func() bool {
		if len(peers) < 2 {
			return votes_granted == 2
		}
		return votes_granted > half_peers
	}

	for {
		select {
		case vote_reply := <-votes_chan:
			// if voter has greater term, candidate MUST step down
			if vote_reply.Term > current_vote.Term {
				new_state <- follower
				return false, nil
			}

			if !vote_reply.VoteGranted {
				continue
			}

			votes_granted++
			if hasMajority() {
				return true, nil
			}

		case <-time.After(5 * time.Second):
			return hasMajority(), nil
		}
	}
}

func sendAppendEntry(peer string, new_log *raft.Log, results chan<- bool) {
	self.mu.RLock()
	current_term := self.currentTerm
	commit_index := self.commitIndex

	followers_prev_log_index, _ := self.peers[peer]
	my_prev_log, _ := self.logEntries.Peek()

	raft_logs := []raft.Log{}
	if followers_prev_log_index < my_prev_log.Index {
		raft_logs = self.logEntries.GetTopOf(followers_prev_log_index, commit_index)
	}
	self.mu.RUnlock()

	if new_log != nil {
		raft_logs = append(raft_logs, *new_log)
	}

	request := raft.AppendEntriesRequest{
		LeadersTerm:        current_term,
		LeadersAddress:     self.address,
		LeadersCommitIndex: commit_index,
		Entries:            raft_logs,
		PrevLogIndex:       followers_prev_log_index,
	}

	client, err := rpc.Dial("tcp", peer)
	if err != nil {
		fmt.Printf("o")
		return
	}
	defer client.Close()

	reply := raft.AppendEntriesReply{}
	err = client.Call("RaftServer.AppendEntries", &request, &reply)
	if err != nil {
		fmt.Printf("o")
		return
	}

	// if peer has higher term than us, we step down
	// from leader
	if reply.Term > current_term {
		new_state <- follower
		return
	}

	self.mu.Lock()
	self.peers[peer] = reply.PrevLogIndex
	self.mu.Unlock()

	results <- reply.Success
}

// Sends AppendEntry RPCs to peers
func sendAppendEntries(peers []string, new_log *raft.Log) (updated_majority bool) {
	defer func() {
		if updated_majority && new_log != nil {
			self.mu.Lock()
			self.commitIndex = new_log.Index
			self.mu.Unlock()
		}
	}()

	if len(peers) == 0 {
		log.Println("sendAppendEntries; waiting for peers to join network...")
		_ = <-new_peers_chan
	}

	fmt.Printf(".")
	results_chan := make(chan bool, len(peers))
	for _, peer := range peers {
		go sendAppendEntry(peer, new_log, results_chan)
	}

	// we also count ourselves
	peers_updated := 1
	half_peers := len(peers) / 2

	updatedMajority := func() bool {
		if len(peers) < 2 {
			return peers_updated == 2
		}
		return peers_updated > half_peers
	}

	for {
		select {
		case ok := <-results_chan:
			if !ok {
				continue
			}

			peers_updated++
			if updatedMajority() {
				return true
			}

		case <-time.After(5 * time.Second):
			return updatedMajority()
		}
	}
}

func startLeader(ctx context.Context) {
	timeout := getTimeout(leader)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	self.mu.Lock()
	self.leadersAddress = self.address
	self.mu.Unlock()

	log.Println("you were selected as leader! 🍻")

	// First mandate as a leader is to reset peers last
	// log index to my last log index
	self.resetPeersLastLog()

	for {
		select {
		case <-ctx.Done():
			// We are done being a leader
			log.Println("stepping down from leader")
			return

		case <-ticker.C:
			peers := self.getPeers()
			sendAppendEntries(peers, nil)
		}
	}
}

func startFollower(ctx context.Context) {
	timeout := getTimeout(follower)
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			self.mu.RLock()
			last_heartbeat := self.lastHeartbeat
			self.mu.RUnlock()

			leader_dead := time.Since(last_heartbeat) > timeout
			if leader_dead {
				log.Println("leader is dead")
				new_state <- candidate
			}
		}
	}
}

func startCandidate() {
	log.Println("becoming candidate")

	peers := self.getPeers()
	won_election, err := requestVotes(peers)
	if err != nil {
		log.Printf("error requesting votes; %v\n", err)
		return
	}

	if won_election {
		new_state <- leader
	} else {
		log.Println("election lost; stepping down from candidate")
		new_state <- follower
	}
}

func main() {
	go launchRPCService()

	_, err := joinNetwork()
	if err != nil {
		log.Printf("error joining raft network; %v\n", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Wait for RPC service to launch
		time.Sleep(1 * time.Second)

		// Start server as follower
		new_state <- follower
	}()

	for {
		new_state := <-new_state

		self.mu.Lock()
		self.currentState = new_state
		self.mu.Unlock()

		// Cancel old context and get a new one.
		// This stops the server from doing whatever it was doing and restarts
		// it with a new context
		cancel()
		ctx, cancel = context.WithCancel(context.Background())

		peers := self.getPeers()
		if len(peers) == 0 {
			log.Println("waiting for peers to join network...")
			<-new_peers_chan
		}

		switch new_state {
		case leader:
			go startLeader(ctx)
		case follower:
			go startFollower(ctx)
		default:
			// Start candidate is not a long running operation
			// so we don't pass any context to it
			go startCandidate()
		}
	}
}
