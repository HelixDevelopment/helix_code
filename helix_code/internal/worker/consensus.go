// Package worker provides distributed worker management with consensus
package worker

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// VoteTransport is the peer-communication primitive for the Raft election +
// heartbeat fan-out. It decouples the consensus state machine from the wire:
// an in-process implementation (ChannelVoteTransport) makes a real multi-node
// election tractable + testable in-package, while a production implementation
// (gRPC/HTTP) can be injected without touching the state machine.
//
// RequestVote sends a VoteRequest to peerID and returns that peer's
// VoteResponse. A transport-level failure (peer unreachable, timeout) MUST be
// returned as a non-nil error — the caller treats an errored peer as "vote not
// granted" and never blocks forever on it.
//
// SendAppendEntries delivers a heartbeat / log-append to peerID. Errors are
// surfaced but non-fatal to the leader (a temporarily-unreachable follower
// does not cost leadership).
type VoteTransport interface {
	RequestVote(ctx context.Context, peerID string, req VoteRequest) (VoteResponse, error)
	SendAppendEntries(ctx context.Context, peerID string, req AppendRequest) error
}

// ConsensusManager handles distributed consensus for worker coordination
type ConsensusManager struct {
	nodeID      string
	peers       []string
	currentTerm int
	votedFor    string
	leaderID    string
	log         []LogEntry
	commitIndex int
	lastApplied int
	state       NodeState
	votes       map[string]bool
	mutex       sync.RWMutex

	// transport is the peer-communication primitive. When nil, the manager
	// is in single-node / no-transport mode: a multi-peer election cannot
	// gather remote votes, so the candidate steps down cleanly to Follower
	// at the end of its bounded election round instead of livelocking.
	transport VoteTransport

	// electionTimeout bounds a single election round. A candidate that has
	// not gathered a quorum within this window steps down to Follower
	// (bounded-safety) rather than remaining Candidate forever.
	electionTimeout   time.Duration
	heartbeatInterval_ time.Duration

	// Channels for Raft protocol
	voteRequests   chan VoteRequest
	voteResponses  chan VoteResponse
	appendRequests chan AppendRequest
	heartbeatTimer *time.Timer
	electionTimer  *time.Ticker

	// Callbacks
	onLeaderElected func(string)
	onStateChanged  func(NodeState)
}

// NodeState represents the current state of a Raft node
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

// LogEntry represents a log entry in Raft
type LogEntry struct {
	Term    int         `json:"term"`
	Index   int         `json:"index"`
	Command interface{} `json:"command"`
}

// VoteRequest represents a vote request in Raft
type VoteRequest struct {
	Term         int    `json:"term"`
	CandidateID  string `json:"candidate_id"`
	LastLogIndex int    `json:"last_log_index"`
	LastLogTerm  int    `json:"last_log_term"`
}

// VoteResponse represents a vote response in Raft
type VoteResponse struct {
	Term int `json:"term"`
	// NodeID identifies the responding peer so the candidate can dedupe
	// the tally: a peer that delivers the same granted VoteResponse twice
	// (network retry, duplicate delivery) is counted once via
	// cm.votes[NodeID]=true. Empty NodeID is tolerated (best-effort,
	// undeduped) for backward compatibility with hand-built responses.
	NodeID      string `json:"node_id"`
	VoteGranted bool   `json:"vote_granted"`
}

// AppendRequest represents an append entries request in Raft
type AppendRequest struct {
	Term         int        `json:"term"`
	LeaderID     string     `json:"leader_id"`
	PrevLogIndex int        `json:"prev_log_index"`
	PrevLogTerm  int        `json:"prev_log_term"`
	Entries      []LogEntry `json:"entries"`
	LeaderCommit int        `json:"leader_commit"`
}

// ConsensusConfig represents configuration for the consensus manager
type ConsensusConfig struct {
	NodeID            string
	Peers             []string
	HeartbeatInterval time.Duration
	ElectionTimeout   time.Duration
	// Transport is the peer-communication primitive used to fan out vote
	// requests and heartbeats. When nil, the manager runs in
	// single-node / no-transport mode (multi-peer elections step the
	// candidate down cleanly to Follower instead of livelocking).
	Transport       VoteTransport
	OnLeaderElected func(string)
	OnStateChanged  func(NodeState)
}

// NewConsensusManager creates a new consensus manager
func NewConsensusManager(config ConsensusConfig) *ConsensusManager {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 100 * time.Millisecond
	}
	if config.ElectionTimeout == 0 {
		config.ElectionTimeout = 500 * time.Millisecond
	}

	cm := &ConsensusManager{
		nodeID:          config.NodeID,
		peers:           config.Peers,
		state:           Follower,
		votes:           make(map[string]bool),
		transport:          config.Transport,
		electionTimeout:    config.ElectionTimeout,
		heartbeatInterval_: config.HeartbeatInterval,
		voteRequests:    make(chan VoteRequest, 100),
		voteResponses:   make(chan VoteResponse, 100),
		appendRequests:  make(chan AppendRequest, 100),
		onLeaderElected: config.OnLeaderElected,
		onStateChanged:  config.OnStateChanged,
	}

	cm.heartbeatTimer = time.NewTimer(config.HeartbeatInterval)
	cm.electionTimer = time.NewTicker(config.ElectionTimeout)

	return cm
}

// Start begins the consensus protocol
func (cm *ConsensusManager) Start(ctx context.Context) error {
	log.Printf("Starting Raft consensus for node %s", cm.nodeID)

	go cm.run(ctx)
	return nil
}

// Stop stops the consensus protocol
func (cm *ConsensusManager) Stop() {
	if cm.heartbeatTimer != nil {
		cm.heartbeatTimer.Stop()
	}
	if cm.electionTimer != nil {
		cm.electionTimer.Stop()
	}
}

// IsLeader returns true if this node is the current leader
func (cm *ConsensusManager) IsLeader() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.state == Leader
}

// GetLeader returns the current leader ID
func (cm *ConsensusManager) GetLeader() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	if cm.state == Leader {
		return cm.nodeID
	}
	// Non-leader nodes learn the leader's ID from the AppendRequest
	// (heartbeat) handler, which persists req.LeaderID into cm.leaderID.
	// Before the first heartbeat is observed this is "" — the honest
	// "leader not yet known" answer, not a stub.
	return cm.leaderID
}

// GetCurrentTerm returns the current term
func (cm *ConsensusManager) GetCurrentTerm() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.currentTerm
}

// GetNodeID returns this node's stable consensus identity. It is the value a
// peer uses as the peerID when canvassing this node for a vote, so a transport
// (e.g. InProcessCluster) can route RequestVote / SendAppendEntries here.
func (cm *ConsensusManager) GetNodeID() string {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.nodeID
}

// Reconfigure injects a new peer set + VoteTransport into a running
// ConsensusManager and resets it to a clean Follower so the next election round
// canvasses the new peers over the new transport. It is the seam the SSH worker
// pool uses to promote a single-node (empty-peer, no-transport) manager into a
// genuine multi-node cluster member without tearing down + recreating the run
// loop: startElection() / sendHeartbeats() read cm.peers + cm.transport under
// the lock on every tick, so a locked swap takes effect on the next round.
//
// Resetting to Follower (rather than leaving a possibly-self-elected single
// node as Leader) is required: a node that became leader of its own
// single-node cluster MUST relinquish that stale leadership when it learns it
// is actually one member of a larger cluster, otherwise the cluster would have
// multiple leaders. The election timer is re-armed so a fresh, contested
// election runs over the real transport.
func (cm *ConsensusManager) Reconfigure(peers []string, transport VoteTransport) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	cm.peers = append([]string(nil), peers...)
	cm.transport = transport

	// Relinquish any stale single-node leadership / candidacy: become a clean
	// Follower so the real multi-node election decides leadership.
	cm.state = Follower
	cm.votedFor = ""
	cm.leaderID = ""
	cm.votes = make(map[string]bool)

	// Re-arm the election timer so this follower starts a contested election
	// promptly over the freshly-injected transport.
	if cm.electionTimer != nil {
		timeout := cm.electionTimeout
		if timeout <= 0 {
			timeout = 500 * time.Millisecond
		}
		cm.electionTimer.Reset(timeout)
	}
	if cm.onStateChanged != nil {
		cm.onStateChanged(Follower)
	}
}

// run is the main consensus loop
func (cm *ConsensusManager) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-cm.electionTimer.C:
			cm.startElection()
		case <-cm.heartbeatTimer.C:
			// Re-arm the heartbeat timer every tick so a leader keeps
			// emitting heartbeats (the previous one-shot fired once and
			// was never re-armed in the loop, so followers never learned
			// the leader ID).
			cm.heartbeatTimer.Reset(cm.heartbeatInterval())
			if cm.IsLeader() {
				cm.sendHeartbeats()
			}
		case req := <-cm.voteRequests:
			cm.handleVoteRequest(req)
		case resp := <-cm.voteResponses:
			cm.handleVoteResponse(resp)
		case req := <-cm.appendRequests:
			cm.handleAppendRequest(req)
		}
	}
}

// startElection starts a new election round. Bounded-safety invariant
// (§11.4.112): an election round ALWAYS terminates in a definite state —
// Leader (quorum gathered) or Follower (quorum not reachable). A node NEVER
// remains Candidate after the round returns, so the previous multi-peer
// livelock (term spiralling on every election-timer tick) cannot recur.
func (cm *ConsensusManager) startElection() {
	cm.mutex.Lock()

	if cm.state == Leader {
		cm.mutex.Unlock()
		return // Already leader
	}

	cm.state = Candidate
	cm.currentTerm++
	cm.votedFor = cm.nodeID
	cm.leaderID = ""
	cm.votes = make(map[string]bool)
	cm.votes[cm.nodeID] = true

	electionTerm := cm.currentTerm
	peers := append([]string(nil), cm.peers...)
	lastLogIndex := len(cm.log) - 1
	lastLogTerm := 0
	if lastLogIndex >= 0 {
		lastLogTerm = cm.log[lastLogIndex].Term
	}
	transport := cm.transport
	timeout := cm.electionTimeout
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}

	log.Printf("Node %s starting election for term %d (peers=%d)", cm.nodeID, electionTerm, len(peers))

	if len(peers) == 0 {
		// Single-node cluster — become leader immediately. No peers to
		// canvass, no quorum to gather.
		cm.becomeLeader()
		cm.mutex.Unlock()
		return
	}

	if transport == nil {
		// Multi-peer cluster with NO transport configured. We cannot
		// gather remote votes, so we MUST NOT pretend to win and MUST
		// NOT remain Candidate forever (the previous livelock). Step
		// down cleanly to Follower (bounded-safety). The next
		// election-timer tick retries; if a transport is later injected
		// the election can succeed.
		log.Printf("WARN [§11.4 / consensus.go]: node %s multi-peer election (term %d, peers=%d) has no VoteTransport configured; cannot gather remote votes. Stepping down to Follower (bounded-safety) instead of livelocking as Candidate.",
			cm.nodeID, electionTerm, len(peers))
		cm.stepDownLocked(electionTerm)
		cm.mutex.Unlock()
		return
	}

	// Release the lock before doing network I/O so vote-request fan-out
	// (and any concurrent handlers) are not serialised behind the
	// election. The collected votes are re-locked when tallied.
	cm.mutex.Unlock()

	req := VoteRequest{
		Term:         electionTerm,
		CandidateID:  cm.nodeID,
		LastLogIndex: lastLogIndex,
		LastLogTerm:  lastLogTerm,
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	type voteResult struct {
		peerID string
		resp   VoteResponse
		err    error
	}
	results := make(chan voteResult, len(peers))
	for _, peerID := range peers {
		go func(pid string) {
			resp, err := transport.RequestVote(ctx, pid, req)
			results <- voteResult{peerID: pid, resp: resp, err: err}
		}(peerID)
	}

	// Tally with responder-ID dedup. Self already counted.
	granted := map[string]bool{cm.nodeID: true}
	totalNodes := len(peers) + 1
	quorum := totalNodes/2 + 1
	highestTerm := electionTerm

	for i := 0; i < len(peers); i++ {
		var r voteResult
		select {
		case r = <-results:
		case <-ctx.Done():
			// Bounded-safety: the round's deadline elapsed. Remaining
			// peers are treated as non-granting; we proceed to the
			// final verdict rather than waiting forever.
			i = len(peers) // break out
			goto verdict
		}
		if r.err != nil {
			log.Printf("Node %s: vote request to peer %s failed: %v (treated as not-granted)", cm.nodeID, r.peerID, r.err)
			continue
		}
		if r.resp.Term > highestTerm {
			highestTerm = r.resp.Term
		}
		if r.resp.VoteGranted {
			id := r.resp.NodeID
			if id == "" {
				id = r.peerID // fall back to the peer we asked
			}
			granted[id] = true
		}
	}

verdict:
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	// A newer term observed during the round (or arriving via another
	// handler) invalidates this candidacy — step down.
	if highestTerm > cm.currentTerm {
		cm.stepDownLocked(highestTerm)
		return
	}
	// Another path may have already changed our state/term while we were
	// canvassing; only conclude this election if we are still the
	// candidate for the same term.
	if cm.state != Candidate || cm.currentTerm != electionTerm {
		return
	}

	cm.votes = granted
	voteCount := len(granted)
	if voteCount >= quorum {
		log.Printf("Node %s won election for term %d (%d/%d votes, quorum=%d)", cm.nodeID, electionTerm, voteCount, totalNodes, quorum)
		cm.becomeLeader()
		return
	}

	// Quorum not reached within the bounded round — step down cleanly so
	// the next election-timer tick can retry with a higher term. NEVER
	// remain Candidate indefinitely.
	log.Printf("Node %s did not reach quorum for term %d (%d/%d votes, quorum=%d); stepping down to Follower (bounded-safety).", cm.nodeID, electionTerm, voteCount, totalNodes, quorum)
	cm.stepDownLocked(cm.currentTerm)
}

// stepDownLocked transitions the node to Follower at the given term. Caller
// MUST hold cm.mutex. Invoking the onStateChanged callback signals monitoring
// that the candidacy ended without a livelock.
func (cm *ConsensusManager) stepDownLocked(term int) {
	changed := cm.state != Follower
	cm.state = Follower
	if term > cm.currentTerm {
		cm.currentTerm = term
	}
	cm.votedFor = ""
	// Ensure the election timer runs again so this follower can start a
	// new election if it stops hearing heartbeats (a former leader that
	// stepped down had its election timer stopped in becomeLeader).
	if cm.electionTimer != nil {
		timeout := cm.electionTimeout
		if timeout <= 0 {
			timeout = 500 * time.Millisecond
		}
		cm.electionTimer.Reset(timeout)
	}
	if changed && cm.onStateChanged != nil {
		cm.onStateChanged(Follower)
	}
}

// HandleVoteRequest processes an incoming vote request and returns this node's
// VoteResponse. It is the receiver-side counterpart of
// VoteTransport.RequestVote: a transport implementation delivers the request
// here and ships the returned response back to the candidate. The response
// carries this node's NodeID so the candidate can dedupe its tally.
func (cm *ConsensusManager) HandleVoteRequest(req VoteRequest) VoteResponse {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	return cm.handleVoteRequestLocked(req)
}

// handleVoteRequest is the channel-fed (cm.run loop) entry point. It evaluates
// the request; the response is consumed by the transport layer in production.
func (cm *ConsensusManager) handleVoteRequest(req VoteRequest) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	_ = cm.handleVoteRequestLocked(req)
}

// handleVoteRequestLocked implements the Raft RequestVote receiver rules.
// Caller MUST hold cm.mutex.
func (cm *ConsensusManager) handleVoteRequestLocked(req VoteRequest) VoteResponse {
	if req.Term > cm.currentTerm {
		cm.currentTerm = req.Term
		cm.state = Follower
		cm.votedFor = ""
		cm.leaderID = ""
	}

	resp := VoteResponse{
		Term:        cm.currentTerm,
		NodeID:      cm.nodeID,
		VoteGranted: false,
	}

	if req.Term == cm.currentTerm &&
		(cm.votedFor == "" || cm.votedFor == req.CandidateID) &&
		cm.isLogUpToDate(req.LastLogIndex, req.LastLogTerm) {

		cm.votedFor = req.CandidateID
		resp.VoteGranted = true
		log.Printf("Node %s granted vote to %s for term %d", cm.nodeID, req.CandidateID, req.Term)
	}

	return resp
}

// handleVoteResponse handles incoming vote responses
func (cm *ConsensusManager) handleVoteResponse(resp VoteResponse) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if resp.Term > cm.currentTerm {
		cm.currentTerm = resp.Term
		cm.state = Follower
		cm.votedFor = ""
		return
	}

	if cm.state == Candidate && resp.Term == cm.currentTerm && resp.VoteGranted {
		// Responder-ID-deduped tally. VoteResponse now carries NodeID,
		// so a peer that delivers the same granted response twice
		// (network retry, duplicate delivery) is counted exactly once
		// via cm.votes[id]=true. Empty NodeID is tolerated for
		// backward-compat with hand-built responses but cannot be
		// deduped — logged so monitoring catches malformed responses.
		id := resp.NodeID
		if id == "" {
			log.Printf("WARN [§11.4 / consensus.go]: handleVoteResponse received granted vote with empty NodeID — cannot dedupe this one. Populate VoteResponse.NodeID to close.")
		} else {
			if cm.votes == nil {
				cm.votes = make(map[string]bool)
			}
			if cm.votes[id] {
				return // duplicate, already counted
			}
			cm.votes[id] = true
		}

		voteCount := 0
		for _, granted := range cm.votes {
			if granted {
				voteCount++
			}
		}
		if id == "" {
			voteCount++ // best-effort count for the undedupable response
		}

		totalNodes := len(cm.peers) + 1 // peers + self
		if voteCount >= totalNodes/2+1 {
			cm.becomeLeader()
		}
	}
}

// HandleAppendRequest processes an incoming append-entries / heartbeat and is
// the receiver-side counterpart of VoteTransport.SendAppendEntries. It records
// the leader ID so followers can answer GetLeader() honestly.
func (cm *ConsensusManager) HandleAppendRequest(req AppendRequest) {
	cm.handleAppendRequest(req)
}

// handleAppendRequest handles incoming append entries requests
func (cm *ConsensusManager) handleAppendRequest(req AppendRequest) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if req.Term >= cm.currentTerm {
		// A heartbeat from a valid leader for our term (or newer) means
		// we are a follower of that leader. A candidate that learns of a
		// current-term leader MUST step down (bounded-safety: a split
		// vote resolves the moment a leader's heartbeat arrives).
		cm.currentTerm = req.Term
		cm.state = Follower
		cm.votedFor = ""
		cm.leaderID = req.LeaderID
	}

	// Reset election timer — a live leader keeps us from starting an
	// election.
	if cm.electionTimer != nil {
		timeout := cm.electionTimeout
		if timeout <= 0 {
			timeout = 500 * time.Millisecond
		}
		cm.electionTimer.Reset(timeout)
	}
}

// sendHeartbeats sends heartbeats to followers (when leader)
func (cm *ConsensusManager) sendHeartbeats() {
	cm.mutex.RLock()
	if cm.state != Leader {
		cm.mutex.RUnlock()
		return
	}

	// Send append entries requests to all peers
	appendReq := AppendRequest{
		Term:         cm.currentTerm,
		LeaderID:     cm.nodeID,
		PrevLogIndex: len(cm.log) - 1,
		PrevLogTerm:  0,
		Entries:      []LogEntry{}, // Empty for heartbeat
		LeaderCommit: cm.commitIndex,
	}
	if len(cm.log) > 0 {
		appendReq.PrevLogTerm = cm.log[len(cm.log)-1].Term
	}
	peers := append([]string(nil), cm.peers...)
	transport := cm.transport
	nodeID := cm.nodeID
	term := cm.currentTerm
	cm.mutex.RUnlock()

	if len(peers) == 0 {
		log.Printf("Leader %s sending heartbeat for term %d (single-node cluster, no transport needed)", nodeID, term)
		return
	}

	if transport == nil {
		// No transport: followers will not see heartbeats. This is not a
		// livelock (the leader keeps its term), but it is a degraded
		// multi-peer mode — surface it so monitoring catches it. A
		// production deployment MUST inject a transport.
		log.Printf("WARN [§11.4 / consensus.go]: leader %s sendHeartbeats for term %d has no VoteTransport configured (peers=%d); followers will not see heartbeats until a transport is injected.",
			nodeID, term, len(peers))
		return
	}

	// Fan out heartbeats. A temporarily-unreachable follower is logged but
	// does not cost leadership. We WAIT for the fan-out to complete before
	// returning so the bounding context is not cancelled out from under the
	// in-flight sends (a defer-cancel here would abort every heartbeat).
	timeout := cm.heartbeatTimeout()
	var wg sync.WaitGroup
	for _, peerID := range peers {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := transport.SendAppendEntries(ctx, pid, appendReq); err != nil {
				log.Printf("Leader %s: heartbeat to follower %s failed: %v", nodeID, pid, err)
			}
		}(peerID)
	}
	wg.Wait()
}

// heartbeatInterval returns the configured heartbeat cadence (default 100ms).
func (cm *ConsensusManager) heartbeatInterval() time.Duration {
	cm.mutex.RLock()
	h := cm.heartbeatInterval_
	cm.mutex.RUnlock()
	if h <= 0 {
		h = 100 * time.Millisecond
	}
	return h
}

// heartbeatTimeout bounds a single heartbeat fan-out round.
func (cm *ConsensusManager) heartbeatTimeout() time.Duration {
	t := cm.electionTimeout
	if t <= 0 {
		t = 500 * time.Millisecond
	}
	return t
}

// becomeLeader transitions this node to leader state
func (cm *ConsensusManager) becomeLeader() {
	cm.state = Leader
	cm.leaderID = cm.nodeID
	log.Printf("Node %s became leader for term %d", cm.nodeID, cm.currentTerm)

	if cm.onLeaderElected != nil {
		cm.onLeaderElected(cm.nodeID)
	}

	if cm.onStateChanged != nil {
		cm.onStateChanged(Leader)
	}

	// Stop the election timer: a leader does not start elections. Re-arm
	// the heartbeat timer at the configured cadence and emit an immediate
	// heartbeat so followers learn the new leader without waiting a full
	// heartbeat interval (closes the window where a follower's election
	// timer could fire before the first heartbeat arrives).
	if cm.electionTimer != nil {
		cm.electionTimer.Stop()
	}
	h := cm.heartbeatInterval_
	if h <= 0 {
		h = 100 * time.Millisecond
	}
	if cm.heartbeatTimer != nil {
		cm.heartbeatTimer.Reset(h)
	}
	go cm.sendHeartbeats()
}

// isLogUpToDate checks if candidate's log is at least as up-to-date as ours
func (cm *ConsensusManager) isLogUpToDate(lastLogIndex, lastLogTerm int) bool {
	ourLastIndex := len(cm.log) - 1
	ourLastTerm := 0
	if ourLastIndex >= 0 {
		ourLastTerm = cm.log[ourLastIndex].Term
	}

	if lastLogTerm > ourLastTerm {
		return true
	}
	if lastLogTerm == ourLastTerm && lastLogIndex >= ourLastIndex {
		return true
	}
	return false
}

// ProposeTask proposes a task for consensus (leader only)
func (cm *ConsensusManager) ProposeTask(task interface{}) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.state != Leader {
		return fmt.Errorf("%s", tr(context.Background(), "internal_worker_not_leader", nil))
	}

	// Add to log
	entry := LogEntry{
		Term:    cm.currentTerm,
		Index:   len(cm.log),
		Command: task,
	}
	cm.log = append(cm.log, entry)

	log.Printf("Leader %s proposed task: %+v", cm.nodeID, task)
	return nil
}

// GetQuorumSize returns the quorum size for the current cluster
func (cm *ConsensusManager) GetQuorumSize() int {
	return (len(cm.peers)+1)/2 + 1
}

// InProcessCluster wires a set of ConsensusManagers running in the same
// process so they can hold a real election over an in-memory transport. It is
// a fully-implemented VoteTransport (not a mock) — useful for embedded
// single-process multi-node coordination and for end-to-end election tests
// that exercise the real state machine rather than a stub.
type InProcessCluster struct {
	mu    sync.RWMutex
	nodes map[string]*ConsensusManager
}

// NewInProcessCluster creates an empty in-process cluster.
func NewInProcessCluster() *InProcessCluster {
	return &InProcessCluster{nodes: make(map[string]*ConsensusManager)}
}

// Register adds a node to the cluster so it can receive vote/append RPCs.
func (c *InProcessCluster) Register(cm *ConsensusManager) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nodes[cm.nodeID] = cm
}

// RequestVote delivers req to peerID's HandleVoteRequest and returns its
// response. An unregistered peer is reported as unreachable (non-nil error),
// which the candidate treats as "not granted".
func (c *InProcessCluster) RequestVote(ctx context.Context, peerID string, req VoteRequest) (VoteResponse, error) {
	c.mu.RLock()
	node, ok := c.nodes[peerID]
	c.mu.RUnlock()
	if !ok {
		return VoteResponse{}, fmt.Errorf("peer %s not reachable in cluster", peerID)
	}
	select {
	case <-ctx.Done():
		return VoteResponse{}, ctx.Err()
	default:
	}
	return node.HandleVoteRequest(req), nil
}

// SendAppendEntries delivers req to peerID's HandleAppendRequest.
func (c *InProcessCluster) SendAppendEntries(ctx context.Context, peerID string, req AppendRequest) error {
	c.mu.RLock()
	node, ok := c.nodes[peerID]
	c.mu.RUnlock()
	if !ok {
		return fmt.Errorf("peer %s not reachable in cluster", peerID)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	node.HandleAppendRequest(req)
	return nil
}
