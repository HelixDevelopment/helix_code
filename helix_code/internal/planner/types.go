package planner

import (
	"errors"
	"time"
)

type StepType int

const (
	StepShell StepType = iota
	StepLLM
)

func (s StepType) String() string {
	switch s {
	case StepShell:
		return "shell"
	case StepLLM:
		return "llm"
	default:
		return "unknown"
	}
}

type StepStatus int

const (
	StepPending StepStatus = iota
	StepRunning
	StepCompleted
	StepFailed
)

func (s StepStatus) String() string {
	switch s {
	case StepPending:
		return "pending"
	case StepRunning:
		return "running"
	case StepCompleted:
		return "completed"
	case StepFailed:
		return "failed"
	default:
		return "unknown"
	}
}

type TaskStep struct {
	ID          string        `json:"id"`
	PlanNodeID  string        `json:"plan_node_id"`
	Type        StepType      `json:"type"`
	Command     string        `json:"command,omitempty"`
	Prompt      string        `json:"prompt,omitempty"`
	Timeout     time.Duration `json:"-"`
	MaxRetries  int           `json:"max_retries"`
	Status      StepStatus    `json:"status"`
	Output      string        `json:"output,omitempty"`
	Error       string        `json:"error,omitempty"`
	RetryCount  int           `json:"retry_count"`
	StartedAt   time.Time     `json:"started_at,omitempty"`
	CompletedAt time.Time     `json:"completed_at,omitempty"`
}

type PlanStatus int

const (
	PlanStatusPending PlanStatus = iota
	PlanStatusRunning
	PlanStatusCompleted
	PlanStatusFailed
)

type TaskPlan struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Steps       []TaskStep `json:"steps"`
	Status      PlanStatus `json:"status"`
	CurrentStep int        `json:"current_step"`
}

var (
	ErrStepTimeout   = errors.New("step execution timed out")
	ErrMaxRetries    = errors.New("step exceeded maximum retries")
	ErrPlanComplete  = errors.New("plan already complete")
	ErrPlanFailed    = errors.New("plan has failed steps")
	ErrInvalidStep   = errors.New("invalid step configuration")
)

const (
	DefaultTimeout  = 5 * time.Minute
	DefaultRetries  = 3
	MaxStepOutput   = 64 * 1024
)
