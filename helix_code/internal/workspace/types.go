package workspace

import (
	"errors"
	"time"
)

type WorkspaceStatus int

const (
	StatusCreating WorkspaceStatus = iota
	StatusRunning
	StatusStopped
	StatusError
)

func (s WorkspaceStatus) String() string {
	switch s {
	case StatusCreating:
		return "creating"
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

type Workspace struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	ContainerID string          `json:"container_id,omitempty"`
	Image       string          `json:"image"`
	ProjectDir  string          `json:"project_dir"`
	Status      WorkspaceStatus `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	TTL         time.Duration   `json:"-"`
}

var (
	ErrRuntimeNotFound    = errors.New("no container runtime found (docker or podman required)")
	ErrWorkspaceNotFound  = errors.New("workspace not found")
	ErrImagePullFailed    = errors.New("container image pull failed")
	ErrContainerCreateFailed = errors.New("container create failed")
	ErrContainerRunFailed    = errors.New("container run failed")
)

const (
	DefaultImage = "alpine:latest"
	DefaultTTL   = 30 * time.Minute
)
