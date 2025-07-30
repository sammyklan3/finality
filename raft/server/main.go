package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sammyklan3/finality/raft"
)

var (
	remote    string
	self      *RaftServer
	broadcast = make(chan string)
)

func init() {
	flag.StringVar(&remote, "remote", "", "Address of the server to contact to join raft network")
	flag.Parse()

	host, port := raft.NewAddress("localhost", 1024)
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

	log.Printf("started new 🛶 raft server %v on %v\n", self.serverId, self.address)

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
	self.peers.Add(remote)
	self.peers.Add(peers...)
	self.mu.Unlock()

	go notifyPeersOfNewServer(peers)

	return peers, nil
}

func notifyPeersOfNewServer(peers []string) {
	wg := sync.WaitGroup{}
	wg.Add(len(peers))
	var peers_notified int32 = 0

	for _, peer := range peers {
		go func(peer string) {
			defer wg.Done()

			client, err := rpc.Dial("tcp", peer)
			if err != nil {
				log.Printf("error dialing peer; %v\n", err)
				return
			}
			defer client.Close()

			request := raft.JoinRequest{Address: self.address}
			reply := raft.JoinReply{}
			err = client.Call("RaftServer.JoinNetwork", &request, &reply)
			if err != nil {
				log.Printf("error calling RaftServer.JoinNetwork RPC func; %v\n", err)
				return
			}

			log.Printf("notified %v of me joining the network\n", peer)
			atomic.AddInt32(&peers_notified, 1)
		}(peer)
	}

	wg.Wait()
}

func requestVote(request raft.VoteRequest, address string, result_chan chan<- raft.VoteReply) {
	log.Printf("requesting %v for their vote 🗳️ on term %v\n", address, request.CandidatesTerm)

	client, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Printf("error requesting vote; %v\n", err)
		if errors.Is(err, syscall.ECONNREFUSED) {
			self.peers.Remove(address)
		}
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
	if len(peers) == 0 {
		return false, fmt.Errorf("zero nodes connected to network")
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
	self.mu.Unlock()

	// vote for self
	my_vote := raft.Vote{
		Term:     current_vote.Term,
		VotedFor: self.serverId,
	}
	err = self.saveVote(my_vote)
	if err != nil {
		return false, fmt.Errorf("error voting for self; %v", err)
	}

	vote_request := raft.VoteRequest{
		CandidatesTerm: current_vote.Term,
		CandidateId:    self.serverId,
	}

	votes_chan := make(chan raft.VoteReply, len(peers))

	// send requestVote RPCs to all peer servers
	for _, peer := range peers {
		go requestVote(vote_request, peer, votes_chan)
	}

	votes_granted := 1 // start counting at 1 bcoz we already voted for ourselves
	half_peers := len(peers) / 2

	for {
		select {
		case vote_reply := <-votes_chan:
			// if voter has greater term, candidate MUST step down
			if vote_reply.Term > current_vote.Term {
				return false, nil
			}

			if vote_reply.VoteGranted {
				votes_granted++
			}

			has_majority := votes_granted > half_peers
			if has_majority {
				return has_majority, nil
			}

		case <-time.After(5 * time.Second):
			has_majority := votes_granted > half_peers
			return has_majority, nil
		}
	}
}

func sendAppendEntry(request raft.AppendEntriesRequest, address string, result_chan chan<- *raft.AppendEntriesReply) {
	client, err := rpc.Dial("tcp", address)
	if err != nil {
		log.Printf("error sending append entry to address %v; %v\n", address, err)
		if errors.Is(err, syscall.ECONNREFUSED) {
			self.peers.Remove(address)
		}
		return
	}
	defer client.Close()

	reply := raft.AppendEntriesReply{}
	err = client.Call("RaftServer.AppendEntries", &request, &reply)
	if err != nil {
		log.Printf("error sending append entry to address %v; %v\n", address, err)
		return
	}

	result_chan <- &reply
}

func sendAppendEntries(request *raft.AppendEntriesRequest, peers []string) (int, error) {
	log.Println("sending appendEntry RPCs")

	if len(peers) == 0 {
		return 0, fmt.Errorf("zero peers connected to network")
	}

	self.mu.RLock()
	current_term := self.currentVote.Term
	self.mu.RUnlock()

	if request == nil {
		request = &raft.AppendEntriesRequest{
			LeadersTerm:    current_term,
			LeadersAddress: self.address,
			// TODO: Add log entries here
		}
	}

	results_chan := make(chan *raft.AppendEntriesReply, len(peers))
	for _, peer := range peers {
		go sendAppendEntry(*request, peer, results_chan)
	}

	servers_updated := 0
	half_peers := len(peers) / 2

	for {
		select {
		case result := <-results_chan:
			if result.Success {
				servers_updated++
			}

			updated_majority := servers_updated > half_peers
			if updated_majority {
				return servers_updated, nil
			}

		case <-time.After(5 * time.Second):
			return servers_updated, nil
		}
	}
}

func main() {
	go launchRPCService()

	// Wait for RPC service to launch
	time.Sleep(1 * time.Second)

	_, err := joinNetwork()
	if err != nil {
		log.Printf("error joining raft network; %v\n", err)
	}

	timeout := 0 * time.Second

	for {
		<-time.After(timeout)

		self.mu.RLock()
		current_state := self.currentState
		last_heartbeat := self.lastHeartbeat
		timeout = self.timeout
		peers := self.peers.GetItems()
		self.mu.RUnlock()

		if len(peers) == 0 {
			log.Println("waiting for peers to join network...")
			_ = <-broadcast
			continue
		}

		if current_state == leader {
			_, err = sendAppendEntries(nil, peers)
			if err != nil {
				log.Printf("error sending append entries; %v\n", err)
			}
			continue
		}

		leader_is_dead := time.Since(last_heartbeat) > timeout
		if !leader_is_dead {
			continue
		}

		log.Println("leader possibly dead")

		if current_state == follower {
			won_election, err := requestVotes(peers)
			if err != nil {
				log.Printf("error sending vote requests; %v\n", err)
				continue
			}

			var new_state state

			if won_election {
				log.Println("you were selected as leader! 🍻")
				new_state = leader

			} else {
				log.Println("failed to achieve majority votes")
				new_state = follower
			}

			if new_state == follower {
				log.Println("stepping down from candidate")
			}

			new_timeout := getTimeout(new_state)

			self.mu.Lock()
			self.currentState = new_state
			self.timeout = new_timeout
			self.mu.Unlock()
			continue
		}

	}
}
