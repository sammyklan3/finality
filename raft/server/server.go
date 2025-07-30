package main

import (
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net/rpc"
	"os"
	"path/filepath"
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
	serverId       string
	peers          *raft.Set[string]
	currentState   state
	lastHeartbeat  time.Time
	timeout        time.Duration
	leadersAddress string
	currentVote    raft.Vote
	logEntries     *raft.Stack[raft.Log]
}

func (s *RaftServer) RequestVote(request *raft.VoteRequest, reply *raft.VoteReply) error {
	log.Printf("candidate %v requesting for vote 🗳️ on term %v\n", request.CandidateId, request.CandidatesTerm)

	s.mu.RLock()
	current_term := s.currentVote.Term
	voted_for := s.currentVote.VotedFor
	s.mu.RUnlock()

	on_same_term := request.CandidatesTerm == current_term
	already_voted := voted_for != ""

	vote_denied := raft.VoteReply{
		Term:        current_term,
		VoteGranted: false,
	}

	if request.CandidatesTerm < current_term {
		log.Printf("vote denied for %v\n", request.CandidateId)
		*reply = vote_denied
		return nil
	}

	if on_same_term && already_voted {
		log.Printf("vote denied for %v\n", request.CandidateId)
		*reply = vote_denied
		return nil
	}

	// TODO: check if candidates log is at least up-to-date as receiver's log

	new_vote := raft.Vote{
		Term:     request.CandidatesTerm,
		VotedFor: request.CandidateId,
	}
	err := s.saveVote(new_vote)
	if err != nil {
		return fmt.Errorf("error saving new vote to file; %v", err)
	}

	s.mu.Lock()
	s.lastHeartbeat = time.Now()
	s.currentState = follower
	s.timeout = getTimeout(follower)
	s.mu.Unlock()

	vote_granted := raft.VoteReply{
		Term:        request.CandidatesTerm,
		VoteGranted: true,
	}
	*reply = vote_granted

	log.Printf("term: %v; votedFor: %v\n", request.CandidatesTerm, request.CandidateId)
	return nil
}

func (s *RaftServer) AppendEntries(request *raft.AppendEntriesRequest, reply *raft.AppendEntriesReply) error {
	log.Println("got appendEntries from leader")

	s.mu.Lock()
	s.lastHeartbeat = time.Now()
	s.leadersAddress = request.LeadersAddress
	current_term := s.currentVote.Term
	s.mu.Unlock()

	if request.LeadersTerm < current_term {
		*reply = raft.AppendEntriesReply{
			Term:    current_term,
			Success: false,
		}
		return nil
	}

	// TODO: Save log entries to disk

	*reply = raft.AppendEntriesReply{
		Term:    current_term,
		Success: true,
	}
	return nil
}

func (s *RaftServer) JoinNetwork(request *raft.JoinRequest, reply *raft.JoinReply) error {
	if strings.TrimSpace(request.Address) == "" {
		return fmt.Errorf("empty server address in join request")
	}

	log.Printf("%v joined network\n", request.Address)

	s.mu.RLock()
	peers := s.peers.GetItems()
	s.mu.RUnlock()

	if request.Address == s.address {
		// sender and receiver sharing the same address??
		return fmt.Errorf("invalid address in join request")
	}

	if len(peers) == 0 {
		go func() {
			broadcast <- request.Address
		}()
	}

	s.mu.Lock()
	s.peers.Add(request.Address)
	s.mu.Unlock()

	*reply = raft.JoinReply{
		RaftServers: peers,
	}
	return nil
}

func (s *RaftServer) NewRequest(request *raft.Request, reply *raft.Reply) error {
	s.mu.RLock()
	current_state := s.currentState
	current_term := s.currentVote.Term
	peers := s.peers.GetItems()
	s.mu.RUnlock()

	if current_state != leader {
		return s.redirectToLeader(request, reply)
	}

	log.Println("received new client request")

	s.mu.RLock()
	prev_log, err := s.logEntries.Peek()
	s.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("error acquiring previous log; %v", err)
	}

	new_log := raft.Log{
		Msg:  request.Msg,
		Term: current_term,
	}
	s.mu.Lock()
	err = s.logEntries.Push(new_log, true)
	s.mu.Unlock()

	if err != nil {
		return fmt.Errorf("error appending log entry; %v", err)
	}

	append_entries_request := raft.AppendEntriesRequest{
		LeadersTerm:    current_term,
		LeadersAddress: s.address,
		PrevLog:        prev_log,
		Entries:        []raft.Log{new_log},
		// TODO:
	}

	n_updated, err := sendAppendEntries(&append_entries_request, peers)
	if err != nil {
		return fmt.Errorf("error sending append entries; %v", err)
	}

	half_peers := len(peers) / 2
	updated_majority := n_updated > half_peers

	if !updated_majority {
		return fmt.Errorf("error updating majority of peers on network")
	}

	log.Println("log entry replicated in majority of peers on network")

	// TODO: commit the log entry; make permanent
	// TODO: apply commited entry to system state and notify client

	*reply = raft.Reply{
		Success: true,
		Result:  fmt.Sprintf("Msg %v executed successfully", request.Msg),
	}
	return nil
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

func (s *RaftServer) saveVote(vote raft.Vote) error {
	voters_file := filepath.Join(raft.ProjectDir, s.serverId, "raft.votes")

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
	s.currentVote = vote
	s.mu.Unlock()

	return nil
}

func (s *RaftServer) incrementTerm() (*raft.Vote, error) {
	s.mu.RLock()
	current_term := s.currentVote.Term
	s.mu.RUnlock()

	new_vote := raft.Vote{
		Term:     current_term + 1,
		VotedFor: "",
	}
	err := s.saveVote(new_vote)
	if err != nil {
		return nil, err
	}

	return &new_vote, nil
}

func NewRaftServer(address string) *RaftServer {
	server_id := fmt.Sprintf("%x", sha1.Sum([]byte(address)))

	// create raft server's working dir
	dir := filepath.Join(raft.ProjectDir, server_id)
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		log.Fatalf("error creating raft server's working dir; %v\n", err)
	}

	voters_file := filepath.Join(raft.ProjectDir, server_id, "raft.votes")
	current_vote := readVotersFile(voters_file)
	log.Printf("term: %v\n", current_vote.Term)

	log_file := filepath.Join(raft.ProjectDir, server_id, "log.entries")
	log_entries, err := newLogEntries(log_file)
	if err != nil {
		log.Fatalf("error creating new log entries; %v\n", err)
	}

	return &RaftServer{
		mu: sync.RWMutex{},

		address:        address,
		serverId:       server_id,
		currentState:   follower,
		lastHeartbeat:  time.Now(),
		timeout:        getTimeout(follower),
		leadersAddress: "", // leaders address is not usually initially known
		peers:          raft.NewSet[string](),
		currentVote:    current_vote,
		logEntries:     log_entries,
	}
}

func readVotersFile(fname string) raft.Vote {
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_RDWR, 0700)
	if err != nil {
		log.Fatalf("error reading voters file; %v\n", err)
	}
	defer file.Close()

	prev_vote := raft.Vote{}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&prev_vote)
	if err != nil && errors.Is(err, io.EOF) {
		// file empty

		encoder := json.NewEncoder(file)
		err = encoder.Encode(prev_vote)
		if err != nil {
			log.Fatalf("error reading voters file; %v\n", err)
		}
	}

	return prev_vote
}

func newLogEntries(fpath string) (*raft.Stack[raft.Log], error) {
	return raft.NewStack[raft.Log](1000, fpath)
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

	log.Printf("follower timeout set to %v\n", timeout)
	return timeout
}
