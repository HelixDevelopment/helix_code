package hooks

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockers_NilSlice(t *testing.T) {
	assert.Nil(t, Blockers(nil))
}

func TestBlockers_AllSucceeded(t *testing.T) {
	results := []*ExecutionResult{
		{HookID: "a", Status: StatusSucceeded, Error: nil},
		{HookID: "b", Status: StatusSucceeded, Error: nil},
	}
	assert.Empty(t, Blockers(results))
}

func TestBlockers_OneFailed(t *testing.T) {
	results := []*ExecutionResult{
		{HookID: "a", Status: StatusSucceeded, Error: nil},
		{HookID: "b", Status: StatusFailed, Error: errors.New("nope")},
	}
	got := Blockers(results)
	assert.Len(t, got, 1)
	assert.Contains(t, got[0].Error(), "nope")
}

func TestBlockers_MultipleFailed_PreservesOrder(t *testing.T) {
	results := []*ExecutionResult{
		{HookID: "a", Status: StatusFailed, Error: errors.New("first")},
		{HookID: "b", Status: StatusSucceeded, Error: nil},
		{HookID: "c", Status: StatusFailed, Error: errors.New("second")},
	}
	got := Blockers(results)
	assert.Len(t, got, 2)
	assert.Contains(t, got[0].Error(), "first")
	assert.Contains(t, got[1].Error(), "second")
}

func TestBlockers_NilResultEntryIsSkipped(t *testing.T) {
	results := []*ExecutionResult{
		nil,
		{HookID: "a", Status: StatusFailed, Error: errors.New("real")},
	}
	got := Blockers(results)
	assert.Len(t, got, 1)
}
