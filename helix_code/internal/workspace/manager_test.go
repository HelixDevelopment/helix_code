package workspace

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRunner struct {
	containers map[string]ContainerInfo
	runErr     error
	stopErr    error
	removeErr  error
	listErr    error
}

func newMockRunner() *mockRunner {
	return &mockRunner{containers: make(map[string]ContainerInfo)}
}

func (m *mockRunner) Run(ctx context.Context, image, name, projectDir string) (string, error) {
	if m.runErr != nil {
		return "", m.runErr
	}
	id := "mock-" + name
	m.containers[id] = ContainerInfo{ID: id, Name: name, Image: image, State: "running"}
	return id, nil
}

func (m *mockRunner) Stop(ctx context.Context, containerID string) error {
	if m.stopErr != nil {
		return m.stopErr
	}
	if info, ok := m.containers[containerID]; ok {
		info.State = "exited"
		m.containers[containerID] = info
	}
	return nil
}

func (m *mockRunner) Remove(ctx context.Context, containerID string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	delete(m.containers, containerID)
	return nil
}

func (m *mockRunner) List(ctx context.Context) ([]ContainerInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []ContainerInfo
	for _, info := range m.containers {
		result = append(result, info)
	}
	return result, nil
}

func TestNewWorkspaceManager_RuntimeDetection(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	assert.NotNil(t, mgr)
	assert.Equal(t, RuntimeDocker, mgr.Runtime())

	_, err := NewWorkspaceManager()
	if err != nil {
		assert.ErrorIs(t, err, ErrRuntimeNotFound)
	}
}

func TestWorkspaceManager_CreateWorkspace(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	ws, err := mgr.CreateWorkspace(ctx, "test-ws", "alpine:latest", "/tmp/project")
	require.NoError(t, err)

	assert.NotEmpty(t, ws.ID)
	assert.Equal(t, "test-ws", ws.Name)
	assert.Equal(t, "alpine:latest", ws.Image)
	assert.Equal(t, StatusRunning, ws.Status)
	assert.NotEmpty(t, ws.ContainerID)
	assert.Contains(t, ws.ContainerID, "mock-test-ws")
}

func TestWorkspaceManager_CreateWorkspace_DefaultImage(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	ws, err := mgr.CreateWorkspace(ctx, "default-img", "", "/tmp/project")
	require.NoError(t, err)

	assert.Equal(t, DefaultImage, ws.Image)
}

func TestWorkspaceManager_ListWorkspaces(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	_, err := mgr.CreateWorkspace(ctx, "ws-a", "alpine", "/tmp/a")
	require.NoError(t, err)
	_, err = mgr.CreateWorkspace(ctx, "ws-b", "alpine", "/tmp/b")
	require.NoError(t, err)

	list := mgr.ListWorkspaces()
	assert.Len(t, list, 2)
}

func TestWorkspaceManager_GetWorkspace(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	ws, err := mgr.CreateWorkspace(ctx, "gotten", "alpine", "/tmp")
	require.NoError(t, err)

	got, err := mgr.GetWorkspace(ws.ID)
	require.NoError(t, err)
	assert.Equal(t, ws.ID, got.ID)
}

func TestWorkspaceManager_GetWorkspace_NotFound(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)

	_, err := mgr.GetWorkspace("nonexistent")
	assert.ErrorIs(t, err, ErrWorkspaceNotFound)
}

func TestWorkspaceManager_CleanupWorkspace(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	ws, err := mgr.CreateWorkspace(ctx, "clean-me", "alpine", "/tmp")
	require.NoError(t, err)

	err = mgr.CleanupWorkspace(ctx, ws.ID)
	require.NoError(t, err)

	// Cleanup is verified through the manager (GetWorkspace), NOT by reading the
	// CreateWorkspace return: that return is a point-in-time snapshot (taken at
	// creation), so it intentionally does not reflect the later cleanup mutation
	// — handing back the live stored pointer would be the data race the snapshot
	// getters were fixed to remove.
	_, err = mgr.GetWorkspace(ws.ID)
	assert.ErrorIs(t, err, ErrWorkspaceNotFound)
}

func TestWorkspaceManager_CleanupWorkspace_NotFound(t *testing.T) {
	mgr := NewWorkspaceManagerWithRunner(newMockRunner(), RuntimeDocker)
	ctx := context.Background()

	err := mgr.CleanupWorkspace(ctx, "nonexistent")
	assert.ErrorIs(t, err, ErrWorkspaceNotFound)
}

func TestWorkspaceManager_RunError(t *testing.T) {
	runner := newMockRunner()
	runner.runErr = ErrContainerCreateFailed
	mgr := NewWorkspaceManagerWithRunner(runner, RuntimeDocker)
	ctx := context.Background()

	ws, err := mgr.CreateWorkspace(ctx, "fail", "alpine", "/tmp")
	assert.Error(t, err)
	assert.Equal(t, StatusError, ws.Status)
}

func TestContainerInfo(t *testing.T) {
	info := ContainerInfo{
		ID:    "abc123",
		Name:  "test",
		Image: "alpine:latest",
		State: "running",
	}
	assert.Equal(t, "abc123", info.ID)
	assert.Equal(t, "test", info.Name)
	assert.Equal(t, "running", info.State)
}

func TestWorkspaceStatus_String(t *testing.T) {
	assert.Equal(t, "creating", StatusCreating.String())
	assert.Equal(t, "running", StatusRunning.String())
	assert.Equal(t, "stopped", StatusStopped.String())
	assert.Equal(t, "error", StatusError.String())
	assert.Equal(t, "unknown", WorkspaceStatus(99).String())
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrRuntimeNotFound)
	assert.Error(t, ErrWorkspaceNotFound)
	assert.Error(t, ErrImagePullFailed)
	assert.Error(t, ErrContainerCreateFailed)
	assert.Error(t, ErrContainerRunFailed)
}

func TestParseContainerList(t *testing.T) {
	output := "abc123\tmy-container\talpine:latest\trunning\n"
	result := parseContainerList(output)
	assert.Len(t, result, 1)
	assert.Equal(t, "abc123", result[0].ID)
	assert.Equal(t, "my-container", result[0].Name)
	assert.Equal(t, "alpine:latest", result[0].Image)
	assert.Equal(t, "running", result[0].State)
}

func TestParseContainerList_Empty(t *testing.T) {
	result := parseContainerList("")
	assert.Empty(t, result)
}
