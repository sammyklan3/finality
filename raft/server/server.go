package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/rpc"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sammyklan3/finality/raft"
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

	votersFile := raft.GetVotersFile(address)
	currentVote := readVotersFile(votersFile)
	log.Printf("term=%v; votedFor=%v\n", currentVote.Term, currentVote.VotedFor)

	logFile := raft.GetLogFile(address)
	logEntries, err := NewLogEntries(logFile)
	if err != nil {
		log.Fatalf("error creating new log entries; %v\n", err)
	}

	return &RaftServer{
		mu: sync.RWMutex{},

		address:       address,
		currentState:  follower,
		lastHeartbeat: time.Now(),
		peers:         map[string]uint64{},
		logEntries:    logEntries,
		currentTerm:   currentVote.Term,
		votedFor:      currentVote.VotedFor,
	}
}

func (s *RaftServer) NewRequest(request *raft.Request, reply *raft.Reply) error {
	s.mu.RLock()
	currentState := s.currentState
	currentTerm := s.currentTerm
	leadersAddress := s.leadersAddress
	s.mu.RUnlock()

	if strings.TrimSpace(leadersAddress) == "" {
		return fmt.Errorf("leader NOT found")
	}

	if currentState != leader {
		return s.redirectToLeader(request, reply)
	}

	log.Println("received new client request")
	newLog := NewLog(request.Msg, currentTerm)
	peers := s.getPeers()

	updatedMajority := sendAppendEntries(peers, &newLog)
	if !updatedMajority {
		*reply = raft.Reply{
			Success: false,
			Result:  "failed to replicate log entry in majority of peers on network",
		}
		return nil
	}

	log.Println("log entry replicated in majority of peers on network")

	// Commit new log entry; make permanent
	s.mu.Lock()
	s.logEntries.Push(newLog)
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
		return fmt.Errorf("empty peer address")
	}

	if request.Address == s.address {
		return fmt.Errorf("hey wait a minute! i'm using this address")
	}

	peers := s.getPeers()
	if len(peers) == 0 {
		newPeersChan <- request.Address
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
	currentTerm := s.currentTerm
	votedFor := s.votedFor
	myPrevLog, _ := s.logEntries.Peek()
	s.mu.RUnlock()

	onSameTerm := request.CandidatesTerm == currentTerm
	alreadyVoted := votedFor != ""

	voteDenied := raft.VoteReply{
		Term:        currentTerm,
		VoteGranted: false,
	}

	if request.CandidatesTerm < currentTerm {
		log.Printf("vote denied for %v\n", request.CandidateAddress)
		*reply = voteDenied
		return nil
	}

	if onSameTerm && alreadyVoted {
		log.Printf("vote denied for %v\n", request.CandidateAddress)
		*reply = voteDenied
		return nil
	}

	// If candidates log is NOT as upto date as the voters log, decline vote
	if myPrevLog.Index > request.LastLog.Index || myPrevLog.Term > request.LastLog.Term {
		*reply = voteDenied
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

	voteGranted := raft.VoteReply{
		Term:        request.CandidatesTerm,
		VoteGranted: true,
	}
	*reply = voteGranted

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
		prevLog, _ := s.logEntries.Peek()
		*reply = raft.AppendEntriesReply{
			Term:         s.currentTerm,
			Success:      true,
			PrevLogIndex: prevLog.Index,
		}
		return nil
	}

	// We reject RPC if our log doesn't contain an entry at PrevLogIndex with PrevLogTerm
	found := s.logEntries.EntryExists(request.PrevLogIndex)
	if !found {
		log.Printf("follower failed to find prevLogIndex=%v\n", request.PrevLogIndex)
		prevLog, _ := s.logEntries.Peek()
		*reply = raft.AppendEntriesReply{
			Term:         s.currentTerm,
			Success:      false,
			PrevLogIndex: prevLog.Index,
		}
		return nil
	}

	entries := request.Entries
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Index < entries[j].Index
	})

	// Truncate the logs after prevLog, and append with leaders logs
	s.logEntries.Truncate(request.PrevLogIndex)
	s.logEntries.Push(entries...)
	s.logEntries.Commit()

	// If the leader's commitIndex is higher than ours, update our commitIndex to
	// match leaders
	if request.LeadersCommitIndex > s.commitIndex {
		s.commitIndex = request.LeadersCommitIndex
	}

	prevLog, _ := s.logEntries.Peek()
	*reply = raft.AppendEntriesReply{
		Term:         s.currentTerm,
		Success:      true,
		PrevLogIndex: prevLog.Index,
	}
	return nil
}

func (s *RaftServer) addPeers(peers ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.peers[peer] = 0
	}
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
	prevLog, _ := s.logEntries.Peek()
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, peer := range peers {
		s.peers[peer] = prevLog.Index
	}
}

// If we receive a request and we are not the leader, we
// redirect the request to the current leader
func (s *RaftServer) redirectToLeader(request *raft.Request, reply *raft.Reply) error {
	log.Println("forwarding client request to raft leader")

	s.mu.RLock()
	leadersAddress := s.leadersAddress
	s.mu.RUnlock()

	client, err := rpc.Dial("tcp", leadersAddress)
	if err != nil {
		return err
	}
	defer client.Close()

	return client.Call("RaftServer.NewRequest", request, reply)
}

func (s *RaftServer) castVote(vote raft.Vote) error {
	votersFile := raft.GetVotersFile(s.address)
	file, err := os.OpenFile(votersFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0700)
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
	currentTerm := s.currentTerm
	s.mu.RUnlock()

	vote := raft.Vote{
		Term:     currentTerm + 1,
		VotedFor: "",
	}
	err := s.castVote(vote)
	if err != nil {
		return nil, err
	}

	return &vote, nil
}

func readVotersFile(path string) raft.Vote {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0700)
	if err != nil {
		log.Fatalf("error reading voters file; %v\n", err)
	}
	defer file.Close()

	previousVote := raft.Vote{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&previousVote)
	if err != nil {
		// file empty
		if errors.Is(err, io.EOF) {
			encoder := json.NewEncoder(file)
			err = encoder.Encode(previousVote)
			if err != nil {
				log.Fatalf("error reading voters file; %v\n", err)
			}
		} else {
			log.Fatalf("error reading voters file; %v\n", err)
		}
	}

	return previousVote
}
