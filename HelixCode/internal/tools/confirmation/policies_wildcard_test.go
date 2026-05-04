package confirmation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCondition_Wildcard_MatchesGlob(t *testing.T) {
	cond := Condition{
		ToolName: "Bash",
		Wildcard: "git status*",
	}
	req := ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "git status -sb"},
	}
	assert.True(t, cond.Matches(req))
}

func TestCondition_Wildcard_DoesNotMatch(t *testing.T) {
	cond := Condition{
		ToolName: "Bash",
		Wildcard: "git status*",
	}
	req := ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{"command": "git push origin main"},
	}
	assert.False(t, cond.Matches(req))
}

func TestCondition_Wildcard_QuestionMark(t *testing.T) {
	cond := Condition{
		ToolName: "Read",
		Wildcard: "?.go",
	}
	req := ConfirmationRequest{
		ToolName:   "Read",
		Parameters: map[string]interface{}{"path": "x.go"},
	}
	assert.True(t, cond.Matches(req))
}

func TestCondition_Wildcard_EmptyMatchesAll(t *testing.T) {
	cond := Condition{
		ToolName: "Read",
		Wildcard: "",
	}
	req := ConfirmationRequest{
		ToolName:   "Read",
		Parameters: map[string]interface{}{"path": "anything.txt"},
	}
	assert.True(t, cond.Matches(req))
}

func TestCondition_Wildcard_MissingParameterIsNoMatch(t *testing.T) {
	cond := Condition{
		ToolName: "Bash",
		Wildcard: "git status*",
	}
	req := ConfirmationRequest{
		ToolName:   "Bash",
		Parameters: map[string]interface{}{},
	}
	assert.False(t, cond.Matches(req))
}
