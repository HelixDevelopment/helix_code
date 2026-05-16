package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCommands_SingleCommand(t *testing.T) {
	cmds, err := SplitCommands("git status -sb")
	require.NoError(t, err)
	assert.Equal(t, []string{"git status -sb"}, cmds)
}

func TestSplitCommands_AndChain(t *testing.T) {
	cmds, err := SplitCommands("ls && git push")
	require.NoError(t, err)
	assert.Equal(t, []string{"ls", "git push"}, cmds)
}

func TestSplitCommands_OrChainAndSemicolon(t *testing.T) {
	cmds, err := SplitCommands("ls || cat readme; rm tmp")
	require.NoError(t, err)
	assert.Equal(t, []string{"ls", "cat readme", "rm tmp"}, cmds)
}

func TestSplitCommands_Pipeline(t *testing.T) {
	cmds, err := SplitCommands("cat foo | grep bar | wc -l")
	require.NoError(t, err)
	assert.Equal(t, []string{"cat foo", "grep bar", "wc -l"}, cmds)
}

func TestSplitCommands_CommandSubstitution_DollarParen(t *testing.T) {
	cmds, err := SplitCommands("echo $(rm -rf /tmp/x)")
	require.NoError(t, err)
	assert.Contains(t, cmds, "rm -rf /tmp/x")
	assert.Contains(t, cmds, "echo")
}

func TestSplitCommands_CommandSubstitution_Backticks(t *testing.T) {
	cmds, err := SplitCommands("echo `rm -rf /tmp/x`")
	require.NoError(t, err)
	assert.Contains(t, cmds, "rm -rf /tmp/x")
}

func TestSplitCommands_QuotedOperatorIsLiteral(t *testing.T) {
	cmds, err := SplitCommands(`echo "foo && bar"`)
	require.NoError(t, err)
	assert.Equal(t, 1, len(cmds), "quoted && must NOT be split")
	assert.Contains(t, cmds[0], `foo && bar`)
}

func TestSplitCommands_Heredoc(t *testing.T) {
	input := "cat <<EOF\nhello\nEOF\nrm /tmp/x"
	cmds, err := SplitCommands(input)
	require.NoError(t, err)
	assert.Contains(t, cmds, "rm /tmp/x")
}

func TestSplitCommands_MalformedReturnsError(t *testing.T) {
	_, err := SplitCommands(`echo "unclosed`)
	require.Error(t, err)
}

func TestSplitCommands_EmptyInput(t *testing.T) {
	cmds, err := SplitCommands("")
	require.NoError(t, err)
	assert.Empty(t, cmds)
}
