package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sammyklan3/finality/raft"
)

type state string

var (
	leader    state = "leader"
	follower  state = "follower"
	candidate state = "candidate"
)

var (
	BASE_TIMEOUT   time.Duration = 20 * time.Second
	LEADER_TIMEOUT time.Duration = 10 * time.Second
)

// create RPC service
type RaftServer struct {
	mu sync.RWMutex

	address        string
	peers          map[string]uint64 // map of peer addresses and their previous log index
	currentState   state
	lastHeartbeat  time.Time
	leadersAddress string
	commitIndex    uint64
	currentTerm    uint64
	votedFor       string
	logEntries     Stack
}

func NewRaftServer(address string) *RaftServer {
	dir := filepath.Join(raft.ProjectDir, address)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		log.Fatalf("error creating raft server's working dir; %v\n", err)
	}

	voters_file := filepath.Join(raft.ProjectDir, address, "raft.votes")
	current_vote := readVotersFile(voters_file)
	log.Printf("term=%v; votedFor=%v\n", current_vote.Term, current_vote.VotedFor)

	log_file := filepath.Join(raft.ProjectDir, address, "raft.logs")
	log_entries, err := NewLogEntries(log_file)
	if err != nil {
		log.Fatalf("error creating new log entries; %v\n", err)
	}

	return &RaftServer{
		mu: sync.RWMutex{},

		address:       address,
		currentState:  follower,
		lastHeartbeat: time.Now(),
		peers:         map[string]uint64{},
		logEntries:    log_entries,
		currentTerm:   current_vote.Term,
		votedFor:      current_vote.VotedFor,
	}
}

func (s *RaftServer) NewRequest(request *raft.Request, reply *raft.Reply) error {
	s.mu.RLock()
	current_state := s.currentState
	current_term := s.currentTerm
	leaders_address := s.leadersAddress
	s.mu.RUnlock()

	if strings.TrimSpace(leaders_address) == "" {
		return fmt.Errorf("leader NOT found")
	}

	if current_state != leader {
		return s.redirectToLeader(request, reply)
	}

	log.Println("received new client request")
	new_log := newLog(request.Msg, current_term)
	peers := s.getPeers()

	updated_majority := sendAppendEntries(peers, &new_log)
	if !updated_majority {
		*reply = raft.Reply{
			Success: false,
			Result:  "failed to replicate log entry in majority of peers on network",
		}
		return nil
	}

	log.Println("log entry replicated in majority of peers on network")

	// Commit new log entry; make permanent
	s.mu.Lock()
	s.logEntries.Push(new_log)
	err := s.logEntries.Commit()
	s.mu.Unlock()

	if err != nil {
		*reply = raft.Reply{
			Success: false,
			Result:  fmt.Sprintf("error committing msg %#v", request.Msg),
		}
		return nil
	}

	*reply = raft.Reply{
		Success: true,
		Result:  fmt.Sprintf("msg %#v committed successfully", request.Msg),
	}
	return nil
}

func (s *RaftServer) JoinNetwork(request *raft.JoinRequest, reply *raft.JoinReply) error {
	if strings.TrimSpace(request.Address) == "" {
		return fmt.Errorf("empty server address in join request")
	}

	if request.Address == s.address {
		return fmt.Errorf("hey wait a minute! i'm using this address")
	}

	peers := s.getPeers()
	if len(peers) == 0 {
		new_peers_chan <- request.Address
	}

	s.mu.Lock()
	s.peers[request.Address] = request.PrevLogIndex
	s.mu.Unlock()
	log.Printf("%v joined network\n", request.Address)

	*reply = raft.JoinReply{
		RaftServers: peers,
	}
	return nil
}

func (s *RaftServer) RequestVote(request *raft.VoteRequest, reply *raft.VoteReply) error {
	log.Printf("candidate %v requesting for vote 🗳️ on term %v\n", request.CandidateAddress, request.CandidatesTerm)

	s.mu.RLock()
	current_term := s.currentTerm
	voted_for := s.votedFor
	my_prev_log, _ := s.logEntries.Peek()
	s.mu.RUnlock()

	on_same_term := request.CandidatesTerm == current_term
	already_voted := voted_for != ""

	vote_denied := raft.VoteReply{
		Term:        current_term,
		VoteGranted: false,
	}

	if request.CandidatesTerm < current_term {
		log.Printf("vote denied for %v\n", request.CandidateAddress)
		*reply = vote_denied
		return nil
	}

	if on_same_term && already_voted {
		log.Printf("vote denied for %v\n", request.CandidateAddress)
		*reply = vote_denied
		return nil
	}

	// If candidates log is NOT as upto date as the voters log, decline vote
	if my_prev_log.Index > request.LastLog.Index || my_prev_log.Term > request.LastLog.Term {
		*reply = vote_denied
		return nil
	}

	vote := raft.Vote{
		Term:     request.CandidatesTerm,
		VotedFor: request.CandidateAddress,
	}
	err := s.castVote(vote)
	if err != nil {
		return fmt.Errorf("error saving new vote to file; %v", err)
	}

	s.mu.Lock()
	s.lastHeartbeat = time.Now()
	s.currentState = follower
	s.mu.Unlock()

	vote_granted := raft.VoteReply{
		Term:        request.CandidatesTerm,
		VoteGranted: true,
	}
	*reply = vote_granted

	log.Printf("term=%v; votedFor=%v\n", request.CandidatesTerm, request.CandidateAddress)
	return nil
}

func (s *RaftServer) AppendEntries(request *raft.AppendEntriesRequest, reply *raft.AppendEntriesReply) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.leadersAddress = request.LeadersAddress

	// If leader has higher term, we update current term to that of the leader
	if request.LeadersTerm > s.currentTerm {
		s.currentTerm = request.LeadersTerm
		s.votedFor = ""
		s.currentState = follower
	}

	// Reject RPC if the leader's term is less than the follower's current term
	if request.LeadersTerm < s.currentTerm {
		log.Printf("AppendEntry RPC rejected; leader on lower term=%v than follower=%v\n", request.LeadersTerm, s.currentTerm)
		*reply = raft.AppendEntriesReply{
			Term:    s.currentTerm,
			Success: false,
		}
		return nil
	}

	// A valid RPC from a leader means we reset the election timer
	s.lastHeartbeat = time.Now()

	// This is just a heartbeat RPC
	if len(request.Entries) == 0 {
		fmt.Printf("x")
		prev_log, _ := s.logEntries.Peek()
		*reply = raft.AppendEntriesReply{
			Term:         s.currentTerm,
			Success:      true,
			PrevLogIndex: prev_log.Index,
		}
		return nil
	}

	// We reject RPC if our log doesn't contain an entry at PrevLogIndex with PrevLogTerm
	found := s.logEntries.EntryExists(request.PrevLogIndex)
	if !found {
		log.Printf("follower failed to find prevLogIndex=%v\n", request.PrevLogIndex)
		prev_log, _ := s.logEntries.Peek()
		*reply = raft.AppendEntriesReply{
			Term:         s.currentTerm,
			Success:      false,
			PrevLogIndex: prev_log.Index,
		}
		return nil
	}

	entries := request.Entries
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Index < entries[j].Index
	})

	// Truncate the logs after prev_log, and append with leaders logs
	s.logEntries.Truncate(request.PrevLogIndex)
	s.logEntries.Push(entries...)
	s.logEntries.Commit()

	// If the leader's commitIndex is higher than ours, update our commitIndex to
	// match leaders
	if request.LeadersCommitIndex > s.commitIndex {
		s.commitIndex = request.LeadersCommitIndex
	}

	prev_log, _ := s.logEntries.Peek()
	*reply = raft.AppendEntriesReply{
		Term:         s.currentTerm,
		Success:      true,
		PrevLogIndex: prev_log.Index,
	}
	return nil
}

func (s *RaftServer) getPeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	peers := []string{}
	for peer := range s.peers {
		peers = append(peers, peer)
	}
	return peers
}

func (s *RaftServer) resetPeersLastLog() {
	s.mu.RLock()
	peers := s.getPeers()
	prev_log, _ := s.logEntries.Peek()
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.peers[peer] = prev_log.Index
	}
}

// If we receive a request and we are not the leader, we
// redirect the request to the current leader
func (s *RaftServer) redirectToLeader(request *raft.Request, reply *raft.Reply) error {
	log.Println("forwarding client request to raft leader")

	s.mu.RLock()
	leaders_address := s.leadersAddress
	s.mu.RUnlock()

	client, err := rpc.Dial("tcp", leaders_address)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Call("RaftServer.NewRequest", request, reply)
}

func (s *RaftServer) castVote(vote raft.Vote) error {
	voters_file := filepath.Join(raft.ProjectDir, s.address, "raft.votes")
	file, err := os.OpenFile(voters_file, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	err = encoder.Encode(&vote)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.currentTerm = vote.Term
	s.votedFor = vote.VotedFor
	s.mu.Unlock()

	return nil
}

func (s *RaftServer) incrementTerm() (*raft.Vote, error) {
	s.mu.RLock()
	current_term := s.currentTerm
	s.mu.RUnlock()

	new_vote := raft.Vote{
		Term:     current_term + 1,
		VotedFor: "",
	}
	err := s.castVote(new_vote)
	if err != nil {
		return nil, err
	}

	return &new_vote, nil
}

func readVotersFile(path string) raft.Vote {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0700)
	if err != nil {
		log.Fatalf("error reading voters file; %v\n", err)
	}
	defer file.Close()

	prev_vote := raft.Vote{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&prev_vote)
	if err != nil {
		// file empty
		if errors.Is(err, io.EOF) {
			encoder := json.NewEncoder(file)
			err = encoder.Encode(prev_vote)
			if err != nil {
				log.Fatalf("error reading voters file; %v\n", err)
			}
		} else {
			log.Fatalf("error reading voters file; %v\n", err)
		}
	}

	return prev_vote
}

func getTimeout(current_state state) time.Duration {
	if current_state == leader {
		timeout := LEADER_TIMEOUT
		log.Printf("leader timeout set to %v\n", timeout)
		return timeout
	}

	// randomized timeout variations prevent more than one
	// follower requesting for votes at around the same time.
	// 1 + randN avoids zero variation value.

	rand_int := 1 + rand.IntN(5)
	variation := time.Duration(rand_int) * time.Second
	timeout := BASE_TIMEOUT + variation

	log.Printf("%s timeout set to %v\n", current_state, timeout)
	return timeout
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}
