package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSSHWorkerPool_DistributedElectionThroughPool is the W6B RED→GREEN proof
// that distributed leader election genuinely runs THROUGH the SSH worker pool,
// across the pool's configured peer set, over an injected real VoteTransport.
//
// §11.4.115 polarity switch: RED_MODE=1 asserts the BROKEN pre-fix behaviour —
// each pool's ConsensusManager was constructed with an EMPTY peer set and NO
// transport (ssh_pool.go NewSSHWorkerPoolWithConfig built it with
// Peers:[]string{} and never injected a Transport, and AddWorker never wired
// the consensus peer set), so every pool degraded to a single-node cluster:
// each one elected ITSELF leader, none knew the others as peers, and no shared
// transport existed for vote fan-out. RED_MODE=1 therefore expects EVERY pool
// to be its own leader (3 leaders across 3 pools — the single-node tell).
//
// GREEN (default RED_MODE=0, post-fix): after WireConsensusCluster joins the
// pools into one cluster with a shared real transport + the correct peer sets,
// a genuine multi-node Raft election runs through the pool and EXACTLY ONE pool
// wins leadership; the other two recognise it as leader. That is impossible
// under the single-node-degrade path, so this test FAILs on the old code and
// PASSes on the fixed code.
func TestSSHWorkerPool_DistributedElectionThroughPool(t *testing.T) {
	// Three pools standing in for three HelixCode nodes. autoInstall=false so
	// no SSH side effects occur; we are exercising the consensus wiring only.
	pools := []*SSHWorkerPool{
		NewSSHWorkerPool(false),
		NewSSHWorkerPool(false),
		NewSSHWorkerPool(false),
	}
	defer func() {
		for _, p := range pools {
			p.StopConsensus()
		}
	}()

	if redMode() {
		// Broken-artifact expectation: with no cluster wiring, each pool is an
		// isolated single-node cluster and elects itself. Give the single-node
		// elections a beat to settle, then assert ALL THREE are leaders.
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			selfLeaders := 0
			for _, p := range pools {
				if p.IsConsensusLeader() {
					selfLeaders++
				}
			}
			if selfLeaders == len(pools) {
				break
			}
			time.Sleep(20 * time.Millisecond)
		}
		selfLeaders := 0
		for _, p := range pools {
			if p.IsConsensusLeader() {
				selfLeaders++
			}
		}
		assert.Equal(t, len(pools), selfLeaders,
			"RED: every un-wired pool is its own single-node leader")
		return
	}

	// GREEN: wire the pools into one real multi-node cluster (shared transport +
	// correct peer sets) and let a genuine election run THROUGH the pool.
	require.NoError(t, WireConsensusCluster(
		20*time.Millisecond, // heartbeat
		pools...,
	), "wiring the pool cluster must succeed")

	// Poll for exactly one leader across the cluster within a bounded window.
	var leaderIdx = -1
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		leaders := 0
		idx := -1
		for i, p := range pools {
			if p.IsConsensusLeader() {
				leaders++
				idx = i
			}
		}
		if leaders == 1 {
			leaderIdx = idx
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	require.GreaterOrEqual(t, leaderIdx, 0,
		"a single leader MUST be elected through the pool cluster within the bounded window")

	leaderNodeID := pools[leaderIdx].GetConsensusNodeID()
	t.Logf("distributed election through pool: leader pool index=%d node=%s term=%d",
		leaderIdx, leaderNodeID, pools[leaderIdx].GetConsensusTerm())

	// Exactly one leader across the whole cluster (NOT three single-node selves).
	leaders := 0
	for _, p := range pools {
		if p.IsConsensusLeader() {
			leaders++
		}
	}
	assert.Equal(t, 1, leaders, "exactly one leader across the pool cluster, not single-node-per-pool")

	// Followers learn the leader's ID via heartbeats fanned out over the shared
	// transport — proving the transport is real and wired, not a nil no-op.
	time.Sleep(150 * time.Millisecond)
	for i, p := range pools {
		if i == leaderIdx {
			assert.Equal(t, leaderNodeID, p.GetConsensusLeaderID(), "leader knows itself")
			continue
		}
		assert.False(t, p.IsConsensusLeader(), "follower pool %d is not leader", i)
		assert.Equal(t, leaderNodeID, p.GetConsensusLeaderID(),
			"follower pool %d recognises leader %s over the wired transport", i, leaderNodeID)
	}
}
