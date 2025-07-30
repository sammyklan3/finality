package raft

type EmptyArgs struct{}

type Vote struct {
	Term     int    `json:"term"`
	VotedFor string `json:"votedFor"`
}

type VoteRequest struct {
	CandidatesTerm int    `json:"term"`
	CandidateId    string `json:"candidateId"`
	LastLogIndex   int    `json:"lastLogIndex"`
	LastLogTerm    int    `json:"lastLogTerm"`
}

type VoteReply struct {
	Term        int  `json:"term"`
	VoteGranted bool `json:"voteGranted"`
}

type JoinRequest struct {
	Address string `json:"address"`
}

type JoinReply struct {
	RaftServers []string `json:"raft_servers"`
}

type AppendEntriesRequest struct {
	LeadersTerm       int    `json:"leaders_term"`
	LeadersAddress    string `json:"leaders_address"`
	PrevLog           Log    `json:"prev_log"`
	Entries           []Log  `json:"entries"`
	LeaderCommitIndex int    `json:"leader_commit_index"`
}

type AppendEntriesReply struct {
	Term    int  `json:"term"`
	Success bool `json:"success"`
}

type Log struct {
	Msg  string `json:"msg"`
	Term int    `json:"term"`
}

type Request struct {
	Msg string `json:"msg"`
}

type Reply struct {
	Success bool `json:"success"`
	Result  any  `json:"result"`
}
