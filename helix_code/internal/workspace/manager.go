package workspace

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ContainerRuntime string

const (
	RuntimeDocker ContainerRuntime = "docker"
	RuntimePodman ContainerRuntime = "podman"
)

type ContainerRunner interface {
	Run(ctx context.Context, image, name, projectDir string) (containerID string, err error)
	Stop(ctx context.Context, containerID string) error
	Remove(ctx context.Context, containerID string) error
	List(ctx context.Context) ([]ContainerInfo, error)
}

type ContainerInfo struct {
	ID    string
	Name  string
	Image string
	State string
}

type dockerRunner struct{}

func (d *dockerRunner) Run(ctx context.Context, image, name, projectDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "run", "-d",
		"--name", name,
		"-v", fmt.Sprintf("%s:/workspace", projectDir),
		"-w", "/workspace",
		image, "tail", "-f", "/dev/null")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker run: %w: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *dockerRunner) Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "stop", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker stop: %w: %s", err, out)
	}
	return nil
}

func (d *dockerRunner) Remove(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker rm: %w: %s", err, out)
	}
	return nil
}

func (d *dockerRunner) List(ctx context.Context) ([]ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "-a",
		"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}
	return parseContainerList(string(out)), nil
}

type podmanRunner struct{}

func (p *podmanRunner) Run(ctx context.Context, image, name, projectDir string) (string, error) {
	cmd := exec.CommandContext(ctx, "podman", "run", "-d",
		"--name", name,
		"-v", fmt.Sprintf("%s:/workspace:Z", projectDir),
		"-w", "/workspace",
		image, "tail", "-f", "/dev/null")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("podman run: %w: %s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *podmanRunner) Stop(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "podman", "stop", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman stop: %w: %s", err, out)
	}
	return nil
}

func (p *podmanRunner) Remove(ctx context.Context, containerID string) error {
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", containerID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("podman rm: %w: %s", err, out)
	}
	return nil
}

func (p *podmanRunner) List(ctx context.Context) ([]ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, "podman", "ps", "-a",
		"--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.State}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}
	return parseContainerList(string(out)), nil
}

func parseContainerList(output string) []ContainerInfo {
	var info []ContainerInfo
	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) >= 4 {
			info = append(info, ContainerInfo{
				ID:    parts[0],
				Name:  parts[1],
				Image: parts[2],
				State: parts[3],
			})
		}
	}
	return info
}

type WorkspaceManager struct {
	mu      sync.RWMutex
	runner  ContainerRunner
	spaces  map[string]*Workspace
	runtime ContainerRuntime
}

func NewWorkspaceManager() (*WorkspaceManager, error) {
	var runner ContainerRunner
	var runtime ContainerRuntime

	if _, err := exec.LookPath("docker"); err == nil {
		runner = &dockerRunner{}
		runtime = RuntimeDocker
	} else if _, err := exec.LookPath("podman"); err == nil {
		runner = &podmanRunner{}
		runtime = RuntimePodman
	} else {
		return nil, ErrRuntimeNotFound
	}

	return &WorkspaceManager{
		runner:  runner,
		spaces:  make(map[string]*Workspace),
		runtime: runtime,
	}, nil
}

func NewWorkspaceManagerWithRunner(runner ContainerRunner, runtime ContainerRuntime) *WorkspaceManager {
	return &WorkspaceManager{
		runner:  runner,
		spaces:  make(map[string]*Workspace),
		runtime: runtime,
	}
}

func (m *WorkspaceManager) CreateWorkspace(ctx context.Context, name, image, projectDir string) (*Workspace, error) {
	if image == "" {
		image = DefaultImage
	}

	ws := &Workspace{
		ID:         uuid.New().String(),
		Name:       name,
		Image:      image,
		ProjectDir: projectDir,
		Status:     StatusCreating,
		CreatedAt:  time.Now().UTC(),
		TTL:        DefaultTTL,
	}

	m.mu.Lock()
	m.spaces[ws.ID] = ws
	m.mu.Unlock()

	containerID, err := m.runner.Run(ctx, image, name, projectDir)
	if err != nil {
		// HXC-WS-RACE: ws is stored in m.spaces and may be read
		// concurrently via ListWorkspaces/GetWorkspace (live-pointer
		// escape). All post-Run field mutations MUST be guarded by the
		// write lock — otherwise the assignment races a concurrent
		// reader (caught by `go test -race`, a §11.4.85(B)
		// state-corruption defect).
		m.mu.Lock()
		ws.Status = StatusError
		cp := *ws // snapshot under the lock — never hand back the live stored pointer
		m.mu.Unlock()
		return &cp, fmt.Errorf("create container: %w", err)
	}

	m.mu.Lock()
	ws.ContainerID = containerID
	ws.Status = StatusRunning
	// Return a SNAPSHOT, not the live stored pointer: a caller that retains the
	// returned *Workspace and reads its fields would otherwise race a concurrent
	// CleanupWorkspace mutating the same struct under the lock (same race class
	// the getters were fixed for). Workspace is all value fields, so *ws is a
	// complete copy. Callers read fields immediately post-create; none rely on
	// observing later mutations through the returned pointer.
	cp := *ws
	m.mu.Unlock()

	return &cp, nil
}

// ListWorkspaces returns a point-in-time SNAPSHOT of every workspace.
//
// HXC-WS-RACE: it MUST return COPIES, never the live *Workspace pointers
// stored in m.spaces. Handing out a live stored pointer (the bug fixed
// here) lets a caller read ws.Status/ws.ContainerID without the lock while
// CreateWorkspace mutates the same struct — a data race caught by
// `go test -race`, a §11.4.85(B) state-corruption defect. The same race
// class was fixed in internal/project: a getter that hands out the live
// stored pointer a writer mutates.
func (m *WorkspaceManager) ListWorkspaces() []*Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Workspace, 0, len(m.spaces))
	for _, ws := range m.spaces {
		cp := *ws
		result = append(result, &cp)
	}
	return result
}

// GetWorkspace returns a COPY of the workspace, never the live stored
// pointer (HXC-WS-RACE — see ListWorkspaces).
func (m *WorkspaceManager) GetWorkspace(id string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ws, ok := m.spaces[id]
	if !ok {
		return nil, ErrWorkspaceNotFound
	}
	cp := *ws
	return &cp, nil
}

func (m *WorkspaceManager) CleanupWorkspace(ctx context.Context, id string) error {
	// HXC-WS-RACE: snapshot the container ID under the lock; reading
	// ws.ContainerID off the live stored pointer without the lock races
	// CreateWorkspace's post-Run mutation.
	m.mu.Lock()
	ws, ok := m.spaces[id]
	var containerID string
	if ok {
		containerID = ws.ContainerID
	}
	m.mu.Unlock()

	if !ok {
		return ErrWorkspaceNotFound
	}

	if containerID != "" {
		if err := m.runner.Stop(ctx, containerID); err != nil {
			return fmt.Errorf("stop container: %w", err)
		}
		if err := m.runner.Remove(ctx, containerID); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}

	// HXC-WS-RACE: the status mutation MUST be guarded by the write lock
	// — it shares the struct with concurrent readers until the delete.
	m.mu.Lock()
	ws.Status = StatusStopped
	delete(m.spaces, id)
	m.mu.Unlock()

	return nil
}

func (m *WorkspaceManager) Runtime() ContainerRuntime {
	return m.runtime
}
