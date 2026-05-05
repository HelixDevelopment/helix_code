package session

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Message represents a single conversation turn stored in a session transcript.
// Role is typically "user", "assistant", or "system".
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// SessionMetadata is the JSON-on-disk shape stored alongside the JSONL
// transcript. It is per-session, not per-message.
type SessionMetadata struct {
	SessionID    string    `json:"session_id"`
	ProjectPath  string    `json:"project_path"`
	ProjectName  string    `json:"project_name"`
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int       `json:"message_count"`
	IsActive     bool      `json:"is_active"`
	BranchName   string    `json:"branch_name,omitempty"`
}

// SessionStore is the interface for session transcript and metadata persistence.
// Implementations persist messages as JSONL lines and metadata as JSON sidecars.
type SessionStore interface {
	// ListSessionMetadata lists metadata for all known sessions. When projectPath
	// is non-empty only sessions with a matching ProjectPath are returned.
	ListSessionMetadata(ctx context.Context, projectPath string) ([]SessionMetadata, error)

	// GetSessionMetadata returns the metadata for a single session. If the
	// metadata.json sidecar is absent it is resynthesised from the JSONL
	// transcript and persisted before returning.
	GetSessionMetadata(ctx context.Context, sessionID string) (*SessionMetadata, error)

	// UpdateSessionMetadata writes (or overwrites) the metadata.json sidecar.
	UpdateSessionMetadata(ctx context.Context, meta SessionMetadata) error

	// Append appends a single message to the JSONL transcript for sessionID.
	Append(ctx context.Context, sessionID string, msg Message) error

	// ReadTranscript reads all messages from the JSONL transcript. Corrupt
	// lines are silently skipped; scanner errors are returned.
	ReadTranscript(ctx context.Context, sessionID string) ([]Message, error)

	// DeleteSession removes the entire session directory (transcript + metadata).
	DeleteSession(ctx context.Context, sessionID string) error
}

// TranscriptStore implements SessionStore using plain files on disk.
// Layout: <baseDir>/<sessionID>/transcript.jsonl
//
//	<baseDir>/<sessionID>/metadata.json
type TranscriptStore struct {
	baseDir string
}

// NewTranscriptStore returns a TranscriptStore rooted at baseDir. The
// directory is created on first use; it need not exist at construction time.
func NewTranscriptStore(baseDir string) *TranscriptStore {
	return &TranscriptStore{baseDir: baseDir}
}

// Compile-time interface assertion.
var _ SessionStore = (*TranscriptStore)(nil)

// sessionDir returns the directory for a given sessionID.
func (s *TranscriptStore) sessionDir(id string) string {
	return filepath.Join(s.baseDir, id)
}

// Append appends msg to <baseDir>/<sessionID>/transcript.jsonl and updates the
// metadata.json sidecar (MessageCount + LastActivity). The metadata sidecar is
// created with defaults on the first append so that it always exists while a
// transcript file exists.
func (s *TranscriptStore) Append(ctx context.Context, sessionID string, msg Message) error {
	dir := s.sessionDir(sessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("transcript_store: mkdir %s: %w", dir, err)
	}

	// Write the JSONL line.
	path := filepath.Join(dir, "transcript.jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("transcript_store: open %s: %w", path, err)
	}
	defer f.Close()

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("transcript_store: marshal message: %w", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("transcript_store: write %s: %w", path, err)
	}
	f.Close() // close before re-reading for count

	// Keep the metadata sidecar in sync. Read the current metadata if it
	// exists, otherwise start with defaults; then increment the count.
	metaPath := filepath.Join(dir, "metadata.json")
	var meta SessionMetadata
	if raw, readErr := os.ReadFile(metaPath); readErr == nil {
		_ = json.Unmarshal(raw, &meta) // ignore unmarshal error — rebuild below
	}
	if meta.SessionID == "" {
		meta.SessionID = sessionID
		meta.StartedAt = time.Now().UTC()
	}
	meta.LastActivity = time.Now().UTC()

	// Count valid lines in the JSONL file to get an accurate MessageCount.
	msgs, countErr := s.ReadTranscript(ctx, sessionID)
	if countErr == nil {
		meta.MessageCount = len(msgs)
	} else {
		meta.MessageCount++
	}

	encoded, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("transcript_store: marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, encoded, 0644); err != nil {
		return fmt.Errorf("transcript_store: write metadata %s: %w", metaPath, err)
	}
	return nil
}

// ReadTranscript reads all messages from <baseDir>/<sessionID>/transcript.jsonl.
// An absent transcript file is treated as an empty transcript (no error).
// Corrupt lines are silently skipped.
func (s *TranscriptStore) ReadTranscript(ctx context.Context, sessionID string) ([]Message, error) {
	path := filepath.Join(s.sessionDir(sessionID), "transcript.jsonl")
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("transcript_store: open %s: %w", path, err)
	}
	defer f.Close()

	var msgs []Message
	scanner := bufio.NewScanner(f)
	// Allow lines up to 16 MiB (large tool outputs can be verbose).
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var m Message
		if err := json.Unmarshal(line, &m); err != nil {
			// Skip corrupt lines — do not abort the whole read.
			continue
		}
		msgs = append(msgs, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("transcript_store: scan %s: %w", path, err)
	}
	return msgs, nil
}

// UpdateSessionMetadata writes meta as pretty-printed JSON to
// <baseDir>/<meta.SessionID>/metadata.json, creating the directory if needed.
func (s *TranscriptStore) UpdateSessionMetadata(ctx context.Context, meta SessionMetadata) error {
	dir := s.sessionDir(meta.SessionID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("transcript_store: mkdir %s: %w", dir, err)
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("transcript_store: marshal metadata: %w", err)
	}
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("transcript_store: write %s: %w", path, err)
	}
	return nil
}

// GetSessionMetadata returns the persisted metadata for sessionID.
// If metadata.json is absent the metadata is resynthesised from the JSONL
// transcript (MessageCount is set to the number of valid lines) and the
// resynthesised metadata is written back so subsequent calls are fast.
func (s *TranscriptStore) GetSessionMetadata(ctx context.Context, sessionID string) (*SessionMetadata, error) {
	path := filepath.Join(s.sessionDir(sessionID), "metadata.json")
	data, err := os.ReadFile(path)
	if err == nil {
		var meta SessionMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, fmt.Errorf("transcript_store: unmarshal %s: %w", path, err)
		}
		return &meta, nil
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("transcript_store: read %s: %w", path, err)
	}

	// metadata.json is absent — resynthesise from the JSONL transcript.
	msgs, rerr := s.ReadTranscript(ctx, sessionID)
	if rerr != nil {
		return nil, fmt.Errorf("transcript_store: metadata absent and transcript read failed: %w", rerr)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("transcript_store: session %s not found (no metadata, no transcript)", sessionID)
	}

	now := time.Now().UTC()
	meta := SessionMetadata{
		SessionID:    sessionID,
		MessageCount: len(msgs),
		StartedAt:    now,
		LastActivity: now,
	}
	// Best-effort write-back; ignore error so the caller still gets the data.
	_ = s.UpdateSessionMetadata(ctx, meta)
	return &meta, nil
}

// ListSessionMetadata returns metadata for all sessions whose directories exist
// under baseDir. When projectPath is non-empty only sessions with a matching
// ProjectPath are included. Sessions whose metadata cannot be read are silently
// skipped.
func (s *TranscriptStore) ListSessionMetadata(ctx context.Context, projectPath string) ([]SessionMetadata, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("transcript_store: readdir %s: %w", s.baseDir, err)
	}

	var out []SessionMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		meta, err := s.GetSessionMetadata(ctx, entry.Name())
		if err != nil {
			continue // skip unreadable/empty session dirs
		}
		if projectPath != "" && meta.ProjectPath != projectPath {
			continue
		}
		out = append(out, *meta)
	}
	return out, nil
}

// DeleteSession removes the entire session directory (transcript + metadata).
func (s *TranscriptStore) DeleteSession(ctx context.Context, sessionID string) error {
	dir := s.sessionDir(sessionID)
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("transcript_store: remove %s: %w", dir, err)
	}
	return nil
}
