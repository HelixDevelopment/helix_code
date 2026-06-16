// Remote-distribution extension of the containers adapter.
//
// adapter.go does LOCAL compose only (ComposeUp / BootAll). This file
// adds the REMOTE-mode path mandated by §11.4.76 (Containers-Submodule
// Mandate): it loads the CONST-045 distribution config from the
// containers submodule's .env via pkg/envconfig, detects
// CONTAINERS_REMOTE_ENABLED, and routes a compose-file deployment to a
// remote host through the submodule's pkg/remote SSH executor +
// RemoteComposeOrchestrator — never a hand-rolled `podman compose up`.
//
// Reference pattern: HelixAgent's NewAdapterFromConfig (containers
// submodule CLAUDE.md "Composition" + "Mandatory Container Orchestration
// Flow"): load Containers/.env -> detect remote-enabled -> build
// SSHExecutor + HostManager -> RemoteComposeUp SCPs the compose file +
// build contexts to the host + runs `podman compose -f <file> up -d`
// remotely.
//
// Authority: §11.4.76 (extend the submodule, never reimplement) +
// CONST-045 (no hardcoded distribution hosts — all from .env).
package containers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
)

// remoteState holds the loaded distribution configuration and the SSH
// executor used to drive remote compose. It is nil until
// LoadRemoteConfig succeeds with CONTAINERS_REMOTE_ENABLED=true.
type remoteState struct {
	cfg      *envconfig.DistributionConfig
	hosts    []remote.RemoteHost
	executor *remote.SSHExecutor
	logger   logging.Logger
}

// LoadRemoteConfig loads the CONST-045 distribution config from the
// given .env path (typically <repo-root>/submodules/containers/.env),
// and — if CONTAINERS_REMOTE_ENABLED=true — builds the SSH executor and
// registers every configured remote host on the adapter.
//
// It returns (enabled, err). When enabled is false the adapter stays in
// local-only mode and ComposeUp/BootAll behave exactly as before. A
// missing .env is reported as (false, error) so callers can fall back
// to local mode without aborting.
//
// §11.4.76: the executor + host construction reuse the submodule's
// pkg/envconfig + pkg/remote — the adapter wires, it does not
// reimplement the SSH/compose machinery.
func (a *Adapter) LoadRemoteConfig(envPath string) (bool, error) {
	cfg, err := envconfig.LoadFromFile(envPath)
	if err != nil {
		return false, fmt.Errorf("load distribution config %s: %w", envPath, err)
	}
	if !cfg.Enabled {
		return false, nil
	}
	if len(cfg.Hosts) == 0 {
		return false, fmt.Errorf(
			"%s has CONTAINERS_REMOTE_ENABLED=true but no CONTAINERS_REMOTE_HOST_N_* entries",
			envPath,
		)
	}

	logger := logging.NewSlogAdapter(nil)

	// Build the SSH executor from the submodule's pkg/remote with the
	// .env-derived timeouts. ControlMaster pooling massively reduces
	// per-command latency for the SCP + compose sequence below.
	exec, err := remote.NewSSHExecutor(
		logger,
		remote.WithConnectTimeout(time.Duration(cfg.ConnectTimeout)*time.Second),
		remote.WithCommandTimeout(time.Duration(cfg.CommandTimeout)*time.Second),
		remote.WithControlMaster(cfg.ControlMasterEnabled),
		remote.WithControlPersist(time.Duration(cfg.ControlPersist)*time.Second),
		remote.WithMaxConnections(cfg.MaxConnections),
	)
	if err != nil {
		return false, fmt.Errorf("create SSH executor: %w", err)
	}

	// ToRemoteHosts applies the DEFAULT_* fallbacks; we additionally
	// expand a leading ~ in the SSH key path because `ssh -i` / `scp -i`
	// are exec'd WITHOUT a shell and therefore never tilde-expand.
	hosts := cfg.ToRemoteHosts()
	for i := range hosts {
		hosts[i].KeyPath = expandHome(hosts[i].KeyPath)
	}

	a.mu.Lock()
	a.remote = &remoteState{
		cfg:      cfg,
		hosts:    hosts,
		executor: exec,
		logger:   logger,
	}
	a.mu.Unlock()
	return true, nil
}

// RemoteEnabled reports whether remote distribution is active (config
// loaded with CONTAINERS_REMOTE_ENABLED=true and at least one host).
func (a *Adapter) RemoteEnabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.remote != nil && len(a.remote.hosts) > 0
}

// RemoteHostNames returns the names of the configured remote hosts.
func (a *Adapter) RemoteHostNames() []string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.remote == nil {
		return nil
	}
	names := make([]string, len(a.remote.hosts))
	for i, h := range a.remote.hosts {
		names[i] = h.Name
	}
	return names
}

// RemoteComposeUp distributes a compose-file deployment to every
// configured remote host through the containers submodule's remote
// orchestration: it SCPs the compose file + the named build-context
// subdirectories to a per-host work directory, then runs
// `<compose> -f <remote-file> up -d --build` REMOTELY via the
// submodule's RemoteComposeOrchestrator (§11.4.76 — reuse, never
// reimplement).
//
// Parameters:
//   - composeFile: absolute path to the compose file on the local host.
//   - projectRoot: absolute path the compose file's relative build
//     contexts resolve against (compose `context:` is relative to the
//     compose file's directory).
//   - services: optional service filter (nil = all default-profile
//     services).
//   - buildContextDirs: project-root-relative directories that contain
//     build contexts referenced by the compose file (e.g.
//     "tests/e2e/mocks"). These are SCP'd so remote `--build` can build
//     them. The whole-repo context "." is deliberately NOT shipped (see
//     submodule CLAUDE.md gotcha #4) — services using it must be gated
//     behind a compose profile that is off by default.
//
// Returns the remote work directory used (where the compose file
// landed) so callers can probe / tear down, and any error.
func (a *Adapter) RemoteComposeUp(
	ctx context.Context,
	composeFile string,
	projectRoot string,
	services []string,
	buildContextDirs []string,
) (string, error) {
	a.mu.RLock()
	rs := a.remote
	a.mu.RUnlock()
	if rs == nil || len(rs.hosts) == 0 {
		return "", fmt.Errorf("remote distribution not enabled (call LoadRemoteConfig first)")
	}

	if err := a.sem.Acquire(ctx, 1); err != nil {
		return "", fmt.Errorf("remote compose up cancelled: %w", err)
	}
	defer a.sem.Release(1)

	composeBase := filepath.Base(composeFile)
	// Deterministic per-deployment remote work dir under the SSH user's
	// home. Keeping it stable lets re-runs reuse the SCP'd contexts and
	// lets callers/operators find it for `podman ps` / teardown.
	remoteWorkDir := "helixcode-fulltest"
	remoteComposePath := remoteWorkDir + "/" + composeBase

	var firstErr error
	for _, host := range rs.hosts {
		if err := a.distributeToHost(
			ctx, rs, host, composeFile, remoteComposePath,
			remoteWorkDir, projectRoot, services, buildContextDirs,
		); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("host %s: %w", host.Name, err)
			}
			rs.logger.Error("remote compose up failed on %s: %v", host.Name, err)
			continue
		}
		rs.logger.Info("remote compose up succeeded on %s (workdir %s)", host.Name, remoteWorkDir)
	}
	return remoteWorkDir, firstErr
}

// distributeToHost performs the SCP-then-remote-compose sequence for one
// host.
func (a *Adapter) distributeToHost(
	ctx context.Context,
	rs *remoteState,
	host remote.RemoteHost,
	localComposeFile string,
	remoteComposePath string,
	remoteWorkDir string,
	projectRoot string,
	services []string,
	buildContextDirs []string,
) error {
	// 1. Ensure the remote work + build-context parent dirs exist.
	mkdirCmd := "mkdir -p " + shellQuote(remoteWorkDir)
	for _, d := range buildContextDirs {
		mkdirCmd += " " + shellQuote(remoteWorkDir+"/"+filepath.Dir(d))
	}
	if res, err := rs.executor.Execute(ctx, host, mkdirCmd); err != nil {
		return fmt.Errorf("mkdir remote workdir: %w", err)
	} else if res.ExitCode != 0 {
		return fmt.Errorf("mkdir remote workdir: exit %d: %s", res.ExitCode, res.Stderr)
	}

	// 2. SCP the compose file.
	if err := rs.executor.CopyFile(ctx, host, localComposeFile, remoteComposePath); err != nil {
		return fmt.Errorf("scp compose file: %w", err)
	}

	// 3. SCP each declared build-context directory so remote `--build`
	//    can build the build-context services. Skip "." — shipping the
	//    whole repo root is forbidden (submodule CLAUDE.md gotcha #4);
	//    services that depend on it must be profile-gated off by default.
	for _, d := range buildContextDirs {
		if d == "." || d == "" {
			continue
		}
		localDir := filepath.Join(projectRoot, d)
		if _, statErr := os.Stat(localDir); statErr != nil {
			return fmt.Errorf("build-context dir %s: %w", localDir, statErr)
		}
		remoteDir := remoteWorkDir + "/" + d
		if err := rs.executor.CopyDir(ctx, host, localDir, remoteDir); err != nil {
			return fmt.Errorf("scp build context %s: %w", d, err)
		}
	}

	// 4. Run compose REMOTELY via the submodule's orchestrator. The
	//    orchestrator auto-detects podman-compose / docker compose on the
	//    host and runs `<cmd> -f <remote compose> --project-name ... up -d
	//    --build`. The compose file's relative build contexts resolve
	//    against the remote work dir, which is why we SCP'd them there.
	orch := remote.NewRemoteComposeOrchestrator(host, rs.executor, rs.logger)
	project := compose.ComposeProject{
		Name:     "helixcode-fulltest",
		File:     remoteComposePath,
		Services: services,
	}
	if err := orch.Up(ctx, project); err != nil {
		return fmt.Errorf("remote compose up: %w", err)
	}
	return nil
}

// RemoteComposeStatus returns the service status reported by the remote
// host's compose runtime, via the submodule's RemoteComposeOrchestrator.
func (a *Adapter) RemoteComposeStatus(
	ctx context.Context, hostName, remoteComposePath string,
) ([]compose.ServiceStatus, error) {
	a.mu.RLock()
	rs := a.remote
	a.mu.RUnlock()
	if rs == nil {
		return nil, fmt.Errorf("remote distribution not enabled")
	}
	for _, host := range rs.hosts {
		if host.Name != hostName {
			continue
		}
		orch := remote.NewRemoteComposeOrchestrator(host, rs.executor, rs.logger)
		return orch.Status(ctx, compose.ComposeProject{
			Name: "helixcode-fulltest",
			File: remoteComposePath,
		})
	}
	return nil, fmt.Errorf("host %s not configured", hostName)
}

// expandHome expands a leading ~ (or ~/) in a path to the current user's
// home directory. ssh/scp are exec'd without a shell so the tilde would
// otherwise be passed literally and the key lookup would fail.
func expandHome(p string) string {
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// shellQuote single-quotes a path for safe use inside a remote `sh -c`
// command line.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
