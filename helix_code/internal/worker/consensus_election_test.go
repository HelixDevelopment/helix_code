package worker

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redMode reports whether the polarity switch (§11.4.115) is engaged. When
// RED_MODE=1 the bounded-election tests run against the EXPECTATION of the
// broken pre-fix behaviour (no leader, term spiral / livelock). With the fix
// in place RED_MODE=1 will FAIL (the defect is gone) — which is the correct
// signal. Default RED_MODE=0: the test is the GREEN regression-guard asserting
// the defect is ABSENT.
func redMode() bool { return os.Getenv("RED_MODE") == "1" }

// TestConsensusManager_MultiPeerElectionResolves_BoundedSafety is the
// no-transport bounded-safety guard: a multi-peer node with NO transport MUST
// NOT livelock as Candidate; it MUST step down cleanly to Follower within a
// bounded number of election rounds, and its term MUST NOT spiral unboundedly.
func TestConsensusManager_MultiPeerElectionResolves_BoundedSafety(t *testing.T) {
	cm := NewConsensusManager(ConsensusConfig{
		NodeID:          "node-1",
		Peers:           []string{"node-2", "node-3"},
		ElectionTimeout: 20 * time.Millisecond,
	})

	// DETERMINISTIC, race-free assertion: invoke a single election round
	// synchronously (no transport configured) and inspect the terminal state
	// directly. The bounded-safety invariant is that ONE election round
	// ALWAYS terminates in a definite state — never leaving the node stuck as
	// Candidate. This is exactly the livelock the broken artifact exhibited
	// (it stayed Candidate after every round), so the assertion catches a
	// re-introduced no-step-down regression with no timing dependence.
	cm.startElection()

	cm.mutex.RLock()
	state := cm.state
	term := cm.currentTerm
	cm.mutex.RUnlock()
	t.Logf("no-transport bounded-safety: after one election round state=%v term=%d", state, term)

	if redMode() {
		// Broken-artifact expectation: the round leaves the node Candidate.
		assert.Equal(t, Candidate, state, "RED expects stuck Candidate after round")
		return
	}
	// GREEN: a node with peers but no transport cannot win, so the round MUST
	// terminate it as Follower (clean step-down), NEVER as Candidate.
	assert.Equal(t, Follower, state, "no-transport multi-peer election MUST step down to Follower, not stay Candidate (livelock)")
	assert.NotEqual(t, Leader, state, "MUST NOT fake-win without a quorum")
}

// TestConsensusManager_ThreeNodeElection_RealTransport exercises a REAL 3-node
// in-process election over the ChannelVoteTransport (InProcessCluster). Exactly
// one node MUST win leadership and the other two MUST recognise it.
func TestConsensusManager_ThreeNodeElection_RealTransport(t *testing.T) {
	cluster := NewInProcessCluster()

	ids := []string{"node-1", "node-2", "node-3"}
	var leaderElected int32
	managers := make(map[string]*ConsensusManager, 3)

	for i, id := range ids {
		peers := []string{}
		for j, pid := range ids {
			if i != j {
				peers = append(peers, pid)
			}
		}
		// Raft timing discipline: heartbeat interval (20ms) MUST be well
		// below the staggered election timeouts (150/250/350ms) so a stable
		// leader's heartbeats reset followers' election timers before they
		// fire, and one node reliably starts first and wins.
		cm := NewConsensusManager(ConsensusConfig{
			NodeID:            id,
			Peers:             peers,
			Transport:         cluster,
			HeartbeatInterval: 20 * time.Millisecond,
			ElectionTimeout:   time.Duration(150+i*100) * time.Millisecond,
			OnLeaderElected: func(string) {
				atomic.AddInt32(&leaderElected, 1)
			},
		})
		cluster.Register(cm)
		managers[id] = cm
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, cm := range managers {
		require.NoError(t, cm.Start(ctx))
	}
	defer func() {
		for _, cm := range managers {
			cm.Stop()
		}
	}()

	// Poll for a leader within a bounded window.
	var leaderID string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		leaders := 0
		lid := ""
		for id, cm := range managers {
			if cm.IsLeader() {
				leaders++
				lid = id
			}
		}
		if leaders == 1 {
			leaderID = lid
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if redMode() {
		assert.Empty(t, leaderID, "RED expects NO leader (broken transport)")
		return
	}

	require.NotEmpty(t, leaderID, "a leader MUST be elected within the bounded window")
	t.Logf("3-node election: leader=%s term=%d", leaderID, managers[leaderID].GetCurrentTerm())

	// Exactly one leader.
	leaders := 0
	for _, cm := range managers {
		if cm.IsLeader() {
			leaders++
		}
	}
	assert.Equal(t, 1, leaders, "exactly one leader")

	// Give heartbeats time to propagate so followers learn the leader ID.
	time.Sleep(120 * time.Millisecond)
	for id, cm := range managers {
		if id == leaderID {
			assert.Equal(t, leaderID, cm.GetLeader(), "leader knows itself")
			continue
		}
		assert.False(t, cm.IsLeader(), "follower %s is not leader", id)
		assert.Equal(t, leaderID, cm.GetLeader(), "follower %s recognises leader %s", id, leaderID)
	}
}

// TestConsensusManager_VoteResponseDedup proves the responder-ID dedup: a
// duplicate granted VoteResponse from the same peer is counted once and does
// not inflate quorum.
func TestConsensusManager_VoteResponseDedup(t *testing.T) {
	cm := NewConsensusManager(ConsensusConfig{
		NodeID: "node-1",
		Peers:  []string{"node-2", "node-3", "node-4"}, // 4 nodes, quorum=3
	})
	cm.state = Candidate
	cm.currentTerm = 1
	cm.votes = map[string]bool{"node-1": true}

	// Same peer grants twice — must count once, must NOT reach quorum (3).
	resp := VoteResponse{Term: 1, NodeID: "node-2", VoteGranted: true}
	cm.handleVoteResponse(resp)
	cm.handleVoteResponse(resp)

	cm.mutex.RLock()
	state := cm.state
	count := 0
	for _, g := range cm.votes {
		if g {
			count++
		}
	}
	cm.mutex.RUnlock()

	assert.Equal(t, 2, count, "duplicate vote from node-2 counted once (self + node-2)")
	assert.Equal(t, Candidate, state, "must not become leader on deduped sub-quorum tally")
}
