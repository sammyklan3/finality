package raft

type EmptyArgs struct{}

type Vote struct {
	Term     uint64 `json:"term"`
	VotedFor string `json:"voted_for"`
}

type VoteRequest struct {
	CandidatesTerm   uint64 `json:"term"`
	CandidateAddress string `json:"candidate_address"`
	LastLog          Log    `json:"last_log"`
}

type VoteReply struct {
	Term        uint64 `json:"term"`
	VoteGranted bool   `json:"vote_granted"`
}

type JoinRequest struct {
	Address      string `json:"address"`
	PrevLogIndex uint64 `json:"prev_log_index"`
}

type JoinReply struct {
	RaftServers []string `json:"raft_servers"`
}

type AppendEntriesRequest struct {
	LeadersTerm        uint64 `json:"leaders_term"`
	LeadersAddress     string `json:"leaders_address"`
	LeadersCommitIndex uint64 `json:"leaders_commit_index"`
	Entries            []Log  `json:"entries"`
	PrevLogIndex       uint64 `json:"prev_log_index"`
}

type AppendEntriesReply struct {
	Term         uint64 `json:"term"`
	Success      bool   `json:"success"`
	PrevLogIndex uint64 `json:"prev_log_index"`
}

type Log struct {
	Index     uint64 `json:"index"`
	Msg       string `json:"msg"`
	Term      uint64 `json:"term"`
	Committed bool   `json:"committed"`
}

type Request struct {
	Msg string `json:"msg"`
}

type Reply struct {
	Success bool `json:"success"`
	Result  any  `json:"result"`
}
