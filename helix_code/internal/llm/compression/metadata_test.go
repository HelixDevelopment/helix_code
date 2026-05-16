package compression

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"dev.helix.code/internal/llm/compressioniface"
)

func TestCompactionMetadata_RoundTrip(t *testing.T) {
	original := &CompactionMetadata{
		OriginalMessageCount: 50,
		OriginalTokenCount:   45_000,
		CompactedTokenCount:  5_000,
		SummarizedAt:         time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC),
		SummaryText:          "User asked about X; agent answered Y; key decision: Z.",
		TopicsCovered:        []string{"X", "Y", "Z"},
		KeyDecisions:         []string{"chose Z over W"},
	}
	msg := &compressioniface.Message{Role: compressioniface.RoleAssistant, Content: "..."}
	require.NoError(t, AttachCompactionMetadata(msg, original))
	round, ok := ReadCompactionMetadata(msg)
	require.True(t, ok)
	require.Equal(t, original.OriginalMessageCount, round.OriginalMessageCount)
	require.Equal(t, original.OriginalTokenCount, round.OriginalTokenCount)
	require.Equal(t, original.CompactedTokenCount, round.CompactedTokenCount)
	require.Equal(t, original.SummarizedAt.UTC(), round.SummarizedAt.UTC())
	require.Equal(t, original.SummaryText, round.SummaryText)
	require.Equal(t, original.TopicsCovered, round.TopicsCovered)
	require.Equal(t, original.KeyDecisions, round.KeyDecisions)
}

func TestReadCompactionMetadata_Absent(t *testing.T) {
	msg := &compressioniface.Message{Role: compressioniface.RoleAssistant, Content: "ordinary message"}
	_, ok := ReadCompactionMetadata(msg)
	require.False(t, ok)
}

func TestAttachCompactionMetadata_RejectsNonAssistantRole(t *testing.T) {
	msg := &compressioniface.Message{Role: compressioniface.RoleUser, Content: "..."}
	err := AttachCompactionMetadata(msg, &CompactionMetadata{})
	require.Error(t, err, "metadata is for assistant messages only")
}
