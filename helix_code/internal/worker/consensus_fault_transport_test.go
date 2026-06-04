package worker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// faultVoteTransport is a TEST-ONLY fault-injecting VoteTransport that wraps a
// real InProcessCluster (the genuine in-process transport — NOT a mock). It
// delegates every RPC to the wrapped cluster unless a fault is configured, in
// which case it drops, delays-to-error, or hard-errors the RPC. This is the
// §11.4.85(B) chaos seam for the consensus election: it injects network-fault
// (drop/error) and process-death (peer removal) faults at the transport layer
// without touching the production state machine.
//
// It lives in a *_test.go file (CONST-050(A) / §11.4.27: fakes & fault
// injectors are permitted ONLY in unit-test sources) and is never imported by
// production code.
//
// All knobs are goroutine-safe (atomics + a mutex-guarded peer-down set) because
// the election fans out RequestVote concurrently across peers.
type faultVoteTransport struct {
	inner *InProcessCluster

	// dropVoteNum/dropVoteDen express the fraction of RequestVote RPCs to drop
	// (return an error, i.e. "peer unreachable"). A drop is decided by a simple
	// rolling counter so the fraction is deterministic for a given call order:
	// every call where (count % den) < num is dropped. den<=0 disables dropping.
	dropVoteNum int64
	dropVoteDen int64
	voteCalls   int64

	// dropAppendNum/dropAppendDen do the same for SendAppendEntries (heartbeats).
	dropAppendNum int64
	dropAppendDen int64
	appendCalls   int64

	// hardErrorVotes, when true, makes EVERY RequestVote return an error
	// (transport-returns-errors-mid-election chaos class).
	hardErrorVotes atomic.Bool

	// peerDown is the set of peerIDs whose RPCs are forced to fail, modelling a
	// peer killed/stopped mid-election (process-death chaos class).
	mu       sync.Mutex
	peerDown map[string]bool

	// counters for evidence.
	votesDropped   int64
	votesErrored   int64
	appendsDropped int64
}

// newFaultVoteTransport wraps an InProcessCluster with fault injection disabled.
func newFaultVoteTransport(inner *InProcessCluster) *faultVoteTransport {
	return &faultVoteTransport{inner: inner, peerDown: make(map[string]bool)}
}

// setVoteDropFraction configures the fraction num/den of RequestVote RPCs to drop.
func (f *faultVoteTransport) setVoteDropFraction(num, den int) {
	atomic.StoreInt64(&f.dropVoteNum, int64(num))
	atomic.StoreInt64(&f.dropVoteDen, int64(den))
}

// setAppendDropFraction configures the fraction num/den of heartbeat RPCs to drop.
func (f *faultVoteTransport) setAppendDropFraction(num, den int) {
	atomic.StoreInt64(&f.dropAppendNum, int64(num))
	atomic.StoreInt64(&f.dropAppendDen, int64(den))
}

// killPeer marks peerID down: all subsequent RPCs to it fail (process-death).
func (f *faultVoteTransport) killPeer(peerID string) {
	f.mu.Lock()
	f.peerDown[peerID] = true
	f.mu.Unlock()
}

// revivePeer clears a previously-killed peer.
func (f *faultVoteTransport) revivePeer(peerID string) {
	f.mu.Lock()
	delete(f.peerDown, peerID)
	f.mu.Unlock()
}

func (f *faultVoteTransport) isDown(peerID string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.peerDown[peerID]
}

// RequestVote implements VoteTransport with injected faults.
func (f *faultVoteTransport) RequestVote(ctx context.Context, peerID string, req VoteRequest) (VoteResponse, error) {
	if f.hardErrorVotes.Load() {
		atomic.AddInt64(&f.votesErrored, 1)
		return VoteResponse{}, fmt.Errorf("fault: transport hard-error on RequestVote to %s", peerID)
	}
	if f.isDown(peerID) {
		atomic.AddInt64(&f.votesErrored, 1)
		return VoteResponse{}, fmt.Errorf("fault: peer %s is down", peerID)
	}
	den := atomic.LoadInt64(&f.dropVoteDen)
	if den > 0 {
		n := atomic.AddInt64(&f.voteCalls, 1) - 1
		if n%den < atomic.LoadInt64(&f.dropVoteNum) {
			atomic.AddInt64(&f.votesDropped, 1)
			return VoteResponse{}, fmt.Errorf("fault: dropped RequestVote to %s", peerID)
		}
	}
	return f.inner.RequestVote(ctx, peerID, req)
}

// SendAppendEntries implements VoteTransport with injected faults.
func (f *faultVoteTransport) SendAppendEntries(ctx context.Context, peerID string, req AppendRequest) error {
	if f.isDown(peerID) {
		atomic.AddInt64(&f.appendsDropped, 1)
		return fmt.Errorf("fault: peer %s is down", peerID)
	}
	den := atomic.LoadInt64(&f.dropAppendDen)
	if den > 0 {
		n := atomic.AddInt64(&f.appendCalls, 1) - 1
		if n%den < atomic.LoadInt64(&f.dropAppendNum) {
			atomic.AddInt64(&f.appendsDropped, 1)
			return fmt.Errorf("fault: dropped SendAppendEntries to %s", peerID)
		}
	}
	return f.inner.SendAppendEntries(ctx, peerID, req)
}

// stats returns a snapshot of injected-fault counters for evidence capture.
func (f *faultVoteTransport) stats() map[string]int64 {
	return map[string]int64{
		"vote_calls":      atomic.LoadInt64(&f.voteCalls),
		"votes_dropped":   atomic.LoadInt64(&f.votesDropped),
		"votes_errored":   atomic.LoadInt64(&f.votesErrored),
		"append_calls":    atomic.LoadInt64(&f.appendCalls),
		"appends_dropped": atomic.LoadInt64(&f.appendsDropped),
	}
}
