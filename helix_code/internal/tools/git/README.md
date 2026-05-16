# Git Package

The `git` package provides Git integration tools for HelixCode, enabling AI-powered commit message generation, automatic attribution tracking, and streamlined git workflow automation.

## Overview

This package enables:
- AI-powered commit message generation based on staged changes
- Attribution tracking for AI-assisted code contributions
- Auto-commit functionality with configurable triggers
- Git repository state analysis and change detection
- Integration with HelixCode's workflow system

## Key Types

### MessageGenerator

Generates intelligent commit messages from code changes.

```go
type MessageGenerator struct {
    llmProvider   llm.Provider
    config        *GeneratorConfig
    templateCache map[string]*template.Template
    mu            sync.RWMutex
}

type GeneratorConfig struct {
    Model            string        // LLM model to use
    MaxDiffSize      int           // Max diff size to analyze
    IncludeFileList  bool          // Include changed files in prompt
    ConventionalCommits bool       // Use conventional commit format
    MaxLength        int           // Max commit message length
    Language         string        // Natural language for messages
    CustomPrompt     string        // Custom prompt template
    Timeout          time.Duration // Generation timeout
}
```

### AttributionTracker

Tracks and manages AI contribution attribution.

```go
type AttributionTracker struct {
    config      *AttributionConfig
    sessionID   string
    entries     []AttributionEntry
    mu          sync.RWMutex
}

type AttributionConfig struct {
    Enabled         bool
    Format          AttributionFormat // trailer, comment, none
    AgentName       string
    AgentEmail      string
    IncludeModel    bool
    IncludeSession  bool
    TrailerKey      string // e.g., "Co-Authored-By"
}

type AttributionEntry struct {
    CommitHash   string
    Timestamp    time.Time
    AgentName    string
    Model        string
    SessionID    string
    FilesChanged []string
    LinesAdded   int
    LinesRemoved int
}
```

### AutoCommitter

Manages automatic commit functionality.

```go
type AutoCommitter struct {
    generator     *MessageGenerator
    tracker       *AttributionTracker
    config        *AutoCommitConfig
    repo          *Repository
    mu            sync.RWMutex
}

type AutoCommitConfig struct {
    Enabled          bool
    TriggerOnSave    bool           // Commit after file saves
    TriggerOnInterval time.Duration // Periodic commits
    MinChanges       int            // Min changes before commit
    MaxChanges       int            // Max changes per commit
    ExcludePatterns  []string       // Files to exclude
    IncludePatterns  []string       // Files to include
    SquashInterval   time.Duration  // Squash commits within interval
    DryRun           bool           // Preview without committing
}
```

### Repository

Wrapper for Git repository operations.

```go
type Repository struct {
    path        string
    gitDir      string
    worktree    *git.Worktree
    repo        *git.Repository
    mu          sync.RWMutex
}

type RepositoryStatus struct {
    Branch        string
    Clean         bool
    Staged        []FileChange
    Unstaged      []FileChange
    Untracked     []string
    AheadBy       int
    BehindBy      int
    RemoteURL     string
    LastCommit    *CommitInfo
}

type FileChange struct {
    Path      string
    Status    ChangeStatus // Added, Modified, Deleted, Renamed
    OldPath   string       // For renames
    Staged    bool
}
```

## Usage Examples

### Generating Commit Messages

```go
package main

import (
    "context"
    "fmt"

    "dev.helix.code/internal/tools/git"
    "dev.helix.code/internal/llm"
)

func main() {
    // Create LLM provider
    provider, err := llm.NewProvider(&llm.Config{
        Provider: "openai",
        Model:    "gpt-4",
        APIKey:   "your-api-key",
    })
    if err != nil {
        panic(err)
    }

    // Create message generator
    generator := git.NewMessageGenerator(provider, &git.GeneratorConfig{
        ConventionalCommits: true,
        MaxLength:           72,
        IncludeFileList:     true,
    })

    ctx := context.Background()

    // Generate message from staged changes
    diff := `diff --git a/src/auth.go b/src/auth.go
index 1234567..abcdefg 100644
--- a/src/auth.go
+++ b/src/auth.go
@@ -10,6 +10,15 @@ func Authenticate(username, password string) error {
+    // Add rate limiting
+    if !rateLimiter.Allow(username) {
+        return ErrTooManyAttempts
+    }
+
     user, err := db.GetUser(username)
     if err != nil {
         return err
     }`

    message, err := generator.Generate(ctx, &git.GenerateRequest{
        Diff:         diff,
        FilesChanged: []string{"src/auth.go"},
        Context:      "Adding security improvements to authentication",
    })

    if err != nil {
        panic(err)
    }

    fmt.Println(message)
    // Output: feat(auth): add rate limiting to prevent brute force attacks
}
```

### Attribution Tracking

```go
// Create attribution tracker
tracker := git.NewAttributionTracker(&git.AttributionConfig{
    Enabled:      true,
    Format:       git.AttributionTrailer,
    AgentName:    "HelixCode AI",
    AgentEmail:   "ai@helix.dev",
    IncludeModel: true,
    TrailerKey:   "Co-Authored-By",
})

// Track a contribution
tracker.Track(&git.Contribution{
    Files:        []string{"src/auth.go", "src/auth_test.go"},
    LinesAdded:   45,
    LinesRemoved: 12,
    Model:        "gpt-4",
    SessionID:    "session-123",
})

// Get attribution line for commit message
attribution := tracker.GetAttributionLine()
// Output: Co-Authored-By: HelixCode AI <ai@helix.dev>

// Get full attribution with model info
fullAttribution := tracker.GetFullAttribution()
// Output:
// Co-Authored-By: HelixCode AI <ai@helix.dev>
// AI-Model: gpt-4
// AI-Session: session-123

// Generate report
report := tracker.GenerateReport()
fmt.Printf("Total contributions: %d\n", report.TotalContributions)
fmt.Printf("Lines added: %d\n", report.TotalLinesAdded)
fmt.Printf("Files modified: %d\n", report.TotalFilesModified)
```

### Auto-Commit

```go
// Create auto-committer
autoCommitter := git.NewAutoCommitter(&git.AutoCommitConfig{
    Enabled:           true,
    TriggerOnInterval: 30 * time.Minute,
    MinChanges:        5,
    ExcludePatterns:   []string{"*.log", "*.tmp", "node_modules/**"},
    IncludePatterns:   []string{"src/**", "tests/**"},
})

// Start auto-commit monitoring
ctx := context.Background()
err := autoCommitter.Start(ctx)

// Manual trigger with preview
preview, err := autoCommitter.Preview(ctx)
fmt.Printf("Would commit %d files:\n", len(preview.Files))
for _, f := range preview.Files {
    fmt.Printf("  %s: %s\n", f.Status, f.Path)
}
fmt.Printf("Suggested message: %s\n", preview.Message)

// Commit now
result, err := autoCommitter.CommitNow(ctx)
fmt.Printf("Created commit: %s\n", result.Hash)

// Stop auto-commit
autoCommitter.Stop()
```

### Repository Operations

```go
// Open repository
repo, err := git.OpenRepository("/path/to/project")

// Get repository status
status, err := repo.GetStatus(ctx)
fmt.Printf("Branch: %s\n", status.Branch)
fmt.Printf("Clean: %v\n", status.Clean)
fmt.Printf("Staged: %d files\n", len(status.Staged))
fmt.Printf("Unstaged: %d files\n", len(status.Unstaged))

// Stage files
err = repo.Stage(ctx, []string{"src/main.go", "src/utils.go"})

// Stage all changes
err = repo.StageAll(ctx)

// Unstage files
err = repo.Unstage(ctx, []string{"src/main.go"})

// Create commit
hash, err := repo.Commit(ctx, &git.CommitOptions{
    Message: "feat: add new feature",
    Author: &git.Author{
        Name:  "Developer",
        Email: "dev@example.com",
    },
    Trailers: map[string]string{
        "Co-Authored-By": "HelixCode AI <ai@helix.dev>",
    },
})

// Get diff
diff, err := repo.GetDiff(ctx, &git.DiffOptions{
    Staged: true,
    Context: 3,
})

// Get commit history
commits, err := repo.Log(ctx, &git.LogOptions{
    MaxCount: 10,
    Since:    time.Now().Add(-7 * 24 * time.Hour),
})

for _, c := range commits {
    fmt.Printf("%s: %s (%s)\n", c.Hash[:7], c.Message, c.Author.Name)
}
```

### Conventional Commits

```go
// Configure for conventional commits
generator := git.NewMessageGenerator(provider, &git.GeneratorConfig{
    ConventionalCommits: true,
    CustomPrompt: `Generate a conventional commit message for these changes.
Use these types: feat, fix, docs, style, refactor, test, chore
Include scope if identifiable from the file paths.`,
})

// Parse conventional commit
parsed, err := git.ParseConventionalCommit("feat(auth): add OAuth2 support")
fmt.Printf("Type: %s\n", parsed.Type)        // feat
fmt.Printf("Scope: %s\n", parsed.Scope)      // auth
fmt.Printf("Subject: %s\n", parsed.Subject)  // add OAuth2 support
fmt.Printf("Breaking: %v\n", parsed.Breaking) // false

// Validate conventional commit
valid := git.ValidateConventionalCommit(message)
if !valid {
    fmt.Println("Message does not follow conventional commit format")
}
```

### Change Detection

```go
// Detect file changes
changes, err := repo.DetectChanges(ctx, &git.DetectOptions{
    IncludeUntracked: true,
    IgnorePatterns:   []string{"*.log", ".git/**"},
})

for _, change := range changes {
    fmt.Printf("[%s] %s\n", change.Status, change.Path)
}

// Get file diff
fileDiff, err := repo.GetFileDiff(ctx, "src/main.go")
fmt.Printf("Lines added: %d\n", fileDiff.Additions)
fmt.Printf("Lines removed: %d\n", fileDiff.Deletions)

// Check if file has changed
changed, err := repo.IsFileChanged(ctx, "src/main.go")

// Get changed files since commit
files, err := repo.ChangedSince(ctx, "abc123")
```

## Configuration Options

### GeneratorConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Model` | string | gpt-4 | LLM model for generation |
| `MaxDiffSize` | int | 10000 | Max diff characters to analyze |
| `IncludeFileList` | bool | true | Include file names in prompt |
| `ConventionalCommits` | bool | true | Use conventional format |
| `MaxLength` | int | 72 | Max message line length |
| `Language` | string | en | Message language |
| `CustomPrompt` | string | "" | Custom generation prompt |
| `Timeout` | time.Duration | 30s | Generation timeout |

### AttributionConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Enabled` | bool | true | Enable attribution tracking |
| `Format` | AttributionFormat | Trailer | Attribution format |
| `AgentName` | string | HelixCode AI | Agent display name |
| `AgentEmail` | string | ai@helix.dev | Agent email |
| `IncludeModel` | bool | false | Include model name |
| `IncludeSession` | bool | false | Include session ID |
| `TrailerKey` | string | Co-Authored-By | Git trailer key |

### AutoCommitConfig

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `Enabled` | bool | false | Enable auto-commit |
| `TriggerOnSave` | bool | false | Commit on file save |
| `TriggerOnInterval` | time.Duration | 0 | Commit interval |
| `MinChanges` | int | 1 | Min changes to trigger |
| `MaxChanges` | int | 0 | Max changes per commit (0=unlimited) |
| `ExcludePatterns` | []string | [] | Excluded file patterns |
| `IncludePatterns` | []string | [] | Included file patterns |
| `SquashInterval` | time.Duration | 0 | Squash commits interval |
| `DryRun` | bool | false | Preview mode |

## Security Considerations

1. **API Key Protection**: LLM API keys should be stored securely and never committed to repositories.

2. **Sensitive Data in Diffs**: The message generator may see sensitive data in diffs. Configure `MaxDiffSize` appropriately and exclude sensitive files.

3. **Attribution Privacy**: Consider privacy implications of including session IDs or model information in commits.

4. **Auto-Commit Risks**: Auto-commit can accidentally commit sensitive files. Always configure `ExcludePatterns` carefully.

5. **Force Push Protection**: The package does not perform force pushes. Any push operations follow standard git safety practices.

## Error Types

```go
var (
    ErrNotARepository    = errors.New("not a git repository")
    ErrNothingToCommit   = errors.New("nothing to commit")
    ErrMergeConflict     = errors.New("merge conflict detected")
    ErrDetachedHead      = errors.New("HEAD is detached")
    ErrDiffTooLarge      = errors.New("diff exceeds maximum size")
    ErrGenerationFailed  = errors.New("message generation failed")
    ErrInvalidCommitMsg  = errors.New("invalid commit message format")
)
```

## Best Practices

1. **Review generated messages** before committing, especially for important changes.

2. **Configure conventional commits** for consistent commit history.

3. **Set appropriate diff size limits** to prevent excessive LLM token usage.

4. **Use attribution** to maintain transparency about AI-assisted contributions.

5. **Test auto-commit patterns** in a safe environment before enabling in production.
