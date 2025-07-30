### What is Consensus?
In a distributed system, **consensus** refers to the process of agreeing on a single value among multiple processes or nodes.  
For a distributed state machine (like a replicated log), this means ensuring that all nodes agree on the **sequence of operations** (entries in the log) even in the presence of node failures or network partitions.

---

### Why Raft?
Raft achieves consensus by decomposing the problem into several relatively independent subproblems:

- **Leader Election**: How a new leader is chosen when the current one fails or none exists.
- **Log Replication**: How the leader manages and replicates log entries to followers.
- **Safety**: Ensuring that the system never returns an incorrect result, even during failures.

---

### Raft States
Each node in a Raft cluster can be in one of **three states**:

1. **Follower**
    - Passive state.
    - Responds to RPCs from leaders and candidates.
    - If it receives no communication within a certain timeout, it becomes a candidate.

2. **Candidate**
    - Initiates a new election.
    - Votes for itself and requests votes from other nodes.
    - If it wins, it becomes the leader.
    - If another node wins, it reverts to follower.
    - If no one wins, it starts a new election.

3. **Leader**
    - Handles all client requests.
    - Replicates log entries to followers.
    - Sends periodic heartbeats (empty `AppendEntries` RPCs) to followers to maintain authority.

---

### Terms
Raft divides time into **arbitrary-length terms** (continuously increasing integers).  
Each term starts with an election. If a leader is elected, it remains the leader for the rest of the term.  
Terms help Raft **detect stale information and conflicts**.

---

### Leader Election

#### Election Trigger
- If a follower’s election timeout expires without hearing from a leader, it becomes a candidate.

#### Vote Request Process
- The candidate:
  - Increments its current term.
  - Votes for itself.
  - Sends `RequestVote` RPCs to all other nodes.

#### Voting Rules
A node grants its vote if:

- The candidate's term is **equal to or greater than** its own.
- It hasn't voted yet in the current term, or it already voted for the candidate.
- The candidate’s log is **at least as up-to-date** as its own.

#### Election Outcomes

- **Candidate Wins**: If it receives votes from a majority, it becomes the leader.
- **Another Leader Discovered**: If it receives `AppendEntries` from a node with a higher/equal term, it reverts to follower.
- **Timeout / Split Vote**: If no winner, a new election starts with a new term.
---

### Log Replication

#### Client Request Handling
- A client sends a command to the **leader**.

#### Append Entry
- Leader appends the command as a **new log entry** (includes term, index, and command).

#### AppendEntries RPC
- Leader sends these RPCs to **all followers**, asking them to replicate the entry.

#### Consistency Check
- Each `AppendEntries` RPC includes the **term and index** of the previous entry.
- Followers reject if logs don’t match; the leader retries with an earlier entry until alignment is found.

#### Commitment
- An entry is **committed** once replicated to a **majority**.
- Committed entries are applied to the **state machine** on leader and followers.
<br>
<br>
<br>
---
<br>
<br>

### Safety Properties
Raft guarantees:

- **Election Safety**: Only one leader per term.
- **Leader Append-Only**: Leaders only append to logs; no overwrites.
- **Log Matching**: If two logs have an entry at the same index and term, all previous entries must match.
- **Leader Completeness**: Committed entries in a term are present in future leaders’ logs.
- **State Machine Safety**: Once a log entry is applied, no different command will be applied at that index on any server.
<br>
<br>
<br>
---
<br>
<br>

### Handling Failures

#### Leader Failure
- No heartbeats → Followers start a new election.

#### Follower Failure
- Leader continues sending RPCs.
- On recovery, the follower catches up automatically.

#### Network Partitions
- A **majority partition** can elect a new leader.
- The **minority** becomes stale, then syncs after healing.
<br>
<br>
<br>
---
<br>
<br>

### Summary

Raft achieves strong consistency and fault tolerance through:

- **Strong Leader**: All changes go through a single leader.
- **Log Consistency**: Via strict rules for log matching and completeness.
- **Majority Rule**: A majority must agree on all key decisions.

Raft is a **robust and widely used** consensus algorithm ideal for reliable distributed systems.

