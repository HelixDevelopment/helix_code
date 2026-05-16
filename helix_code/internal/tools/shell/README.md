# Shell Package

The `shell` package provides secure, controlled, and efficient shell command execution for HelixCode. It implements comprehensive security measures including command allowlists/blocklists, sandboxing, resource limits, and real-time output streaming.

## Overview

This package enables:
- Secure shell command execution with configurable restrictions
- Command allowlist and blocklist enforcement
- Sandboxed execution with resource limits
- Real-time output streaming for long-running commands
- Signal handling and graceful termination
- Working directory and environment variable control
- Execution auditing and logging

## Key Types

### ShellExecutor

The main interface for shell command execution.

```go
type ShellExecutor interface {
    // Execute runs a command and returns the result
    Execute(ctx context.Context, cmd string, opts *ExecuteOptions) (*ExecuteResult, error)

    // ExecuteStream runs a command with real-time output streaming
    ExecuteStream(ctx context.Context, cmd string, opts *ExecuteOptions, output chan<- OutputChunk) error

    // IsAllowed checks if a command is allowed to execute
    IsAllowed(cmd string) bool

    // Kill terminates a running command
    Kill(cmdID string) error
}
```

### DefaultExecutor

The primary implementation of ShellExecutor.

```go
type DefaultExecutor struct {
    config          *ExecutorConfig
    securityManager *SecurityManager
    runningCmds     map[string]*runningCommand
    mu              sync.RWMutex
}

type ExecutorConfig struct {
    Shell           string        // Shell to use (bash, sh, zsh)
    Timeout         time.Duration // Default command timeout
    WorkingDir      string        // Default working directory
    Environment     []string      // Environment variables
    AllowedCommands []string      // Command allowlist
    BlockedCommands []string      // Command blocklist
    BlockedPatterns []string      // Regex patterns to block
    MaxOutputSize   int64         // Max output buffer size
    SandboxEnabled  bool          // Enable sandboxing
    SandboxConfig   *SandboxConfig
}
```

### ExecuteOptions

Per-command execution options.

```go
type ExecuteOptions struct {
    WorkingDir    string            // Override working directory
    Environment   map[string]string // Additional env vars
    Timeout       time.Duration     // Command timeout
    Stdin         io.Reader         // Standard input
    CombineOutput bool              // Combine stdout/stderr
    Background    bool              // Run in background
    User          string            // Run as specific user
    Group         string            // Run as specific group
}
```

### ExecuteResult

The result of a command execution.

```go
type ExecuteResult struct {
    CommandID   string
    ExitCode    int
    Stdout      string
    Stderr      string
    Duration    time.Duration
    StartTime   time.Time
    EndTime     time.Time
    Killed      bool
    TimedOut    bool
    Error       error
}
```

### OutputChunk

Real-time output data for streaming execution.

```go
type OutputChunk struct {
    Stream    string    // "stdout" or "stderr"
    Data      []byte    // Output data
    Timestamp time.Time // Chunk timestamp
    EOF       bool      // End of stream
}
```

### SecurityManager

Handles command security validation.

```go
type SecurityManager struct {
    config         *SecurityConfig
    allowedPattern []*regexp.Regexp
    blockedPattern []*regexp.Regexp
    mu             sync.RWMutex
}

type SecurityConfig struct {
    AllowedCommands   []string       // Exact command allowlist
    BlockedCommands   []string       // Exact command blocklist
    AllowedPatterns   []string       // Regex patterns allowed
    BlockedPatterns   []string       // Regex patterns blocked
    BlockRootUser     bool           // Block running as root
    BlockSudo         bool           // Block sudo commands
    MaxCmdLength      int            // Max command length
    AuditEnabled      bool           // Enable audit logging
    AuditLogPath      string         // Audit log file path
}
```

### SandboxConfig

Configuration for sandboxed execution.

```go
type SandboxConfig struct {
    Enabled         bool
    NetworkAccess   bool           // Allow network access
    FileSystemRoot  string         // Chroot directory
    ReadOnlyPaths   []string       // Read-only mount paths
    ReadWritePaths  []string       // Read-write mount paths
    HiddenPaths     []string       // Hidden/inaccessible paths
    MaxMemory       int64          // Memory limit in bytes
    MaxCPU          float64        // CPU limit (1.0 = 100%)
    MaxProcesses    int            // Max process count
    MaxFileSize     int64          // Max file size
    MaxOpenFiles    int            // Max open file descriptors
    Timeout         time.Duration  // Sandbox timeout
}
```

## Usage Examples

### Basic Command Execution

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/shell"
)

func main() {
    // Create executor with security config
    executor := shell.NewDefaultExecutor(&shell.ExecutorConfig{
        Shell:      "/bin/bash",
        Timeout:    30 * time.Second,
        WorkingDir: "/home/user/project",
        BlockedCommands: []string{
            "rm -rf /",
            "dd if=/dev/zero",
            ":(){:|:&};:",
        },
    })

    ctx := context.Background()

    // Execute a simple command
    result, err := executor.Execute(ctx, "ls -la", nil)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Exit code: %d\n", result.ExitCode)
    fmt.Printf("Output:\n%s\n", result.Stdout)
    fmt.Printf("Duration: %v\n", result.Duration)
}
```

### Command with Options

```go
// Execute with custom options
result, err := executor.Execute(ctx, "npm install", &shell.ExecuteOptions{
    WorkingDir: "/home/user/project/frontend",
    Environment: map[string]string{
        "NODE_ENV": "development",
        "CI":       "true",
    },
    Timeout:       5 * time.Minute,
    CombineOutput: true,
})

if result.ExitCode != 0 {
    fmt.Printf("Command failed:\n%s\n", result.Stderr)
}
```

### Streaming Output

```go
// Create output channel
output := make(chan shell.OutputChunk, 100)

// Execute with streaming
go func() {
    err := executor.ExecuteStream(ctx, "make build", nil, output)
    if err != nil {
        fmt.Printf("Stream error: %v\n", err)
    }
}()

// Process output in real-time
for chunk := range output {
    if chunk.EOF {
        break
    }
    fmt.Printf("[%s] %s", chunk.Stream, string(chunk.Data))
}
```

### Security Validation

```go
// Create security-focused executor
executor := shell.NewDefaultExecutor(&shell.ExecutorConfig{
    AllowedCommands: []string{
        "git", "npm", "go", "make", "ls", "cat", "grep",
    },
    BlockedCommands: []string{
        "rm", "sudo", "chmod", "chown", "curl", "wget",
    },
    BlockedPatterns: []string{
        `rm\s+-rf`,
        `>\s*/dev/`,
        `\|\s*sh`,
        `eval\s+`,
    },
})

// Check if command is allowed
if !executor.IsAllowed("rm -rf /tmp/*") {
    fmt.Println("Command blocked by security policy")
}

// Execute allowed command
result, err := executor.Execute(ctx, "git status", nil)
```

### Sandboxed Execution

```go
// Create sandboxed executor
executor := shell.NewDefaultExecutor(&shell.ExecutorConfig{
    SandboxEnabled: true,
    SandboxConfig: &shell.SandboxConfig{
        Enabled:        true,
        NetworkAccess:  false,
        FileSystemRoot: "/home/user/sandbox",
        ReadOnlyPaths:  []string{"/usr", "/lib"},
        ReadWritePaths: []string{"/home/user/sandbox/workspace"},
        HiddenPaths:    []string{"/etc/passwd", "/etc/shadow"},
        MaxMemory:      512 * 1024 * 1024, // 512MB
        MaxCPU:         0.5,               // 50% CPU
        MaxProcesses:   10,
        MaxFileSize:    100 * 1024 * 1024, // 100MB
        Timeout:        1 * time.Minute,
    },
})

// Execute in sandbox
result, err := executor.Execute(ctx, "python script.py", nil)
```

### Background Execution

```go
// Start command in background
result, err := executor.Execute(ctx, "npm run dev", &shell.ExecuteOptions{
    Background: true,
})

cmdID := result.CommandID
fmt.Printf("Started background command: %s\n", cmdID)

// Later, check status
status, err := executor.GetStatus(cmdID)
fmt.Printf("Running: %v\n", status.Running)

// Kill the background process
err = executor.Kill(cmdID)
```

### Input Handling

```go
// Execute with stdin input
input := strings.NewReader("yes\nno\nmaybe\n")

result, err := executor.Execute(ctx, "interactive-script.sh", &shell.ExecuteOptions{
    Stdin: input,
})

// Pipe content to command
fileContent := strings.NewReader("Hello, World!")
result, err = executor.Execute(ctx, "cat > output.txt", &shell.ExecuteOptions{
    Stdin:      fileContent,
    WorkingDir: "/tmp",
})
```

### Audit Logging

```go
// Create executor with audit logging
executor := shell.NewDefaultExecutor(&shell.ExecutorConfig{
    SecurityConfig: &shell.SecurityConfig{
        AuditEnabled: true,
        AuditLogPath: "/var/log/helix/shell-audit.log",
    },
})

// All commands are now logged
executor.Execute(ctx, "ls -la", nil)
executor.Execute(ctx, "git status", nil)

// Query audit log
entries, err := executor.GetAuditLog(&shell.AuditQuery{
    StartTime: time.Now().Add(-1 * time.Hour),
    EndTime:   time.Now(),
    User:      "developer",
})

for _, entry := range entries {
    fmt.Printf("[%s] %s: %s (exit: %d)\n",
        entry.Timestamp, entry.User, entry.Command, entry.ExitCode)
}
```

### Signal Handling

```go
// Execute with custom signal handling
result, err := executor.Execute(ctx, "long-running-task", &shell.ExecuteOptions{
    Timeout: 10 * time.Minute,
})

// If context is cancelled, command receives SIGTERM
ctx, cancel := context.WithCancel(context.Background())

go func() {
    time.Sleep(5 * time.Second)
    cancel() // This will send SIGTERM to the process
}()

result, err := executor.Execute(ctx, "sleep 60", nil)
if result.Killed {
    fmt.Println("Command was terminated")
}
```

### Environment Management

```go
// Set default environment
executor := shell.NewDefaultExecutor(&shell.ExecutorConfig{
    Environment: []string{
        "PATH=/usr/local/bin:/usr/bin:/bin",
        "HOME=/home/user",
        "LANG=en_US.UTF-8",
    },
})

// Override for specific command
result, err := executor.Execute(ctx, "printenv", &shell.ExecuteOptions{
    Environment: map[string]string{
        "CUSTOM_VAR": "value",
        "DEBUG":      "true",
    },
})
```

## Configuration Options

### ExecutorConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Shell` | string | /bin/sh | Shell to use for execution |
| `Timeout` | time.Duration | 30s | Default command timeout |
| `WorkingDir` | string | cwd | Default working directory |
| `Environment` | []string | [] | Default environment variables |
| `AllowedCommands` | []string | [] | Command allowlist |
| `BlockedCommands` | []string | [] | Command blocklist |
| `BlockedPatterns` | []string | [] | Regex patterns to block |
| `MaxOutputSize` | int64 | 10MB | Max output buffer size |
| `SandboxEnabled` | bool | false | Enable sandboxing |

### SandboxConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Enabled` | bool | false | Enable sandbox |
| `NetworkAccess` | bool | false | Allow network access |
| `FileSystemRoot` | string | / | Chroot directory |
| `ReadOnlyPaths` | []string | [] | Read-only mount paths |
| `ReadWritePaths` | []string | [] | Read-write mount paths |
| `HiddenPaths` | []string | [] | Hidden paths |
| `MaxMemory` | int64 | 512MB | Memory limit |
| `MaxCPU` | float64 | 1.0 | CPU limit |
| `MaxProcesses` | int | 100 | Max process count |
| `MaxFileSize` | int64 | 100MB | Max file size |
| `MaxOpenFiles` | int | 1024 | Max open files |

## Security Considerations

1. **Command Allowlisting**: Always prefer allowlisting over blocklisting for security-critical environments.

2. **Pattern Blocking**: Use regex patterns to block dangerous command patterns like `rm -rf`, shell injection, etc.

3. **Sandboxing**: Enable sandboxing for untrusted code execution with strict resource limits.

4. **Root Blocking**: Block execution as root user unless absolutely necessary.

5. **Sudo Prevention**: Block sudo commands to prevent privilege escalation.

6. **Output Limits**: Set `MaxOutputSize` to prevent memory exhaustion from verbose commands.

7. **Timeout Enforcement**: Always set appropriate timeouts to prevent runaway processes.

8. **Audit Logging**: Enable audit logging for compliance and security monitoring.

9. **Environment Isolation**: Control environment variables to prevent information leakage.

## Built-in Blocked Patterns

The package includes default blocked patterns for common dangerous commands:

```go
var DefaultBlockedPatterns = []string{
    `rm\s+-rf\s+/`,           // rm -rf /
    `dd\s+if=/dev/zero`,      // dd disk wipe
    `mkfs\.`,                 // filesystem format
    `>\s*/dev/sd`,            // write to block device
    `:(){:|:&};:`,            // fork bomb
    `chmod\s+777`,            // insecure permissions
    `curl.*\|\s*sh`,          // curl pipe to shell
    `wget.*\|\s*sh`,          // wget pipe to shell
    `eval\s+.*\$`,            // eval with variable
    `\$\(.*\)`,               // command substitution (configurable)
}
```

## Error Types

```go
var (
    ErrCommandBlocked      = errors.New("command blocked by security policy")
    ErrCommandNotAllowed   = errors.New("command not in allowlist")
    ErrTimeout             = errors.New("command execution timed out")
    ErrMaxOutputExceeded   = errors.New("output size limit exceeded")
    ErrCommandNotFound     = errors.New("command not found")
    ErrPermissionDenied    = errors.New("permission denied")
    ErrSandboxViolation    = errors.New("sandbox policy violation")
    ErrResourceLimitHit    = errors.New("resource limit exceeded")
)
```

## Best Practices

1. **Use allowlists** for known-safe commands rather than trying to block all dangerous ones.

2. **Set appropriate timeouts** based on expected command duration.

3. **Enable sandboxing** when executing user-provided or untrusted commands.

4. **Monitor resource usage** through sandbox limits to prevent DoS attacks.

5. **Log all executions** for audit trails and security monitoring.

6. **Validate working directories** to prevent directory traversal attacks.

7. **Sanitize environment variables** to prevent injection attacks.
