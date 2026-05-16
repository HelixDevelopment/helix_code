package roocode

import "errors"

type TaskSpec struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	AssignedTo  string `json:"assigned_to,omitempty"`
}

type GenerateSpec struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Template string `json:"template"`
	Prompt   string `json:"prompt"`
}

type ReviewResult struct {
	File        string   `json:"file"`
	Issues      []string `json:"issues"`
	Suggestions []string `json:"suggestions"`
	Approved    bool     `json:"approved"`
}

type BootstrapSpec struct {
	ProjectType string `json:"project_type"`
	Name        string `json:"name"`
	OutputDir   string `json:"output_dir"`
}

var (
	ErrTaskDelegationFailed = errors.New("task delegation failed")
	ErrGenerationFailed     = errors.New("code generation failed")
	ErrReviewFailed         = errors.New("code review failed")
)
