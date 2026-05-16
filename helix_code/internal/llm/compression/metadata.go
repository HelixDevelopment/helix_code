package compression

import (
	"encoding/json"
	"errors"
	"time"

	"dev.helix.code/internal/llm/compressioniface"
)

// MetadataKey is the key under compressioniface.MessageMetadata.Extra where
// the CompactionMetadata blob is stored (JSON-encoded). Stable across versions
// to allow forensic tooling to find compaction summaries.
const MetadataKey = "context_management"

// ErrInvalidMessageRole is returned when AttachCompactionMetadata is given
// a message whose role is not "assistant".
var ErrInvalidMessageRole = errors.New("compaction metadata is for assistant messages only")

// CompactionMetadata captures the forensic record of a single compaction
// event. It is attached to the assistant message that replaces the
// summarised portion of the conversation history.
type CompactionMetadata struct {
	OriginalMessageCount int       `json:"original_message_count"`
	OriginalTokenCount   int       `json:"original_token_count"`
	CompactedTokenCount  int       `json:"compacted_token_count"`
	SummarizedAt         time.Time `json:"summarized_at"`
	SummaryText          string    `json:"summary_text"`
	TopicsCovered        []string  `json:"topics_covered,omitempty"`
	KeyDecisions         []string  `json:"key_decisions,omitempty"`
}

// AttachCompactionMetadata serialises m as JSON and stores it under
// MetadataKey in msg.Metadata.Extra. The map is created if nil.
// Returns ErrInvalidMessageRole if the message role is not "assistant".
func AttachCompactionMetadata(msg *compressioniface.Message, m *CompactionMetadata) error {
	if msg == nil {
		return errors.New("nil message")
	}
	if msg.Role != compressioniface.RoleAssistant {
		return ErrInvalidMessageRole
	}
	if m == nil {
		return errors.New("nil metadata")
	}
	encoded, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if msg.Metadata.Extra == nil {
		msg.Metadata.Extra = make(map[string]interface{})
	}
	msg.Metadata.Extra[MetadataKey] = string(encoded)
	return nil
}

// ReadCompactionMetadata returns the CompactionMetadata stored on the
// message, or (nil, false) if absent or unparseable.
func ReadCompactionMetadata(msg *compressioniface.Message) (*CompactionMetadata, bool) {
	if msg == nil {
		return nil, false
	}
	if msg.Metadata.Extra == nil {
		return nil, false
	}
	raw, ok := msg.Metadata.Extra[MetadataKey]
	if !ok {
		return nil, false
	}
	str, ok := raw.(string)
	if !ok {
		return nil, false
	}
	var cm CompactionMetadata
	if err := json.Unmarshal([]byte(str), &cm); err != nil {
		return nil, false
	}
	return &cm, true
}
