package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/rpc"
	"slices"
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
	remote string
	self   *RaftServer

	// test flag
	isLeader   bool = false
	isFollower bool = false

	newState     chan state  = make(chan state)
	newPeersChan chan string = make(chan string)
)

func init() {
	flag.StringVar(&remote, "remote", "", "Address of the server to contact to join raft network")
	flag.BoolVar(&isLeader, "leader", false, "Start off server as leader")
	flag.BoolVar(&isFollower, "follower", false, "Start of server as follower")
	flag.Parse()

	if isLeader && isFollower {
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

func joinNetwork(address string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("empty peer address")
	}

	client, err := rpc.Dial("tcp", address)
	if err != nil {
		return err
	}
	defer client.Close()

	self.mu.RLock()
	prevLog, _ := self.logEntries.Peek()
	self.mu.RUnlock()

	request := raft.JoinRequest{
		Address:      self.address,
		PrevLogIndex: prevLog.Index,
	}
	reply := raft.JoinReply{}

	log.Printf("sending JoinNetwork request to %v\n", address)
	err = client.Call("RaftServer.JoinNetwork", &request, &reply)
	if err != nil {
		return err
	}

	peers := reply.RaftServers
	peers = slices.DeleteFunc(peers, func(peer string) bool {
		return peer == self.address || peer == address
	})
	self.addPeers(peers...)

	return nil
}

func notifyPeersOfJoin(peers []string) {
	wg := sync.WaitGroup{}
	wg.Add(len(peers))

	for _, peer := range peers {
		go func(address string) {
			defer wg.Done()

			err := joinNetwork(address)
			if err != nil {
				log.Printf("error joining network through %v; %v\n", address, err)
			}
		}(peer)
	}

	wg.Wait()
}

func requestVote(request raft.VoteRequest, address string, resultChan chan<- raft.VoteReply) {
	log.Printf("requesting %v for their vote 🗳️, term=%v\n", address, request.CandidatesTerm)

	client, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Printf("error requesting vote; %v\n", err)
		return
	}
	defer client.Close()

	voteReply := raft.VoteReply{}
	err = client.Call("RaftServer.RequestVote", &request, &voteReply)
	if err != nil {
		log.Printf("error requesting vote; %v\n", err)
		return
	}

	voteGranted := voteReply.VoteGranted
	if voteGranted {
		log.Printf("%v granted you their vote\n", address)
	} else {
		log.Printf("%v denied you their vote\n", address)
	}

	resultChan <- voteReply
}

func requestVotes(peers []string) (bool, error) {
	if isFollower {
		newState <- follower
		return false, nil
	}

	if len(peers) == 0 {
		log.Println("requestVotes; waiting for peers to join network...")
		_ = <-newPeersChan
	}

	currentVote, err := self.incrementTerm()
	if err != nil {
		return false, fmt.Errorf("error incrementing current term; %v", err)
	}

	log.Printf("requesting votes for term %v\n", currentVote.Term)

	self.mu.Lock()
	self.currentState = candidate
	myPrevLog, _ := self.logEntries.Peek()
	self.mu.Unlock()

	// Vote for self
	myVote := raft.Vote{
		Term:     currentVote.Term,
		VotedFor: self.address,
	}
	err = self.castVote(myVote)
	if err != nil {
		return false, fmt.Errorf("error voting for self; %v", err)
	}

	voteRequest := raft.VoteRequest{
		CandidatesTerm:   currentVote.Term,
		CandidateAddress: self.address,
		LastLog:          myPrevLog,
	}

	votesChan := make(chan raft.VoteReply, len(peers))
	for _, peer := range peers {
		go requestVote(voteRequest, peer, votesChan)
	}

	// We also count our own vote
	votesGranted := 1
	halfPeers := len(peers) / 2

	hasMajority := func() bool {
		// If there is only one other node connected to the network,
		// majority equals;
		// my vote + other guy's = 2 votes
		if len(peers) == 1 {
			return votesGranted == 2
		}
		return votesGranted > halfPeers
	}

	for {
		select {
		case voteReply := <-votesChan:
			// If voter has greater term, candidate MUST step down
			if voteReply.Term > currentVote.Term {
				newState <- follower
				return false, nil
			}

			if !voteReply.VoteGranted {
				continue
			}

			votesGranted++
			if hasMajority() {
				return true, nil
			}

		// We don't have to wait for all the votes to come back;
		// timeout after n seconds and collect the results
		case <-time.After(5 * time.Second):
			return hasMajority(), nil
		}
	}
}

func sendAppendEntry(peer string, newLog *raft.Log, results chan<- bool) {
	self.mu.RLock()
	currentTerm := self.currentTerm
	commitIndex := self.commitIndex

	followersLogIndex, _ := self.peers[peer]
	myPrevLog, _ := self.logEntries.Peek()

	missedEntries := []raft.Log{}
	if followersLogIndex < myPrevLog.Index {
		missedEntries = self.logEntries.GetTopOf(followersLogIndex, commitIndex)
	}
	self.mu.RUnlock()

	if newLog != nil {
		missedEntries = append(missedEntries, *newLog)
	}

	request := raft.AppendEntriesRequest{
		LeadersTerm:        currentTerm,
		LeadersAddress:     self.address,
		LeadersCommitIndex: commitIndex,
		Entries:            missedEntries,
		PrevLogIndex:       followersLogIndex,
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
	if reply.Term > currentTerm {
		newState <- follower
		return
	}

	self.mu.Lock()
	self.peers[peer] = reply.PrevLogIndex
	self.mu.Unlock()

	results <- reply.Success
}

//	When a leader receives a request from a client, it first replicates
//	the request across several nodes on the network by sending AppendEntry RPCs.
//
// This function sends AppendEntry RPCs to peers.
func sendAppendEntries(peers []string, newLog *raft.Log) (updatedMajority bool) {
	defer func() {
		if updatedMajority && newLog != nil {
			self.mu.Lock()
			self.commitIndex = newLog.Index
			self.mu.Unlock()
		}
	}()

	if len(peers) == 0 {
		log.Println("sendAppendEntries; waiting for peers to join network...")
		_ = <-newPeersChan
	}

	// The dot indicates leader sending AppendEntry RPCs to its
	// followers
	fmt.Printf(".")
	resultsChan := make(chan bool, len(peers))
	for _, peer := range peers {
		go sendAppendEntry(peer, newLog, resultsChan)
	}

	// We also count ourselves
	peersUpdated := 1
	halfPeers := len(peers) / 2

	hasMajority := func() bool {
		// If there is only one other node connected to the network,
		// majority equals;
		// me + other guy = 2 AppendEntry replies
		if len(peers) == 1 {
			return peersUpdated == 2
		}
		return peersUpdated > halfPeers
	}

	for {
		select {
		case ok := <-resultsChan:
			if !ok {
				continue
			}

			peersUpdated++
			if hasMajority() {
				return true
			}

		// We don't have to wait for everyone to respond;
		// timeout after n seconds and collect the results
		case <-time.After(5 * time.Second):
			return hasMajority()
		}
	}
}

func getTimeout(currentState state) time.Duration {
	var (
		BASE_TIMEOUT   time.Duration = 20 * time.Second
		LEADER_TIMEOUT time.Duration = 10 * time.Second

		timeout time.Duration = BASE_TIMEOUT
	)

	switch currentState {
	case leader:
		timeout = LEADER_TIMEOUT
	default:
		// Randomized timeout variations prevent more than one
		// follower requesting for votes at around the same time.
		// 1 + randN avoids zero variation value.

		randInt := 1 + rand.IntN(5)
		variation := time.Duration(randInt) * time.Second
		timeout = BASE_TIMEOUT + variation
	}

	log.Printf("%s timeout set to %v\n", currentState, timeout)
	return timeout
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
			// It is the duty of a leader to periodically send
			// AppendEntry RPCs inorder to maintain its reign
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
			lastHeartbeat := self.lastHeartbeat
			self.mu.RUnlock()

			// A follower listens to AppendEntry RPCs from the leader
			// up until the leader no longer responds and is considered 'dead'.
			// Once a leader 'dies', the follower steps up to become a candidate
			kingsDead := time.Since(lastHeartbeat) > timeout
			if kingsDead {
				log.Println("leader is dead")
				newState <- candidate
			}
		}
	}
}

func startCandidate() {
	log.Println("becoming candidate")

	// A candidate requests votes from its peers inorder to become
	// the leader
	peers := self.getPeers()
	wonElection, err := requestVotes(peers)
	if err != nil {
		log.Printf("error requesting votes; %v\n", err)
		return
	}

	if wonElection {
		newState <- leader
	} else {
		log.Println("election lost; stepping down from candidate")
		newState <- follower
	}
}

func main() {
	go launchRPCService()

	err := joinNetwork(remote)
	if err != nil {
		log.Printf("error joining network through remote=%v; %v\n", remote, err)
	} else {
		peers := self.getPeers()
		go notifyPeersOfJoin(peers)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Wait for RPC service to launch
		time.Sleep(1 * time.Second)

		// Start server as follower
		newState <- follower
	}()

	for {
		currentState := <-newState

		self.mu.Lock()
		self.currentState = currentState
		self.mu.Unlock()

		// Cancel old context and get a new one.
		// This stops the server from doing whatever it was doing and restarts
		// it with a new context
		cancel()
		ctx, cancel = context.WithCancel(context.Background())

		peers := self.getPeers()
		if len(peers) == 0 {
			log.Println("waiting for peers to join network...")
			<-newPeersChan
		}

		switch currentState {
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
