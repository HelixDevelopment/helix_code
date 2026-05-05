package session

import (
	"context"
	"fmt"
	"sort"
)

// ResumeMode determines the scope of session resumption.
type ResumeMode string

const (
	// ResumeProject limits the search to the current project path.
	ResumeProject ResumeMode = "project"
	// ResumeGlobal searches across all stored sessions regardless of project.
	ResumeGlobal ResumeMode = "global"
)

// ResumeFinder locates resumable sessions and reads their transcripts.
type ResumeFinder struct {
	store SessionStore
}

// NewResumeFinder constructs a finder bound to the given store.
func NewResumeFinder(store SessionStore) *ResumeFinder {
	return &ResumeFinder{store: store}
}

// FindResumeTarget returns the most-recently-active session metadata for the
// given mode. ResumeProject filters by currentProject; ResumeGlobal returns
// the most recent across all sessions.
func (rf *ResumeFinder) FindResumeTarget(ctx context.Context, mode ResumeMode, currentProject string) (*SessionMetadata, error) {
	var lookup string
	if mode == ResumeProject {
		lookup = currentProject
	}
	metas, err := rf.store.ListSessionMetadata(ctx, lookup)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	if len(metas) == 0 {
		return nil, fmt.Errorf("no sessions found to resume")
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].LastActivity.After(metas[j].LastActivity)
	})
	out := metas[0]
	return &out, nil
}

// Resume loads metadata + transcript for a specific session ID.
func (rf *ResumeFinder) Resume(ctx context.Context, sessionID string) ([]Message, *SessionMetadata, error) {
	meta, err := rf.store.GetSessionMetadata(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("metadata: %w", err)
	}
	msgs, err := rf.store.ReadTranscript(ctx, sessionID)
	if err != nil {
		return nil, meta, fmt.Errorf("transcript: %w", err)
	}
	return msgs, meta, nil
}
