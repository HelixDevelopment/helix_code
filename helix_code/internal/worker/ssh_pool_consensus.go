package worker

import (
	"context"
	"fmt"
	"time"
)

// Distributed leader election wiring for the SSH worker pool.
//
// W6B forensic (root cause): NewSSHWorkerPoolWithConfig constructs the pool's
// ConsensusManager with Peers:[]string{} and NO VoteTransport, and AddWorker
// never wires the consensus peer set. The result is that every pool ran as an
// isolated single-node cluster — it elected itself leader and never canvassed
// any peer, so the W4B multi-peer election machinery (VoteTransport +
// InProcessCluster + bounded-safety step-down) was latent: present in the
// state machine but never reached because no transport was ever injected.
//
// This file wires a REAL VoteTransport into the pool so a genuine multi-node
// election runs across the pool's configured peers.
//
// Transport choice + tradeoffs (§11.4.101 autonomous-safe decision):
//   - A full network vote RPC (SSH-exec or gRPC/HTTP vote endpoint per worker)
//     is the eventual production transport, but it is a large multi-file
//     networking effort: it needs a vote-RPC server listening on every remote
//     helix worker, request/response framing over the wire, and a remote daemon
//     that does not yet exist. Shipping a half-built network stub would be a
//     §11.4 bluff.
//   - Instead the pool is fully wired to construct + inject a working,
//     in-process-compatible transport (InProcessCluster — a real, fully
//     implemented VoteTransport, not a mock) from its actual peer set, so
//     multi-node election genuinely runs THROUGH the pool today. The network
//     layer is a documented, typed seam (RemoteVoteTransport below) that
//     implements the SAME VoteTransport interface and can be injected via
//     SSHWorkerPool.SetConsensusTransport without touching the state machine.
//
// Follow-up (tracked): implement RemoteVoteTransport over the pool's existing
// SSH plumbing (a vote-RPC `helix consensus vote` exec, or a small gRPC vote
// endpoint) and inject it via SetConsensusTransport in place of the in-process
// transport for cross-host deployments. See RemoteVoteTransport docstring.

// GetConsensusNodeID returns the pool's consensus node identity.
func (p *SSHWorkerPool) GetConsensusNodeID() string {
	if p.consensus == nil {
		return ""
	}
	return p.consensus.GetNodeID()
}

// IsConsensusLeader reports whether this pool is the current consensus leader.
func (p *SSHWorkerPool) IsConsensusLeader() bool {
	if p.consensus == nil {
		return false
	}
	return p.consensus.IsLeader()
}

// GetConsensusLeaderID returns the consensus leader's node ID as known to this
// pool ("" before the first heartbeat / election outcome is observed).
func (p *SSHWorkerPool) GetConsensusLeaderID() string {
	if p.consensus == nil {
		return ""
	}
	return p.consensus.GetLeader()
}

// GetConsensusTerm returns the current consensus term observed by this pool.
func (p *SSHWorkerPool) GetConsensusTerm() int {
	if p.consensus == nil {
		return 0
	}
	return p.consensus.GetCurrentTerm()
}

// StopConsensus stops the pool's consensus protocol (timers + loop). Safe to
// call on a pool whose consensus was never started.
func (p *SSHWorkerPool) StopConsensus() {
	if p.consensus != nil {
		p.consensus.Stop()
	}
}

// SetConsensusPeers reconfigures the pool's consensus manager with an explicit
// peer set + VoteTransport, promoting it from single-node to a genuine
// multi-node cluster member. The transport MUST be able to route
// RequestVote / SendAppendEntries to each peerID in peers.
func (p *SSHWorkerPool) SetConsensusTransport(peers []string, transport VoteTransport) error {
	if p.consensus == nil {
		return fmt.Errorf("pool has no consensus manager to configure")
	}
	if transport == nil {
		return fmt.Errorf("a non-nil VoteTransport is required to run multi-node election (single-node pools need no wiring)")
	}
	p.consensus.Reconfigure(peers, transport)
	return nil
}

// WireConsensusCluster joins the given SSH worker pools into a single, real
// multi-node consensus cluster over a shared in-process VoteTransport, so a
// genuine leader election runs across them THROUGH each pool. It is the
// single-process / co-located-pool wiring (e.g. multiple pools in one server,
// or an integration topology) — the network equivalent injects a
// RemoteVoteTransport via SetConsensusTransport instead.
//
// Each pool's ConsensusManager is registered with a shared InProcessCluster and
// reconfigured with the other pools' node IDs as its peer set + that cluster as
// its transport. After this returns, exactly one pool wins leadership within a
// bounded election window and the others recognise it (heartbeats fan out over
// the shared transport).
//
// heartbeat is the heartbeat cadence for every pool; staggered election
// timeouts are derived from it so one node reliably starts first and wins (Raft
// timing discipline: heartbeat << election timeout). Passing 0 uses a sane
// default.
func WireConsensusCluster(heartbeat time.Duration, pools ...*SSHWorkerPool) error {
	if len(pools) < 2 {
		return fmt.Errorf("a multi-node cluster needs at least 2 pools, got %d", len(pools))
	}
	if heartbeat <= 0 {
		heartbeat = 50 * time.Millisecond
	}

	cluster := NewInProcessCluster()

	// Collect node IDs and register every manager with the shared transport.
	ids := make([]string, 0, len(pools))
	for _, p := range pools {
		if p.consensus == nil {
			return fmt.Errorf("pool %p has no consensus manager", p)
		}
		id := p.consensus.GetNodeID()
		if id == "" {
			return fmt.Errorf("pool %p has an empty consensus node ID", p)
		}
		ids = append(ids, id)
		cluster.Register(p.consensus)
	}

	// Reconfigure each pool's manager with the OTHER pools as peers + the shared
	// transport, and stagger the election timeouts so one node starts first.
	for i, p := range pools {
		peers := make([]string, 0, len(ids)-1)
		for j, id := range ids {
			if i != j {
				peers = append(peers, id)
			}
		}
		// Stagger: election timeout must be well above the heartbeat so a
		// stable leader's heartbeats reset followers' timers before they fire.
		electionTimeout := heartbeat*5 + time.Duration(i)*heartbeat*4
		p.consensus.electionTimeout = electionTimeout
		p.consensus.heartbeatInterval_ = heartbeat
		if err := p.SetConsensusTransport(peers, cluster); err != nil {
			return fmt.Errorf("failed to wire pool %d into the consensus cluster: %w", i, err)
		}
	}

	return nil
}

// RemoteVoteTransport is the documented network-layer seam for distributed
// leader election across SEPARATE hosts (the production cross-host transport).
// It implements the SAME VoteTransport interface as InProcessCluster, so the
// consensus state machine is unchanged — only the wire differs.
//
// STATUS (§11.4.6 honest): NOT YET IMPLEMENTED. The methods return an explicit
// not-implemented error rather than a fabricated success, so any caller that
// injects it before the network layer lands gets an honest failure (which the
// election treats as "peer unreachable / vote not granted") instead of a silent
// bluff. The co-located path (WireConsensusCluster over InProcessCluster) is the
// fully-working transport shipped in this change; this type marks exactly where
// the SSH/gRPC vote RPC plugs in.
//
// Follow-up implementation plan (tracked): RequestVote should open an SSH
// session to peerEndpoints[peerID] and run a `helix consensus vote` RPC (JSON
// VoteRequest in, JSON VoteResponse out), OR call a small gRPC/HTTP vote
// endpoint; SendAppendEntries should deliver the heartbeat the same way. Both
// already have receiver-side handlers on ConsensusManager
// (HandleVoteRequest / HandleAppendRequest) — only the wire transport is
// missing.
type RemoteVoteTransport struct {
	// peerEndpoints maps a consensus nodeID to the network address of that
	// peer's vote endpoint (host:port). Populated by the caller from the pool's
	// worker set when the network transport is implemented.
	peerEndpoints map[string]string
}

// NewRemoteVoteTransport constructs the (not-yet-implemented) network transport
// with the given nodeID→endpoint routing table.
func NewRemoteVoteTransport(peerEndpoints map[string]string) *RemoteVoteTransport {
	cp := make(map[string]string, len(peerEndpoints))
	for k, v := range peerEndpoints {
		cp[k] = v
	}
	return &RemoteVoteTransport{peerEndpoints: cp}
}

// errRemoteTransportNotImplemented is the honest not-yet-wired error returned by
// every RemoteVoteTransport method. Returning it (rather than a fabricated vote)
// keeps the election correct: an errored peer counts as "vote not granted".
var errRemoteTransportNotImplemented = fmt.Errorf("worker: RemoteVoteTransport (cross-host SSH/gRPC vote RPC) is not yet implemented — use WireConsensusCluster (in-process transport) for co-located pools; tracked follow-up wires this over the pool's SSH plumbing")

// RequestVote is the not-yet-implemented network vote RPC (see type docstring).
func (r *RemoteVoteTransport) RequestVote(ctx context.Context, peerID string, req VoteRequest) (VoteResponse, error) {
	return VoteResponse{}, errRemoteTransportNotImplemented
}

// SendAppendEntries is the not-yet-implemented network heartbeat RPC.
func (r *RemoteVoteTransport) SendAppendEntries(ctx context.Context, peerID string, req AppendRequest) error {
	return errRemoteTransportNotImplemented
}
