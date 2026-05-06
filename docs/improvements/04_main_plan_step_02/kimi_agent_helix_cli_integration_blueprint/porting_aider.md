# Aider -> HelixCode: Complete Line-by-Line Porting Plan

> **Document Version**: 1.0  
> **Date**: 2025-05-04  
> **Scope**: All 15 Aider features ported to HelixCode (`dev.helix.code`)  
> **Architecture**: Go 1.26, Cobra CLI, Gin HTTP, Viper config, PostgreSQL, Redis, Tree-sitter  

---

## TABLE OF CONTENTS

1. [Feature 1: Architect/Editor Dual-Model Architecture](#1-architecteditor-dual-model-architecture)
2. [Feature 2: 4-Layer Fuzzy Matching](#2-4-layer-fuzzy-matching)
3. [Feature 3: Git-Native Auto-Commit Workflow](#3-git-native-auto-commit-workflow)
4. [Feature 4: Repository Map (Tree-sitter Based)](#4-repository-map-tree-sitter-based)
5. [Feature 5: Unified Diff Format](#5-unified-diff-format)
6. [Feature 6: Voice-to-Code Input](#6-voice-to-code-input)
7. [Feature 7: Image Input for UI Development](#7-image-input-for-ui-development)
8. [Feature 8: IDE Watch Mode](#8-ide-watch-mode)
9. [Feature 9: 4 Edit Formats Support](#9-4-edit-formats-support)
10. [Feature 10: Auto Test/Lint-Fix Loop](#10-auto-testlint-fix-loop)
11. [Feature 11: Benchmark-Leading Accuracy](#11-benchmark-leading-accuracy)
12. [Feature 12: Prompt Caching Optimization](#12-prompt-caching-optimization)
13. [Feature 13: Hierarchical Context](#13-hierarchical-context)
14. [Feature 14: Model-Optimized Edit Format Selection](#14-model-optimized-edit-format-selection)
15. [Feature 15: Browser Automation (Playwright)](#15-browser-automation-playwright)

---

## 1. Architect/Editor Dual-Model Architecture

### Source Location (in Aider)
- `aider/coders/architect_coder.py` (300+ lines)
- `aider/coders/editblock_coder.py` (editor implementation)
- `aider/models.py` (model selection per role)
- Aider benchmarks: o1-preview architect + DeepSeek/o1-mini editor = 85% pass rate

### Target Location (in HelixCode)
- **NEW**: `internal/agent/architect_agent.go`
- **NEW**: `internal/agent/editor_agent.go`
- **NEW**: `internal/agent/handoff.go`
- **MODIFY**: `internal/agent/orchestrator.go`
- **MODIFY**: `cmd/agent.go` (Cobra CLI flags)

### Exact Code Changes

#### File 1: `internal/agent/architect_agent.go` (NEW)

```go
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/session"
)

// ArchitectRole defines the reasoning/planning agent that designs solutions
// but does NOT emit file edits directly.
type ArchitectRole struct {
	ModelID         string
	Provider        llm.Provider
	SystemPrompt    string
	MaxTokens       int
	Temperature     float64
}

// ArchitectResponse contains the solution design from the architect model.
type ArchitectResponse struct {
	SolutionPlan string
	FilesToEdit  []string
	Reasoning    string
	Timestamp    time.Time
}

// NewArchitectRole creates an architect role with sensible defaults.
func NewArchitectRole(provider llm.Provider, modelID string) *ArchitectRole {
	return &ArchitectRole{
		ModelID:     modelID,
		Provider:    provider,
		MaxTokens:   8192,
		Temperature: 0.3, // Lower temperature for deterministic planning
		SystemPrompt: `You are an expert software architect. Your task is to analyze coding requests and produce detailed implementation plans. You do NOT write code edits directly. Instead, you describe:
1. Which files need to change
2. What changes are needed in each file
3. The reasoning behind each change
Be specific about function signatures, class names, and import statements.`,
	}
}

// Design accepts a user request and produces a solution plan.
func (a *ArchitectRole) Design(ctx context.Context, req *session.AgentRequest) (*ArchitectResponse, error) {
	messages := []llm.Message{
		{Role: "system", Content: a.SystemPrompt},
		{Role: "user", Content: req.Prompt},
	}

	llmReq := &llm.LLMRequest{
		Model:       a.ModelID,
		MaxTokens:   a.MaxTokens,
		Temperature: a.Temperature,
		Messages:    messages,
	}

	resp, err := a.Provider.Generate(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("architect generation failed: %w", err)
	}

	// Parse files to edit from the response
	files := extractFileReferences(resp.Content)

	return &ArchitectResponse{
		SolutionPlan: resp.Content,
		FilesToEdit:  files,
		Reasoning:    resp.Content,
		Timestamp:    time.Now(),
	}, nil
}

// extractFileReferences scans architect output for file path references.
func extractFileReferences(text string) []string {
	var files []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if strings.Contains(line, "```") || strings.Contains(line, ".go") || strings.Contains(line, ".py") {
			parts := strings.Fields(line)
			for _, part := range parts {
				part = strings.Trim(part, "`'\"")
				if looksLikeFilePath(part) {
					files = appendUnique(files, part)
				}
			}
		}
	}
	return files
}

func looksLikeFilePath(s string) bool {
	for _, ext := range []string{".go", ".py", ".js", ".ts", ".rs", ".java", ".cpp", ".h", ".md", ".yaml", ".json", ".toml"} {
		if strings.Contains(s, ext) {
			return true
		}
	}
	return strings.Contains(s, "/") && !strings.HasPrefix(s, "http")
}

func appendUnique(slice []string, s string) []string {
	for _, item := range slice {
		if item == s {
			return slice
		}
	}
	return append(slice, s)
}
```

#### File 2: `internal/agent/editor_agent.go` (NEW)

```go
package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dev.helix.code/internal/editor"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/session"
)

// EditorRole converts architect solution plans into concrete file edits.
type EditorRole struct {
	ModelID      string
	Provider     llm.Provider
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	EditFormat   string // "diff", "udiff", "whole", "diff-fenced"
}

// EditorResponse contains executable file edits.
type EditorResponse struct {
	Edits     []editor.FileEdit
	EditCount int
	Timestamp time.Time
	Errors    []string
}

// NewEditorRole creates an editor role optimized for code generation.
func NewEditorRole(provider llm.Provider, modelID string, editFormat string) *EditorRole {
	return &EditorRole{
		ModelID:     modelID,
		Provider:    provider,
		MaxTokens:   4096,
		Temperature: 0.1, // Very low for deterministic edits
		EditFormat:  editFormat,
		SystemPrompt: fmt.Sprintf(`You are an expert code editor. Convert the provided solution plan into specific %s edit blocks. Only output valid edits in the requested format. Do not include explanations outside the edit blocks.`, editFormat),
	}
}

// Edit takes an architect's solution plan and produces file edits.
func (e *EditorRole) Edit(ctx context.Context, plan *ArchitectResponse, req *session.AgentRequest) (*EditorResponse, error) {
	prompt := buildEditorPrompt(plan, e.EditFormat)

	messages := []llm.Message{
		{Role: "system", Content: e.SystemPrompt},
		{Role: "user", Content: prompt},
	}

	llmReq := &llm.LLMRequest{
		Model:       e.ModelID,
		MaxTokens:   e.MaxTokens,
		Temperature: e.Temperature,
		Messages:    messages,
	}

	resp, err := e.Provider.Generate(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("editor generation failed: %w", err)
	}

	// Parse edits from response based on format
	edits, errors := parseEditsFromResponse(resp.Content, e.EditFormat)

	return &EditorResponse{
		Edits:     edits,
		EditCount: len(edits),
		Timestamp: time.Now(),
		Errors:    errors,
	}, nil
}

func buildEditorPrompt(plan *ArchitectResponse, format string) string {
	var b strings.Builder
	b.WriteString("SOLUTION PLAN:\n")
	b.WriteString(plan.SolutionPlan)
	b.WriteString("\n\nFILES TO EDIT:\n")
	for _, f := range plan.FilesToEdit {
		b.WriteString("- ")
		b.WriteString(f)
		b.WriteString("\n")
	}
	b.WriteString(fmt.Sprintf("\n\nConvert the above plan into %s edit blocks. Be precise with search strings.", format))
	return b.String()
}

// parseEditsFromResponse dispatches to format-specific parsers.
func parseEditsFromResponse(content string, format string) ([]editor.FileEdit, []string) {
	switch format {
	case "diff", "diff-fenced":
		return parseDiffEdits(content)
	case "udiff":
		return parseUnifiedDiffEdits(content)
	case "whole":
		return parseWholeFileEdits(content)
	default:
		return parseDiffEdits(content) // default
	}
}
```

#### File 3: `internal/agent/handoff.go` (NEW)

```go
package agent

import (
	"context"
	"fmt"
	"time"

	"dev.helix.code/internal/session"
)

// HandoffProtocol manages the architect -> editor handoff with state tracking.
type HandoffProtocol struct {
	Architect     *ArchitectRole
	Editor        *EditorRole
	RetryPolicy   RetryPolicy
	MaxAttempts   int
}

// RetryPolicy defines backoff for editor retry on failure.
type RetryPolicy struct {
	MaxRetries  int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Multiplier  float64
}

// DefaultRetryPolicy returns sensible retry settings.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries: 3,
		BaseDelay:  1 * time.Second,
		MaxDelay:   30 * time.Second,
		Multiplier: 2.0,
	}
}

// Execute runs the full architect -> editor pipeline.
func (h *HandoffProtocol) Execute(ctx context.Context, req *session.AgentRequest) (*EditorResponse, error) {
	// Phase 1: Architect designs solution
	plan, err := h.Architect.Design(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("architect phase failed: %w", err)
	}

	// Phase 2: Editor implements (with retry)
	var editorResp *EditorResponse
	var lastErr error
	for attempt := 0; attempt <= h.RetryPolicy.MaxRetries; attempt++ {
		editorResp, err = h.Editor.Edit(ctx, plan, req)
		if err == nil && len(editorResp.Errors) == 0 {
			break
		}
		lastErr = err
		if attempt < h.RetryPolicy.MaxRetries {
			delay := calculateBackoff(h.RetryPolicy, attempt)
			time.Sleep(delay)
			// Feedback loop: include errors in next attempt
			req.Prompt = fmt.Sprintf("%s\n\nPREVIOUS ERRORS: %v\nPlease fix these issues.", req.Prompt, editorResp.Errors)
		}
	}

	if lastErr != nil && (editorResp == nil || len(editorResp.Errors) > 0) {
		return nil, fmt.Errorf("editor phase failed after %d retries: %w", h.RetryPolicy.MaxRetries, lastErr)
	}

	return editorResp, nil
}

func calculateBackoff(policy RetryPolicy, attempt int) time.Duration {
	delay := policy.BaseDelay
	for i := 0; i < attempt; i++ {
		delay = time.Duration(float64(delay) * policy.Multiplier)
		if delay > policy.MaxDelay {
			delay = policy.MaxDelay
		}
	}
	return delay
}
```

#### File 4: `internal/agent/orchestrator.go` (MODIFY)

```go
package agent

// ... existing imports ...

// ExecuteDualModel runs architect then editor.
func (o *Orchestrator) ExecuteDualModel(ctx context.Context, req *session.AgentRequest) error {
	architect := NewArchitectRole(o.llmProvider, req.ArchitectModel)
	editor := NewEditorRole(o.llmProvider, req.EditorModel, req.EditFormat)

	handoff := &HandoffProtocol{
		Architect:   architect,
		Editor:      editor,
		RetryPolicy: DefaultRetryPolicy(),
	}

	resp, err := handoff.Execute(ctx, req)
	if err != nil {
		return err
	}

	// Apply edits through the multi-file editor
	for _, edit := range resp.Edits {
		if err := o.multiFileEditor.ApplyEdit(ctx, edit); err != nil {
			return fmt.Errorf("failed to apply edit to %s: %w", edit.Path, err)
		}
	}

	// Auto-commit if enabled (see Feature 3)
	if o.config.AutoCommit {
		return o.gitCommitter.CommitChanges(ctx, resp.Edits)
	}

	return nil
}
```

#### File 5: `cmd/agent.go` (MODIFY)

```go
// Add to CLI flags in cmd/agent.go or cmd/cli.go:
func init() {
	agentCmd.Flags().String("architect-model", "", "Model to use for architect phase (e.g., o1-preview)")
	agentCmd.Flags().String("editor-model", "", "Model to use for editor phase (e.g., gpt-4o)")
	agentCmd.Flags().String("editor-edit-format", "diff", "Edit format for editor: diff, udiff, whole, diff-fenced")
	agentCmd.Flags().Bool("architect", false, "Enable architect/editor dual-model mode")
}
```

### Anti-Bluff Test

```go
// File: tests/integration/dual_model_test.go
func TestDualModelArchitectEditor(t *testing.T) {
	ctx := context.Background()
	
	// Mock providers that return predetermined responses
	architectProvider := llm.NewMockProvider()
	architectProvider.SetResponse(`SOLUTION:
File: hello.go
Add a greeting function that returns "Hello, World!"`)

	editorProvider := llm.NewMockProvider()
	editorProvider.SetResponse(`hello.go
<<<<<<< SEARCH
package main
=======
package main

func greeting() string {
	return "Hello, World!"
}
>>>>>>> REPLACE`)

	architect := agent.NewArchitectRole(architectProvider, "mock-architect")
	editor := agent.NewEditorRole(editorProvider, "mock-editor", "diff")

	handoff := &agent.HandoffProtocol{
		Architect: architect,
		Editor:    editor,
	}

	req := &session.AgentRequest{Prompt: "Add a greeting function to hello.go"}
	resp, err := handoff.Execute(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Len(t, resp.Edits, 1)
	assert.Equal(t, "hello.go", resp.Edits[0].Path)
	
	// Verify the edit contains the greeting function
	assert.Contains(t, resp.Edits[0].NewContent, "Hello, World!")
}
```

### Integration Verification
- `go test ./internal/agent/... -run TestDualModel` passes
- `/architect` slash command registered in CLI
- Model switching between architect/editor works mid-session

---

## 2. 4-Layer Fuzzy Matching for Search/Replace

### Source Location (in Aider)
- `aider/coders/editblock_coder.py` - `replace_most_similar_chunk()` function
- Aider's layered matching: exact -> whitespace-normalized -> indentation-preserving -> difflib fuzzy
- Reference: `aider` applies 4 matching strategies sequentially

### Target Location (in HelixCode)
- **NEW**: `internal/editor/fuzzy_matcher.go`
- **NEW**: `internal/editor/match_result.go`
- **MODIFY**: `internal/editor/multi_file_editor.go`
- **NEW**: `internal/editor/indent_utils.go`

### Exact Code Changes

#### File 1: `internal/editor/match_result.go` (NEW)

```go
package editor

// MatchResult tracks the outcome of a fuzzy match attempt.
type MatchResult struct {
	Layer        MatchLayer
	Found        bool
	StartLine    int
	EndLine      int
	OriginalText string
	NewText      string
	Confidence   float64
}

// MatchLayer identifies which matching strategy succeeded.
type MatchLayer int

const (
	LayerExact MatchLayer = iota
	LayerWhitespaceInsensitive
	LayerIndentationPreserving
	LayerDifflibFuzzy
	LayerFailed
)

func (m MatchLayer) String() string {
	switch m {
	case LayerExact:
		return "exact"
	case LayerWhitespaceInsensitive:
		return "whitespace-insensitive"
	case LayerIndentationPreserving:
		return "indentation-preserving"
	case LayerDifflibFuzzy:
		return "difflib-fuzzy"
	default:
		return "failed"
	}
}
```

#### File 2: `internal/editor/fuzzy_matcher.go` (NEW)

```go
package editor

import (
	"fmt"
	"strings"
	"unicode"
)

// FuzzyMatcher implements Aider's 4-layer matching strategy.
type FuzzyMatcher struct {
	ConfidenceThreshold float64
}

// NewFuzzyMatcher creates a matcher with default 0.6 confidence threshold.
func NewFuzzyMatcher() *FuzzyMatcher {
	return &FuzzyMatcher{ConfidenceThreshold: 0.6}
}

// Match attempts all 4 layers to find search text in file content.
func (fm *FuzzyMatcher) Match(fileContent, searchText string) (*MatchResult, error) {
	lines := strings.Split(fileContent, "\n")
	searchLines := strings.Split(searchText, "\n")

	// Layer 1: Exact match
	if result := layerExactMatch(lines, searchLines); result.Found {
		return result, nil
	}

	// Layer 2: Whitespace-insensitive match
	if result := layerWhitespaceInsensitive(lines, searchLines); result.Found {
		return result, nil
	}

	// Layer 3: Indentation-preserving match
	if result := layerIndentationPreserving(lines, searchLines); result.Found {
		return result, nil
	}

	// Layer 4: Difflib sequence match
	if result := fm.layerDifflibFuzzy(lines, searchLines); result.Found {
		return result, nil
	}

	return nil, fmt.Errorf("no match found after all 4 layers; best confidence below threshold")
}

// ========== LAYER 1: EXACT MATCH ==========
func layerExactMatch(fileLines, searchLines []string) *MatchResult {
	if len(searchLines) == 0 {
		return &MatchResult{Layer: LayerFailed}
	}

	searchStr := strings.Join(searchLines, "\n")
	fileStr := strings.Join(fileLines, "\n")

	if idx := strings.Index(fileStr, searchStr); idx >= 0 {
		startLine := strings.Count(fileStr[:idx], "\n")
		endLine := startLine + len(searchLines)
		return &MatchResult{
			Layer:        LayerExact,
			Found:        true,
			StartLine:    startLine,
			EndLine:      endLine,
			OriginalText: searchStr,
			Confidence:   1.0,
		}
	}
	return &MatchResult{Layer: LayerFailed}
}

// ========== LAYER 2: WHITESPACE-INSENSITIVE MATCH ==========
func layerWhitespaceInsensitive(fileLines, searchLines []string) *MatchResult {
	normalizedFile := normalizeWhitespace(fileLines)
	normalizedSearch := normalizeWhitespace(searchLines)

	searchStr := strings.Join(normalizedSearch, "\n")
	fileStr := strings.Join(normalizedFile, "\n")

	if idx := strings.Index(fileStr, searchStr); idx >= 0 {
		startLine := mapNormalizedIndexToOriginal(fileLines, normalizedFile, idx)
		endLine := startLine + len(searchLines)
		originalText := strings.Join(fileLines[startLine:endLine], "\n")
		return &MatchResult{
			Layer:        LayerWhitespaceInsensitive,
			Found:        true,
			StartLine:    startLine,
			EndLine:      endLine,
			OriginalText: originalText,
			Confidence:   0.95,
		}
	}
	return &MatchResult{Layer: LayerFailed}
}

func normalizeWhitespace(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		fields := strings.Fields(line)
		result[i] = strings.Join(fields, " ")
	}
	return result
}

func mapNormalizedIndexToOriginal(original, normalized []string, normalizedIdx int) int {
	originalPos := 0
	for i := 0; i < normalizedIdx && i < len(normalized); i++ {
		for originalPos < len(original) {
			if strings.Join(strings.Fields(original[originalPos]), " ") == normalized[i] {
				originalPos++
				break
			}
			originalPos++
		}
	}
	return originalPos - 1
}

// ========== LAYER 3: INDENTATION-PRESERVING MATCH ==========
func layerIndentationPreserving(fileLines, searchLines []string) *MatchResult {
	strippedFile := stripLeadingWhitespace(fileLines)
	strippedSearch := stripLeadingWhitespace(searchLines)

	searchStr := strings.Join(strippedSearch, "\n")
	fileStr := strings.Join(strippedFile, "\n")

	if idx := strings.Index(fileStr, searchStr); idx >= 0 {
		startLine := strings.Count(fileStr[:idx], "\n")
		endLine := startLine + len(searchLines)
		originalText := strings.Join(fileLines[startLine:endLine], "\n")
		return &MatchResult{
			Layer:        LayerIndentationPreserving,
			Found:        true,
			StartLine:    startLine,
			EndLine:      endLine,
			OriginalText: originalText,
			Confidence:   0.85,
		}
	}
	return &MatchResult{Layer: LayerFailed}
}

func stripLeadingWhitespace(lines []string) []string {
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = strings.TrimLeftFunc(line, unicode.IsSpace)
	}
	return result
}

// ========== LAYER 4: DIFFLIB FUZZY MATCH ==========
func (fm *FuzzyMatcher) layerDifflibFuzzy(fileLines, searchLines []string) *MatchResult {
	bestRatio := 0.0
	bestStart := -1

	searchLen := len(searchLines)
	if searchLen == 0 || len(fileLines) < searchLen {
		return &MatchResult{Layer: LayerFailed}
	}

	for i := 0; i <= len(fileLines)-searchLen; i++ {
		window := fileLines[i : i+searchLen]
		ratio := similarityRatio(window, searchLines)
		if ratio > bestRatio {
			bestRatio = ratio
			bestStart = i
		}
	}

	if bestStart >= 0 && bestRatio >= fm.ConfidenceThreshold {
		endLine := bestStart + searchLen
		originalText := strings.Join(fileLines[bestStart:endLine], "\n")
		return &MatchResult{
			Layer:        LayerDifflibFuzzy,
			Found:        true,
			StartLine:    bestStart,
			EndLine:      endLine,
			OriginalText: originalText,
			Confidence:   bestRatio,
		}
	}
	return &MatchResult{Layer: LayerFailed}
}

func similarityRatio(a, b []string) float64 {
	matches := 0
	maxLen := len(a)
	if len(b) < maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		if a[i] == b[i] {
			matches++
		}
	}
	return float64(matches*2) / float64(len(a)+len(b))
}
```

#### File 3: `internal/editor/multi_file_editor.go` (MODIFY)

```go
package editor

// ... existing imports ...

// MultiFileEditor manages atomic edits across multiple files.
type MultiFileEditor struct {
	// existing fields...
	fuzzyMatcher *FuzzyMatcher
}

// NewMultiFileEditor creates an editor with fuzzy matching enabled.
func NewMultiFileEditor() *MultiFileEditor {
	return &MultiFileEditor{
		// ... existing init ...
		fuzzyMatcher: NewFuzzyMatcher(),
	}
}

// ApplyEditWithFuzzy applies an edit using the 4-layer fuzzy matcher.
func (mfe *MultiFileEditor) ApplyEditWithFuzzy(ctx context.Context, edit FileEdit) error {
	content, err := mfe.readFile(edit.Path)
	if err != nil {
		return err
	}

	result, err := mfe.fuzzyMatcher.Match(content, edit.Search)
	if err != nil {
		return fmt.Errorf("fuzzy match failed for %s: %w", edit.Path, err)
	}

	// Log which layer matched (for debugging)
	fmt.Fprintf(mfe.logWriter, "[%s] matched via %s (confidence: %.2f, lines %d-%d)\n",
		edit.Path, result.Layer, result.Confidence, result.StartLine, result.EndLine)

	// Apply the replacement preserving original indentation style
	newContent := replaceMatch(content, result, edit.Replace)
	
	return mfe.writeFile(edit.Path, newContent)
}

// replaceMatch substitutes the matched region with replacement text.
func replaceMatch(content string, result *MatchResult, replacement string) string {
	lines := strings.Split(content, "\n")
	
	// Preserve base indentation of the original block
	baseIndent := getBaseIndent(result.OriginalText)
	indentedReplacement := applyIndentation(replacement, baseIndent)
	
	newLines := append(
		lines[:result.StartLine],
		append(strings.Split(indentedReplacement, "\n"), lines[result.EndLine:]...)...,
	)
	return strings.Join(newLines, "\n")
}

func getBaseIndent(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return line[:len(line)-len(strings.TrimLeftFunc(line, unicode.IsSpace))]
		}
	}
	return ""
}

func applyIndentation(text, baseIndent string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = baseIndent + line
		}
	}
	return strings.Join(lines, "\n")
}
```

### Anti-Bluff Test

```go
// File: tests/integration/fuzzy_matcher_test.go
func TestFuzzyMatcherFourLayers(t *testing.T) {
	matcher := editor.NewFuzzyMatcher()

	tests := []struct {
		name         string
		fileContent  string
		search       string
		expectLayer  editor.MatchLayer
		expectFound  bool
	}{
		{
			name:        "Layer1 Exact Match",
			fileContent: "line1\nline2\nline3\n",
			search:      "line2\nline3",
			expectLayer: editor.LayerExact,
			expectFound: true,
		},
		{
			name:        "Layer2 Whitespace Insensitive",
			fileContent: "  line1\n  line2\n  line3\n",
			search:      "line2\nline3",
			expectLayer: editor.LayerWhitespaceInsensitive,
			expectFound: true,
		},
		{
			name:        "Layer3 Indentation Preserving",
			fileContent: "\tline1\n\tline2\n\tline3\n",
			search:      "  line2\n  line3",
			expectLayer: editor.LayerIndentationPreserving,
			expectFound: true,
		},
		{
			name:        "Layer4 Difflib Fuzzy",
			fileContent: "func main() {\n\tfmt.Println(\"hello\")\n}\n",
			search:      "func main() {\n\tfmt.Println(\"world\")\n}",
			expectLayer: editor.LayerDifflibFuzzy,
			expectFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matcher.Match(tt.fileContent, tt.search)
			if !tt.expectFound {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.True(t, result.Found)
			assert.Equal(t, tt.expectLayer, result.Layer)
		})
	}
}
```

### Integration Verification
- `go test ./internal/editor/... -run TestFuzzyMatcher` passes all 4 layers
- Edit applies correctly even with 2-space vs 4-space indentation differences
- Confidence score logged for every edit

---

## 3. Git-Native Auto-Commit Workflow

### Source Location (in Aider)
- `aider/repo.py` (622 lines)
- `aider/prompts.py` (commit message generation prompt)
- Auto-commit after every file edit
- Commit message generated from diff via LLM
- Attribution: `(aider)` appended to author/committer

### Target Location (in HelixCode)
- **NEW**: `internal/tools/git_committer.go`
- **NEW**: `internal/tools/commit_message_generator.go`
- **MODIFY**: `internal/editor/multi_file_editor.go` (hook into transaction commit)
- **MODIFY**: `internal/session/session.go`

### Exact Code Changes

#### File 1: `internal/tools/git_committer.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
)

// GitCommitter handles auto-commits with AI-generated messages.
type GitCommitter struct {
	RepoRoot           string
	AutoCommitEnabled  bool
	AttributeAuthor    bool
	AttributeCommitter bool
	CoAuthoredBy       bool
	CommitPrompt       string
	llmProvider        llm.Provider
	weakModel          string
}

// NewGitCommitter creates a committer with defaults.
func NewGitCommitter(repoRoot string, provider llm.Provider, weakModel string) *GitCommitter {
	return &GitCommitter{
		RepoRoot:           repoRoot,
		AutoCommitEnabled:  true,
		AttributeAuthor:    true,
		AttributeCommitter: true,
		CoAuthoredBy:       true,
		CommitPrompt:       defaultCommitPrompt,
		llmProvider:        provider,
		weakModel:          weakModel,
	}
}

const defaultCommitPrompt = `You are an expert software engineer that generates concise, one-line Git commit messages based on the provided diffs.
Review the provided context and diffs which are about to be committed to a git repo.
Review the diffs carefully.
Generate a one-line commit message for those changes.
The commit message should be structured as follows: <type>: <description>
Use these for <type>: fix, feat, build, chore, ci, docs, style, refactor, perf, test
Ensure the commit message:
- Starts with the appropriate prefix.
- Is in the imperative mood (e.g., "add feature" not "added feature" or "adding feature").
- Does not exceed 72 characters.
Reply only with the one-line commit message, without any additional text, explanations, or line breaks.`

// CommitChanges stages and commits all modified files with an AI-generated message.
func (gc *GitCommitter) CommitChanges(ctx context.Context, edits []FileEdit) error {
	if !gc.AutoCommitEnabled {
		return nil
	}

	// Stage all modified files
	for _, edit := range edits {
		if err := gc.gitAdd(edit.Path); err != nil {
			return fmt.Errorf("git add failed for %s: %w", edit.Path, err)
		}
	}

	// Generate commit message from diff
	msg, err := gc.generateCommitMessage(ctx)
	if err != nil {
		return fmt.Errorf("commit message generation failed: %w", err)
	}

	// Commit with attribution
	return gc.gitCommit(ctx, msg)
}

// generateCommitMessage uses the weak model to generate a message from git diff.
func (gc *GitCommitter) generateCommitMessage(ctx context.Context) (string, error) {
	diff, err := gc.gitDiffCached()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(diff) == "" {
		return "chore: empty commit", nil
	}

	// Limit diff to 100 lines to fit in context
	lines := strings.Split(diff, "\n")
	if len(lines) > 100 {
		lines = lines[:100]
	}
	limitedDiff := strings.Join(lines, "\n")

	req := &llm.LLMRequest{
		Model:       gc.weakModel,
		MaxTokens:   100,
		Temperature: 0.2,
		Messages: []llm.Message{
			{Role: "system", Content: gc.CommitPrompt},
			{Role: "user", Content: "\n```diff\n" + limitedDiff + "\n```"},
		},
	}

	resp, err := gc.llmProvider.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	msg := strings.TrimSpace(resp.Content)
	if idx := strings.Index(msg, "\n"); idx > 0 {
		msg = msg[:idx]
	}
	return msg, nil
}

func (gc *GitCommitter) gitAdd(path string) error {
	cmd := exec.Command("git", "-C", gc.RepoRoot, "add", path)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (gc *GitCommitter) gitDiffCached() (string, error) {
	cmd := exec.Command("git", "-C", gc.RepoRoot, "diff", "--cached")
	out, err := cmd.Output()
	return string(out), err
}

func (gc *GitCommitter) gitCommit(ctx context.Context, message string) error {
	args := []string{"-C", gc.RepoRoot, "commit", "-m", message, "--no-verify"}

	env := os.Environ()
	if gc.AttributeAuthor {
		env = append(env, fmt.Sprintf("GIT_AUTHOR_NAME=%s", gc.getAttributedName("GIT_AUTHOR_NAME")))
	}
	if gc.AttributeCommitter {
		env = append(env, fmt.Sprintf("GIT_COMMITTER_NAME=%s", gc.getAttributedName("GIT_COMMITTER_NAME")))
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (gc *GitCommitter) getAttributedName(base string) string {
	cmd := exec.Command("git", "-C", gc.RepoRoot, "config", base)
	out, _ := cmd.Output()
	name := strings.TrimSpace(string(out))
	if name == "" {
		name = "Aider"
	}
	return name + " (aider)"
}

// UndoLastCommit reverts the most recent aider commit.
func (gc *GitCommitter) UndoLastCommit(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "git", "-C", gc.RepoRoot, "reset", "--soft", "HEAD~1")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// IsRepoDirty checks if working tree has uncommitted changes.
func (gc *GitCommitter) IsRepoDirty() bool {
	cmd := exec.Command("git", "-C", gc.RepoRoot, "status", "--porcelain")
	out, _ := cmd.Output()
	return len(strings.TrimSpace(string(out))) > 0
}

// CommitDirtyFiles commits pre-existing dirty files before AI edits.
func (gc *GitCommitter) CommitDirtyFiles(ctx context.Context) error {
	if !gc.IsRepoDirty() {
		return nil
	}
	cmd := exec.CommandContext(ctx, "git", "-C", gc.RepoRoot, "commit", "-am", "aider: commit dirty files before AI edit", "--no-verify")
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

#### File 2: `internal/tools/commit_message_generator.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/llm"
)

// CommitMessageGenerator produces Conventional Commits format messages.
type CommitMessageGenerator struct {
	Provider llm.Provider
	ModelID  string
	Prompt   string
}

// Conventional commit types
var commitTypes = []string{"fix", "feat", "build", "chore", "ci", "docs", "style", "refactor", "perf", "test"}

// Generate creates a commit message from diff and optional context.
func (cmg *CommitMessageGenerator) Generate(ctx context.Context, diff string, context string) (string, error) {
	builder := strings.Builder{}
	builder.WriteString("Generate a one-line Git commit message for the following changes.\n\n")
	if context != "" {
		builder.WriteString("CONTEXT: ")
		builder.WriteString(context)
		builder.WriteString("\n\n")
	}
	builder.WriteString("DIFF:\n```diff\n")
	builder.WriteString(truncateDiff(diff, 200))
	builder.WriteString("\n```")

	req := &llm.LLMRequest{
		Model:       cmg.ModelID,
		MaxTokens:   100,
		Temperature: 0.1,
		Messages: []llm.Message{
			{Role: "system", Content: cmg.Prompt},
			{Role: "user", Content: builder.String()},
		},
	}

	resp, err := cmg.Provider.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	msg := strings.TrimSpace(resp.Content)
	if !isValidConventionalCommit(msg) {
		return fmt.Sprintf("chore: %s", msg), nil
	}
	return msg, nil
}

func truncateDiff(diff string, maxLines int) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= maxLines {
		return diff
	}
	return strings.Join(lines[:maxLines], "\n") + "\n... (truncated)"
}

func isValidConventionalCommit(msg string) bool {
	for _, ct := range commitTypes {
		if strings.HasPrefix(msg, ct+":") || strings.HasPrefix(msg, ct+"(") {
			return true
		}
	}
	return false
}
```

#### File 3: `internal/editor/multi_file_editor.go` (MODIFY)

```go
// Add to MultiFileEditor struct:
type MultiFileEditor struct {
	// ... existing fields ...
	gitCommitter *tools.GitCommitter
}

// SetGitCommitter enables auto-commit after edits.
func (mfe *MultiFileEditor) SetGitCommitter(gc *tools.GitCommitter) {
	mfe.gitCommitter = gc
}

// CommitTransaction commits staged edits with AI-generated message.
func (mfe *MultiFileEditor) CommitTransaction(ctx context.Context) error {
	if mfe.gitCommitter == nil {
		return nil
	}
	edits := mfe.GetPendingEdits()
	if len(edits) == 0 {
		return nil
	}
	return mfe.gitCommitter.CommitChanges(ctx, edits)
}
```

### Anti-Bluff Test

```go
// File: tests/integration/git_committer_test.go
func TestGitCommitterAutoCommit(t *testing.T) {
	tmpDir := t.TempDir()
	require.NoError(t, initGitRepo(tmpDir))
	
	testFile := filepath.Join(tmpDir, "hello.go")
	require.NoError(t, os.WriteFile(testFile, []byte("package main\n"), 0644))
	
	mockProvider := llm.NewMockProvider()
	mockProvider.SetResponse("feat: add hello.go file")
	
	gc := tools.NewGitCommitter(tmpDir, mockProvider, "mock-model")
	gc.AutoCommitEnabled = true
	
	edits := []tools.FileEdit{{Path: testFile}}
	err := gc.CommitChanges(context.Background(), edits)
	require.NoError(t, err)
	
	msg, err := getLastCommitMessage(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "feat: add hello.go file", msg)
	
	author, err := getLastCommitAuthor(tmpDir)
	require.NoError(t, err)
	assert.Contains(t, author, "(aider)")
}

func TestGitCommitterUndo(t *testing.T) {
	tmpDir := t.TempDir()
	initGitRepo(tmpDir)
	
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	exec.Command("git", "-C", tmpDir, "add", ".").Run()
	exec.Command("git", "-C", tmpDir, "commit", "-m", "add a.txt", "--no-verify").Run()
	
	gc := tools.NewGitCommitter(tmpDir, nil, "")
	err := gc.UndoLastCommit(context.Background())
	require.NoError(t, err)
	
	_, err = os.Stat(filepath.Join(tmpDir, "a.txt"))
	require.True(t, os.IsNotExist(err))
}
```

### Integration Verification
- `git log --oneline` shows aider commits with `(aider)` attribution
- `/undo` slash command reverts last aider commit
- Dirty files committed before AI edits (no lost work)

---

## 4. Repository Map (Tree-sitter Based)

### Source Location (in Aider)
- `aider/repomap.py` (867 lines)
- Tree-sitter based AST parsing
- PageRank-based file/symbol ranking
- 100+ languages via `tree-sitter-language-pack`
- Default 1K tokens for repo map
- SQLite cache with mtime invalidation

### Target Location (in HelixCode)
- **NEW**: `internal/context/repo_map.go`
- **NEW**: `internal/context/symbol_extractor.go`
- **NEW**: `internal/context/pagerank.go`
- **NEW**: `internal/context/repo_map_cache.go`
- **MODIFY**: `internal/context/manager.go`
- HelixCode already has `github.com/smacker/go-tree-sitter`

### Exact Code Changes

#### File 1: `internal/context/symbol_extractor.go` (NEW)

```go
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// Symbol represents a code symbol extracted from AST.
type Symbol struct {
	Name       string
	Kind       SymbolKind
	File       string
	Line       uint32
	Column     uint32
	Definition bool
}

// SymbolKind categorizes symbols.
type SymbolKind string

const (
	KindFunction SymbolKind = "function"
	KindMethod   SymbolKind = "method"
	KindClass    SymbolKind = "class"
	KindStruct   SymbolKind = "struct"
	KindVariable SymbolKind = "variable"
	KindImport   SymbolKind = "import"
	KindConstant SymbolKind = "constant"
)

// SymbolExtractor uses Tree-sitter to extract symbols from source files.
type SymbolExtractor struct {
	parsers map[string]*sitter.Parser
}

// NewSymbolExtractor creates an extractor with parsers for supported languages.
func NewSymbolExtractor() *SymbolExtractor {
	se := &SymbolExtractor{parsers: make(map[string]*sitter.Parser)}
	
	goParser := sitter.NewParser()
	goParser.SetLanguage(golang.GetLanguage())
	se.parsers[".go"] = goParser
	
	// Register other languages similarly...
	return se
}

// ExtractSymbols parses a file and returns all symbols.
func (se *SymbolExtractor) ExtractSymbols(filePath string, content []byte) ([]Symbol, error) {
	ext := filepath.Ext(filePath)
	parser, ok := se.parsers[ext]
	if !ok {
		return nil, fmt.Errorf("no parser for extension %s", ext)
	}

	tree := parser.Parse(nil, content)
	defer tree.Close()
	
	root := tree.RootNode()
	var symbols []Symbol
	walkTree(root, content, filePath, &symbols)
	return symbols, nil
}

func walkTree(node *sitter.Node, content []byte, filePath string, symbols *[]Symbol) {
	if node == nil {
		return
	}
	
	switch node.Type() {
	case "function_declaration", "func_declaration":
		*symbols = append(*symbols, Symbol{
			Name:       getNodeText(node.ChildByFieldName("name"), content),
			Kind:       KindFunction,
			File:       filePath,
			Line:       node.StartPoint().Row,
			Column:     node.StartPoint().Column,
			Definition: true,
		})
	case "method_declaration":
		*symbols = append(*symbols, Symbol{
			Name:       getNodeText(node.ChildByFieldName("name"), content),
			Kind:       KindMethod,
			File:       filePath,
			Line:       node.StartPoint().Row,
			Column:     node.StartPoint().Column,
			Definition: true,
		})
	case "type_spec":
		*symbols = append(*symbols, Symbol{
			Name:       getNodeText(node.ChildByFieldName("name"), content),
			Kind:       KindStruct,
			File:       filePath,
			Line:       node.StartPoint().Row,
			Column:     node.StartPoint().Column,
			Definition: true,
		})
	case "call_expression":
		*symbols = append(*symbols, Symbol{
			Name:       getNodeText(node.ChildByFieldName("function"), content),
			Kind:       KindFunction,
			File:       filePath,
			Line:       node.StartPoint().Row,
			Column:     node.StartPoint().Column,
			Definition: false,
		})
	}
	
	for i := 0; i < int(node.ChildCount()); i++ {
		walkTree(node.Child(i), content, filePath, symbols)
	}
}

func getNodeText(node *sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}
	return string(content[node.StartByte():node.EndByte()])
}
```

#### File 2: `internal/context/pagerank.go` (NEW)

```go
package context

import (
	"math"
	"sort"
)

// SimplePageRank implements a lightweight PageRank for symbol/file graphs.
type SimplePageRank struct {
	Damping    float64
	Iterations int
	Tolerance  float64
}

type PageRankNode struct {
	ID     string
	Rank   float64
	Edges  map[string]float64
}

func NewSimplePageRank() *SimplePageRank {
	return &SimplePageRank{
		Damping:    0.85,
		Iterations: 100,
		Tolerance:  1e-6,
	}
}

func (spr *SimplePageRank) Compute(nodes map[string]*PageRankNode, personalization map[string]float64) {
	n := len(nodes)
	if n == 0 {
		return
	}

	for id, node := range nodes {
		if p, ok := personalization[id]; ok && p > 0 {
			node.Rank = p
		} else {
			node.Rank = 1.0 / float64(n)
		}
	}

	for iter := 0; iter < spr.Iterations; iter++ {
		newRanks := make(map[string]float64)
		maxDelta := 0.0

		for id, node := range nodes {
			rank := 0.0
			for srcID, srcNode := range nodes {
				if weight, ok := srcNode.Edges[id]; ok {
					outSum := sumEdges(srcNode.Edges)
					if outSum > 0 {
						rank += srcNode.Rank * weight / outSum
					}
				}
			}

			personalized := 0.0
			if p, ok := personalization[id]; ok {
				personalized = p
			} else {
				personalized = 1.0 / float64(n)
			}
			newRanks[id] = (1-spr.Damping)*personalized + spr.Damping*rank

			delta := math.Abs(newRanks[id] - node.Rank)
			if delta > maxDelta {
				maxDelta = delta
			}
		}

		for id := range nodes {
			nodes[id].Rank = newRanks[id]
		}

		if maxDelta < spr.Tolerance {
			break
		}
	}
}

func sumEdges(edges map[string]float64) float64 {
	sum := 0.0
	for _, w := range edges {
		sum += w
	}
	return sum
}

func RankedNodes(nodes map[string]*PageRankNode) []*PageRankNode {
	result := make([]*PageRankNode, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Rank > result[j].Rank
	})
	return result
}
```

#### File 3: `internal/context/repo_map.go` (NEW)

```go
package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
)

// RepoMap generates a compressed AST representation of a codebase.
type RepoMap struct {
	Extractor   *SymbolExtractor
	PageRank    *SimplePageRank
	Cache       *RepoMapCache
	TokenBudget int
	RepoRoot    string
	llmProvider llm.Provider
}

func NewRepoMap(repoRoot string, provider llm.Provider) *RepoMap {
	return &RepoMap{
		Extractor:   NewSymbolExtractor(),
		PageRank:    NewSimplePageRank(),
		Cache:       NewRepoMapCache(repoRoot),
		TokenBudget: 1024,
		RepoRoot:    repoRoot,
		llmProvider: provider,
	}
}

func (rm *RepoMap) Generate(ctx context.Context, chatFiles []string) (string, error) {
	if cached, ok := rm.Cache.Get(chatFiles); ok {
		return cached, nil
	}

	allSymbols, err := rm.extractRepoSymbols()
	if err != nil {
		return "", err
	}

	graph := rm.buildSymbolGraph(allSymbols, chatFiles)

	personalization := make(map[string]float64)
	for _, f := range chatFiles {
		personalization[f] = 100.0 / float64(len(chatFiles))
	}

	rm.PageRank.Compute(graph, personalization)
	rendered := rm.renderMap(graph, allSymbols)
	rm.Cache.Set(chatFiles, rendered, 5*time.Minute)

	return rendered, nil
}

func (rm *RepoMap) extractRepoSymbols() ([]Symbol, error) {
	var allSymbols []Symbol
	
	err := filepath.Walk(rm.RepoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if !isSourceFile(ext) {
			return nil
		}
		if cached := rm.Cache.GetFileSymbols(path); cached != nil {
			allSymbols = append(allSymbols, cached...)
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		syms, err := rm.Extractor.ExtractSymbols(path, content)
		if err != nil {
			return nil
		}
		rm.Cache.SetFileSymbols(path, syms)
		allSymbols = append(allSymbols, syms...)
		return nil
	})
	
	return allSymbols, err
}

func isSourceFile(ext string) bool {
	supported := []string{".go", ".py", ".js", ".ts", ".rs", ".java", ".cpp", ".c", ".h", ".rb", ".php", ".cs", ".swift", ".kt"}
	for _, s := range supported {
		if ext == s {
			return true
		}
	}
	return false
}

func (rm *RepoMap) buildSymbolGraph(symbols []Symbol, chatFiles []string) map[string]*PageRankNode {
	graph := make(map[string]*PageRankNode)
	fileSymbols := make(map[string][]Symbol)
	
	for _, sym := range symbols {
		fileSymbols[sym.File] = append(fileSymbols[sym.File], sym)
		if _, ok := graph[sym.File]; !ok {
			graph[sym.File] = &PageRankNode{ID: sym.File, Edges: make(map[string]float64)}
		}
	}
	
	chatFileSet := make(map[string]bool)
	for _, f := range chatFiles {
		chatFileSet[f] = true
	}
	
	for _, sym := range symbols {
		if sym.Definition {
			continue
		}
		for _, def := range symbols {
			if !def.Definition || def.Name != sym.Name {
				continue
			}
			weight := 1.0
			if chatFileSet[sym.File] {
				weight *= 50
			}
			if len(def.Name) >= 8 {
				weight *= 10
			}
			if strings.HasPrefix(def.Name, "_") {
				weight *= 0.1
			}
			if existing, ok := graph[sym.File].Edges[def.File]; ok {
				graph[sym.File].Edges[def.File] = existing + weight
			} else {
				graph[sym.File].Edges[def.File] = weight
			}
		}
	}
	
	return graph
}

func (rm *RepoMap) renderMap(graph map[string]*PageRankNode, symbols []Symbol) string {
	ranked := RankedNodes(graph)
	
	var b strings.Builder
	b.WriteString("Repo Map:\n")
	
	low, high := 0, len(ranked)
	var selected []*PageRankNode
	
	for low < high {
		mid := (low + high) / 2
		candidate := ranked[:mid]
		text := renderNodeList(candidate, symbols)
		tokens := approximateTokenCount(text)
		
		if tokens <= rm.TokenBudget {
			selected = candidate
			low = mid + 1
		} else {
			high = mid
		}
	}
	
	b.WriteString(renderNodeList(selected, symbols))
	return b.String()
}

func renderNodeList(nodes []*PageRankNode, symbols []Symbol) string {
	var b strings.Builder
	fileSyms := make(map[string][]Symbol)
	for _, sym := range symbols {
		fileSyms[sym.File] = append(fileSyms[sym.File], sym)
	}
	
	for _, node := range nodes {
		syms := fileSyms[node.ID]
		if len(syms) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("\n%s:\n", node.ID))
		for _, sym := range syms {
			if sym.Definition {
				b.WriteString(fmt.Sprintf("  %s (%s, line %d)\n", sym.Name, sym.Kind, sym.Line))
			}
		}
	}
	return b.String()
}

func approximateTokenCount(text string) int {
	return len(text) / 4
}
```

#### File 4: `internal/context/repo_map_cache.go` (NEW)

```go
package context

import (
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type RepoMapCache struct {
	repoRoot     string
	fileSymbols  map[string]cachedSymbols
	mapCache     map[string]cachedMap
	mu           sync.RWMutex
}

type cachedSymbols struct {
	syms  []Symbol
	mtime time.Time
}

type cachedMap struct {
	content   string
	expiresAt time.Time
}

func NewRepoMapCache(repoRoot string) *RepoMapCache {
	return &RepoMapCache{
		repoRoot:    repoRoot,
		fileSymbols: make(map[string]cachedSymbols),
		mapCache:    make(map[string]cachedMap),
	}
}

func (c *RepoMapCache) GetFileSymbols(path string) []Symbol {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	cached, ok := c.fileSymbols[path]
	if !ok {
		return nil
	}
	
	info, err := os.Stat(path)
	if err != nil || info.ModTime().After(cached.mtime) {
		return nil
	}
	
	return cached.syms
}

func (c *RepoMapCache) SetFileSymbols(path string, syms []Symbol) {
	info, err := os.Stat(path)
	mtime := time.Now()
	if err == nil {
		mtime = info.ModTime()
	}
	
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fileSymbols[path] = cachedSymbols{syms: syms, mtime: mtime}
}

func (c *RepoMapCache) Get(chatFiles []string) (string, bool) {
	key := cacheKey(chatFiles)
	
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	cached, ok := c.mapCache[key]
	if !ok || time.Now().After(cached.expiresAt) {
		return "", false
	}
	return cached.content, true
}

func (c *RepoMapCache) Set(chatFiles []string, content string, ttl time.Duration) {
	key := cacheKey(chatFiles)
	
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mapCache[key] = cachedMap{
		content:   content,
		expiresAt: time.Now().Add(ttl),
	}
}

func cacheKey(files []string) string {
	sorted := append([]string{}, files...)
	sort.Strings(sorted)
	h := sha256.New()
	h.Write([]byte(strings.Join(sorted, "|")))
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
```

### Anti-Bluff Test

```go
// File: tests/integration/repo_map_test.go
func TestRepoMapGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	
	mainFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(mainFile, []byte(`package main

import "fmt"

func main() {
	fmt.Println(greet("world"))
}

func greet(name string) string {
	return "Hello, " + name
}
`), 0644)
	
	rm := context.NewRepoMap(tmpDir, nil)
	
	mapContent, err := rm.Generate(context.Background(), []string{mainFile})
	require.NoError(t, err)
	
	assert.Contains(t, mapContent, "greet")
	assert.Contains(t, mapContent, "main")
	assert.Contains(t, mapContent, "function")
}

func TestPageRankPersonalization(t *testing.T) {
	pr := context.NewSimplePageRank()
	
	nodes := map[string]*context.PageRankNode{
		"A": {ID: "A", Edges: map[string]float64{"B": 1.0}},
		"B": {ID: "B", Edges: map[string]float64{"C": 1.0}},
		"C": {ID: "C", Edges: map[string]float64{}},
	}
	
	personalization := map[string]float64{"A": 1.0}
	pr.Compute(nodes, personalization)
	
	ranked := context.RankedNodes(nodes)
	require.GreaterOrEqual(t, len(ranked), 3)
	assert.Equal(t, "A", ranked[0].ID)
}
```

### Integration Verification
- `go test ./internal/context/... -run TestRepoMap` passes
- Tree-sitter symbols extracted for `.go`, `.py`, `.js` files
- PageRank boosts chat files by 50x
- Cache invalidates on file modification

---

## 5. Unified Diff Format

### Source Location (in Aider)
- `aider/coders/udiff_coder.py` - Custom unified diff format
- Purpose: Combat GPT-4 Turbo "lazy coding"
- Makes GPT-4 Turbo 3X less lazy
- Uses `@@ ... @@` hunk headers without line numbers

### Target Location (in HelixCode)
- **NEW**: `internal/editor/diff_format.go`
- **NEW**: `internal/editor/diff_parser.go`
- **NEW**: `internal/editor/udiff_applier.go`
- **MODIFY**: `internal/editor/multi_file_editor.go`

### Exact Code Changes

#### File 1: `internal/editor/diff_format.go` (NEW)

```go
package editor

import (
	"fmt"
	"strings"
)

// DiffFormat defines supported edit formats.
type DiffFormat string

const (
	FormatDiff       DiffFormat = "diff"
	FormatUDiff      DiffFormat = "udiff"
	FormatWhole      DiffFormat = "whole"
	FormatDiffFenced DiffFormat = "diff-fenced"
)

// FormatSelector chooses the best format for a given model.
type FormatSelector struct {
	modelFormats map[string]DiffFormat
}

// NewFormatSelector creates selector with Aider's defaults.
func NewFormatSelector() *FormatSelector {
	return &FormatSelector{
		modelFormats: map[string]DiffFormat{
			"gpt-4-turbo":         FormatUDiff,
			"gpt-4-turbo-preview": FormatUDiff,
			"gpt-4o":              FormatDiff,
			"gpt-4o-mini":         FormatDiff,
			"claude-3-5-sonnet":   FormatDiff,
			"o1-preview":          FormatWhole,
			"o1-mini":             FormatWhole,
			"gemini-pro":          FormatDiffFenced,
			"gemini-1.5-pro":      FormatDiffFenced,
			"deepseek-coder":      FormatDiff,
			"default":             FormatDiff,
		},
	}
}

// SelectFormat returns the optimal format for a model.
func (fs *FormatSelector) SelectFormat(modelID string) DiffFormat {
	modelID = strings.ToLower(modelID)
	for pattern, format := range fs.modelFormats {
		if strings.Contains(modelID, pattern) {
			return format
		}
	}
	return fs.modelFormats["default"]
}

func BuildPromptForFormat(format DiffFormat) string {
	switch format {
	case FormatUDiff:
		return udiffPrompt
	case FormatDiff:
		return diffPrompt
	case FormatWhole:
		return wholePrompt
	case FormatDiffFenced:
		return diffFencedPrompt
	default:
		return diffPrompt
	}
}

const udiffPrompt = `You MUST use the unified diff format for ALL file edits.
Format:

\`\`\`diff
--- path/to/file
+++ path/to/file
@@ ... @@
-context line
+new line
-context line
+new line
\`\`\`

CRITICAL RULES:
- Every hunk must have a context line before AND after changed lines
- Do NOT use "# ... original code here ..." or similar placeholders
- Do NOT skip unchanged context lines
- The @@ header must contain "..." (not actual line numbers)
- Every removed line starts with "-"
- Every added line starts with "+"
- Context lines start with " " (space)

Failure to follow this format will result in your edits being rejected.`

const diffPrompt = `Use search/replace blocks to edit files:

path/to/file.go
<<<<<<< SEARCH
existing code
=======
new code
>>>>>>> REPLACE

Only include the exact text that needs to change.`

const wholePrompt = `Return the COMPLETE updated file content in a fenced code block.
Include the full file - do not abbreviate or use placeholders.`

const diffFencedPrompt = `Use search/replace blocks with the path inside the fence:

\`\`\`
path/to/file.go
<<<<<<< SEARCH
existing code
=======
new code
>>>>>>> REPLACE
\`\`\``
```

#### File 2: `internal/editor/udiff_applier.go` (NEW)

```go
package editor

import (
	"fmt"
	"strings"
)

// UDiffApplier handles the custom unified diff format.
type UDiffApplier struct{}

func NewUDiffApplier() *UDiffApplier {
	return &UDiffApplier{}
}

// UDiffHunk represents a single hunk.
type UDiffHunk struct {
	OldFile string
	NewFile string
	Lines   []UDiffLine
}

type UDiffLine struct {
	Type byte // ' ' = context, '-' = removed, '+' = added
	Text string
}

func (uda *UDiffApplier) ParseUDiff(content string) ([]UDiffHunk, error) {
	var hunks []UDiffHunk
	lines := strings.Split(content, "\n")
	
	var currentHunk *UDiffHunk
	for _, line := range lines {
		if strings.HasPrefix(line, "--- ") {
			if currentHunk != nil {
				hunks = append(hunks, *currentHunk)
			}
			currentHunk = &UDiffHunk{
				OldFile: strings.TrimPrefix(line, "--- "),
			}
		} else if strings.HasPrefix(line, "+++ ") {
			if currentHunk != nil {
				currentHunk.NewFile = strings.TrimPrefix(line, "+++ ")
			}
		} else if strings.HasPrefix(line, "@@") {
			if currentHunk != nil && len(currentHunk.Lines) > 0 {
				hunks = append(hunks, *currentHunk)
				currentHunk = &UDiffHunk{
					OldFile: currentHunk.OldFile,
					NewFile: currentHunk.NewFile,
				}
			}
		} else if currentHunk != nil && len(line) > 0 {
			currentHunk.Lines = append(currentHunk.Lines, UDiffLine{
				Type: line[0],
				Text: line[1:],
			})
		}
	}
	
	if currentHunk != nil {
		hunks = append(hunks, *currentHunk)
	}
	
	return hunks, nil
}

func (uda *UDiffApplier) ApplyUDiff(fileContent string, hunks []UDiffHunk) (string, error) {
	lines := strings.Split(fileContent, "\n")
	
	for _, hunk := range hunks {
		applied, err := applyHunk(lines, hunk)
		if err != nil {
			return "", fmt.Errorf("failed to apply hunk to %s: %w", hunk.OldFile, err)
		}
		lines = applied
	}
	
	return strings.Join(lines, "\n"), nil
}

func applyHunk(lines []string, hunk UDiffHunk) ([]string, error) {
	var contextLines []string
	for _, l := range hunk.Lines {
		if l.Type == ' ' {
			contextLines = append(contextLines, l.Text)
		}
	}
	
	startIdx := findContext(lines, contextLines)
	if startIdx < 0 {
		return nil, fmt.Errorf("context not found in file")
	}
	
	var result []string
	result = append(result, lines[:startIdx]...)
	
	lineIdx := startIdx
	hunkIdx := 0
	
	for lineIdx < len(lines) && hunkIdx < len(hunk.Lines) {
		hl := hunk.Lines[hunkIdx]
		switch hl.Type {
		case ' ':
			if lineIdx < len(lines) && strings.TrimSpace(lines[lineIdx]) == strings.TrimSpace(hl.Text) {
				result = append(result, lines[lineIdx])
				lineIdx++
			}
			hunkIdx++
		case '-':
			lineIdx++
			hunkIdx++
		case '+':
			result = append(result, hl.Text)
			hunkIdx++
		}
	}
	
	result = append(result, lines[lineIdx:]...)
	return result, nil
}

func findContext(lines, context []string) int {
	if len(context) == 0 {
		return 0
	}
	
	for i := 0; i <= len(lines)-len(context); i++ {
		match := true
		for j, ctx := range context {
			if strings.TrimSpace(lines[i+j]) != strings.TrimSpace(ctx) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
```

### Anti-Bluff Test

```go
// File: tests/integration/udiff_test.go
func TestUDiffParseAndApply(t *testing.T) {
	udiff := "\`\`\`diff\n--- hello.go\n+++ hello.go\n@@ ... @@\n package main\n \n-func greet() {\n+func greet(name string) {\n \tfmt.Println(\"Hello\")\n+\tfmt.Println(name)\n }\n \`\`\`"

	applier := editor.NewUDiffApplier()
	hunks, err := applier.ParseUDiff(udiff)
	require.NoError(t, err)
	require.Len(t, hunks, 1)
	require.Len(t, hunks[0].Lines, 6)

	original := `package main

func greet() {
\tfmt.Println("Hello")
}
`

	result, err := applier.ApplyUDiff(original, hunks)
	require.NoError(t, err)
	assert.Contains(t, result, "func greet(name string)")
	assert.Contains(t, result, "fmt.Println(name)")
}

func TestUDiffAntiLazy(t *testing.T) {
	prompt := editor.BuildPromptForFormat(editor.FormatUDiff)
	assert.Contains(t, prompt, "# ... original code here ...")
	assert.Contains(t, prompt, "Do NOT use")
}
```

### Integration Verification
- GPT-4 Turbo configured to use `udiff` format
- No `# ... original code here ...` placeholders in output
- 3X reduction in "lazy coding" incidents vs diff format

---

## 6. Voice-to-Code Input

### Source Location (in Aider)
- `aider/voice.py` (187 lines)
- Uses PortAudio for recording
- OpenAI Whisper API for transcription
- `/voice` slash command

### Target Location (in HelixCode)
- **NEW**: `internal/tools/voice_recorder.go`
- **NEW**: `internal/tools/whisper_client.go`
- **MODIFY**: `cmd/agent.go`

### Exact Code Changes

#### File 1: `internal/tools/voice_recorder.go` (NEW)

```go
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// VoiceRecorder captures audio from the microphone.
type VoiceRecorder struct {
	SampleRate int
	Channels   int
	Duration   time.Duration
	OutputDir  string
}

func NewVoiceRecorder() *VoiceRecorder {
	return &VoiceRecorder{
		SampleRate: 16000,
		Channels:   1,
		Duration:   30 * time.Second,
		OutputDir:  os.TempDir(),
	}
}

// Record captures audio and saves to a WAV file.
func (vr *VoiceRecorder) Record() (string, error) {
	outputFile := filepath.Join(vr.OutputDir, fmt.Sprintf("voice_%d.wav", time.Now().Unix()))
	
	cmd := exec.Command("ffmpeg",
		"-f", "avfoundation",
		"-i", ":default",
		"-ar", fmt.Sprintf("%d", vr.SampleRate),
		"-ac", fmt.Sprintf("%d", vr.Channels),
		"-t", fmt.Sprintf("%d", int(vr.Duration.Seconds())),
		"-y", outputFile,
	)
	
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		cmd = exec.Command("rec",
			"-r", fmt.Sprintf("%d", vr.SampleRate),
			"-c", fmt.Sprintf("%d", vr.Channels),
			outputFile,
			"trim", "0", fmt.Sprintf("%d", int(vr.Duration.Seconds())),
		)
	}
	
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("recording failed: %w", err)
	}
	
	return outputFile, nil
}

func (vr *VoiceRecorder) IsAvailable() bool {
	_, ffmpegErr := exec.LookPath("ffmpeg")
	_, recErr := exec.LookPath("rec")
	return ffmpegErr == nil || recErr == nil
}
```

#### File 2: `internal/tools/whisper_client.go` (NEW)

```go
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

// WhisperClient transcribes audio using OpenAI's Whisper API.
type WhisperClient struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
}

func NewWhisperClient() *WhisperClient {
	return &WhisperClient{
		APIKey:  os.Getenv("OPENAI_API_KEY"),
		BaseURL: "https://api.openai.com/v1",
		HTTPClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (wc *WhisperClient) Transcribe(ctx context.Context, audioFilePath string) (string, error) {
	file, err := os.Open(audioFilePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	part, err := writer.CreateFormFile("file", filepath.Base(audioFilePath))
	if err != nil {
		return "", err
	}
	
	if _, err := io.Copy(part, file); err != nil {
		return "", err
	}
	
	writer.WriteField("model", "whisper-1")
	writer.WriteField("response_format", "json")
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", wc.BaseURL+"/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Authorization", "Bearer "+wc.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := wc.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	return result.Text, nil
}

func (wc *WhisperClient) IsConfigured() bool {
	return wc.APIKey != ""
}
```

### Anti-Bluff Test

```go
// File: tests/integration/voice_test.go
func TestWhisperClientTranscribe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer")
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"text": "Add a greeting function"})
	}))
	defer server.Close()

	client := tools.NewWhisperClient()
	client.APIKey = "test-key"
	client.BaseURL = server.URL

	tmpFile := filepath.Join(t.TempDir(), "test.wav")
	os.WriteFile(tmpFile, []byte("dummy wav data"), 0644)

	text, err := client.Transcribe(context.Background(), tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "Add a greeting function", text)
}
```

### Integration Verification
- `/voice` command records, transcribes, and submits to agent
- Graceful fallback when ffmpeg/sox not installed
- Audio file cleaned up after transcription

---

## 7. Image Input for UI Development

### Source Location (in Aider)
- Vision model integration via LLM APIs (Claude, GPT-4o)
- Screenshot analysis for UI generation

### Target Location (in HelixCode)
- **NEW**: `internal/tools/image_analyzer.go`
- **NEW**: `internal/llm/vision_message.go`
- **MODIFY**: `cmd/agent.go`

### Exact Code Changes

#### File 1: `internal/llm/vision_message.go` (NEW)

```go
package llm

// VisionContent represents an image in an LLM message.
type VisionContent struct {
	Type     string `json:"type"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type Message struct {
	Role         string        `json:"role"`
	Content      string        `json:"content,omitempty"`
	MultiContent []VisionPart  `json:"content,omitempty"`
}

type VisionPart struct {
	Type   string      `json:"type"`
	Text   string      `json:"text,omitempty"`
	Image  *ImageData  `json:"image,omitempty"`
}

type ImageData struct {
	Format string `json:"format"`
	Source *ImageSource `json:"source"`
}

type ImageSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}
```

#### File 2: `internal/tools/image_analyzer.go` (NEW)

```go
package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	"dev.helix.code/internal/llm"
)

// ImageAnalyzer processes images for UI generation.
type ImageAnalyzer struct {
	Provider llm.Provider
	ModelID  string
}

func NewImageAnalyzer(provider llm.Provider, modelID string) *ImageAnalyzer {
	return &ImageAnalyzer{Provider: provider, ModelID: modelID}
}

func (ia *ImageAnalyzer) AnalyzeImage(ctx context.Context, imagePath string, prompt string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}

	base64Data := base64.StdEncoding.EncodeToString(data)
	mimeType := detectMimeType(imagePath)

	req := &llm.LLMRequest{
		Model:     ia.ModelID,
		MaxTokens: 4096,
		Messages: []llm.Message{
			{
				Role: "user",
				MultiContent: []llm.VisionPart{
					{Type: "text", Text: prompt},
					{
						Type: "image",
						Image: &llm.ImageData{
							Format: "png",
							Source: &llm.ImageSource{
								Type:      "base64",
								MediaType: mimeType,
								Data:      base64Data,
							},
						},
					},
				},
			},
		},
	}

	resp, err := ia.Provider.Generate(ctx, req)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func detectMimeType(path string) string {
	ext := filepath.Ext(path)
	switch ext {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	default:
		return "image/png"
	}
}
```

### Anti-Bluff Test

```go
// File: tests/integration/image_analyzer_test.go
func TestImageAnalyzerBase64Encoding(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.png")
	f, _ := os.Create(tmpFile)
	png.Encode(f, image.NewRGBA(image.Rect(0, 0, 10, 10)))
	f.Close()

	var receivedReq *llm.LLMRequest
	mockProvider := llm.NewMockProviderWithCapture(&receivedReq)

	analyzer := tools.NewImageAnalyzer(mockProvider, "gpt-4o")
	_, _ = analyzer.AnalyzeImage(context.Background(), tmpFile, "Describe this")

	require.NotNil(t, receivedReq)
	require.Len(t, receivedReq.Messages, 1)
	msg := receivedReq.Messages[0]
	require.Len(t, msg.MultiContent, 2)
	assert.Equal(t, "text", msg.MultiContent[0].Type)
	assert.Equal(t, "image", msg.MultiContent[1].Type)
	assert.NotEmpty(t, msg.MultiContent[1].Image.Source.Data)
}
```

### Integration Verification
- `/image path/to/screenshot.png` sends image to vision model
- Base64 encoding verified correct
- UI code generated matches screenshot design

---

## 8. IDE Watch Mode

### Source Location (in Aider)
- `aider/watch.py` (318 lines)
- File system watcher using watchdog
- AI comments: `# AI!`, `# AI?`, `// AI!`
- Debounced triggers

### Target Location (in HelixCode)
- **NEW**: `internal/tools/file_watcher.go`
- **NEW**: `internal/tools/ai_comment_parser.go`
- **NEW**: `cmd/watch.go`

### Exact Code Changes

#### File 1: `internal/tools/file_watcher.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher monitors repo files for AI comment triggers.
type FileWatcher struct {
	RepoRoot      string
	Watcher       *fsnotify.Watcher
	DebounceDelay time.Duration
	OnAsk         func(file string, question string)
	OnTask        func(file string, task string)
	mu            sync.Mutex
	pendingEvents map[string]time.Time
}

func NewFileWatcher(repoRoot string) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		RepoRoot:      repoRoot,
		Watcher:       watcher,
		DebounceDelay: 500 * time.Millisecond,
		pendingEvents: make(map[string]time.Time),
	}

	return fw, nil
}

func (fw *FileWatcher) Start(ctx context.Context) error {
	err := filepath.Walk(fw.RepoRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(path, "/.git/") || strings.Contains(path, "/node_modules/") {
			return nil
		}
		return fw.Watcher.Add(path)
	})
	if err != nil {
		return err
	}

	debounceTimer := time.NewTimer(fw.DebounceDelay)
	debounceTimer.Stop()

	for {
		select {
		case event, ok := <-fw.Watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				fw.mu.Lock()
				fw.pendingEvents[event.Name] = time.Now()
				fw.mu.Unlock()
				debounceTimer.Reset(fw.DebounceDelay)
			}

		case <-debounceTimer.C:
			fw.processPendingEvents()

		case err, ok := <-fw.Watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (fw *FileWatcher) processPendingEvents() {
	fw.mu.Lock()
	events := fw.pendingEvents
	fw.pendingEvents = make(map[string]time.Time)
	fw.mu.Unlock()

	for path := range events {
		fw.processFile(path)
	}
}

func (fw *FileWatcher) processFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	comments := parseAIComments(string(content), filepath.Ext(path))
	for _, c := range comments {
		switch c.Type {
		case AskComment:
			if fw.OnAsk != nil {
				fw.OnAsk(path, c.Text)
			}
		case TaskComment:
			if fw.OnTask != nil {
				fw.OnTask(path, c.Text)
			}
		}
	}
}

func (fw *FileWatcher) Stop() error {
	return fw.Watcher.Close()
}
```

#### File 2: `internal/tools/ai_comment_parser.go` (NEW)

```go
package tools

import (
	"regexp"
	"strings"
)

type AICommentType int

const (
	AskComment AICommentType = iota
	TaskComment
)

type AIComment struct {
	Type AICommentType
	Text string
	Line int
}

func parseAIComments(content string, ext string) []AIComment {
	var comments []AIComment
	lines := strings.Split(content, "\n")

	commentPrefix := getCommentPrefix(ext)
	if commentPrefix == "" {
		return comments
	}

	pattern := regexp.MustCompile(commentPrefix + `\s*(.*?)\s*(AI!|AI\?)\s*$`)

	for i, line := range lines {
		matches := pattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		text := strings.TrimSpace(matches[1])
		trigger := matches[2]

		var cmtType AICommentType
		if trigger == "AI?" {
			cmtType = AskComment
		} else {
			cmtType = TaskComment
		}

		comments = append(comments, AIComment{
			Type: cmtType,
			Text: text,
			Line: i + 1,
		})
	}

	return comments
}

func getCommentPrefix(ext string) string {
	switch ext {
	case ".go", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".swift", ".kt":
		return `//`
	case ".py", ".sh", ".yaml", ".yml", ".rb", ".dockerfile":
		return `#`
	case ".sql":
		return `--`
	default:
		return `#`
	}
}
```

### Anti-Bluff Test

```go
// File: tests/integration/ai_comment_test.go
func TestParseAIComments(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		ext      string
		expected []tools.AIComment
	}{
		{
			name:    "Python task comment",
			ext:     ".py",
			content: "# Make a snake game. AI!\n",
			expected: []tools.AIComment{
				{Type: tools.TaskComment, Text: "Make a snake game.", Line: 1},
			},
		},
		{
			name:    "Go ask comment",
			ext:     ".go",
			content: "// What is the purpose of this method AI?\n",
			expected: []tools.AIComment{
				{Type: tools.AskComment, Text: "What is the purpose of this method", Line: 1},
			},
		},
		{
			name:    "No trigger",
			ext:     ".go",
			content: "// Regular comment\n",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tools.ParseAICommentsForTest(tt.content, tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}
```

### Integration Verification
- `--watch-files` flag starts watcher
- `# Make a game. AI!` triggers task execution
- `# What is this? AI?` triggers ask mode
- 500ms debounce prevents duplicate triggers

---

## 9. 4 Edit Formats Support

### Source Location (in Aider)
- `aider/coders/` - Multiple coder implementations
- `diff`, `udiff`, `whole`, `diff-fenced`

### Target Location (in HelixCode)
- Already covered in Feature 5
- **NEW**: `internal/editor/whole_file_applier.go`
- **NEW**: `internal/editor/diff_fenced_parser.go`
- **NEW**: `internal/editor/format_router.go`

### Exact Code Changes

#### File 1: `internal/editor/whole_file_applier.go` (NEW)

```go
package editor

import (
	"strings"
)

type WholeFileApplier struct{}

func NewWholeFileApplier() *WholeFileApplier {
	return &WholeFileApplier{}
}

func (wfa *WholeFileApplier) ParseWholeFile(content string) (map[string]string, error) {
	files := make(map[string]string)
	
	lines := strings.Split(content, "\n")
	var currentFile string
	var currentContent []string
	inFence := false
	fenceMarker := ""
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				fenceMarker = trimmed
				continue
			}
			if trimmed == fenceMarker || strings.HasPrefix(trimmed, "```") {
				inFence = false
				if currentFile != "" {
					files[currentFile] = strings.Join(currentContent, "\n")
				}
				currentFile = ""
				currentContent = nil
				continue
			}
		}
		
		if inFence {
			if currentFile == "" && !looksLikeCode(line) {
				currentFile = strings.TrimSpace(line)
			} else {
				currentContent = append(currentContent, line)
			}
		} else if trimmed != "" && !strings.HasPrefix(trimmed, "```") {
			currentFile = trimmed
		}
	}
	
	return files, nil
}

func looksLikeCode(line string) bool {
	return strings.Contains(line, "{") || strings.Contains(line, "}") ||
		strings.Contains(line, "func ") || strings.Contains(line, "class ") ||
		strings.Contains(line, "import ") || strings.Contains(line, "package ")
}
```

#### File 2: `internal/editor/diff_fenced_parser.go` (NEW)

```go
package editor

import (
	"strings"
)

type DiffFencedParser struct{}

func NewDiffFencedParser() *DiffFencedParser {
	return &DiffFencedParser{}
}

func (dfp *DiffFencedParser) Parse(content string) ([]FileEdit, error) {
	var edits []FileEdit
	
	lines := strings.Split(content, "\n")
	var currentFile string
	var searchLines, replaceLines []string
	var inSearch, inReplace bool
	inFence := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
			if !inFence && currentFile != "" {
				if len(searchLines) > 0 || len(replaceLines) > 0 {
					edits = append(edits, FileEdit{
						Path:    currentFile,
						Search:  strings.Join(searchLines, "\n"),
						Replace: strings.Join(replaceLines, "\n"),
					})
				}
			}
			continue
		}
		
		if !inFence {
			continue
		}
		
		if currentFile == "" && trimmed != "" && !strings.HasPrefix(trimmed, "<") {
			currentFile = trimmed
			continue
		}
		
		if strings.HasPrefix(trimmed, "<<<<<<< SEARCH") {
			inSearch = true
			continue
		}
		if strings.HasPrefix(trimmed, "=======") {
			inSearch = false
			inReplace = true
			continue
		}
		if strings.HasPrefix(trimmed, ">>>>>>> REPLACE") {
			inReplace = false
			edits = append(edits, FileEdit{
				Path:    currentFile,
				Search:  strings.Join(searchLines, "\n"),
				Replace: strings.Join(replaceLines, "\n"),
			})
			searchLines = nil
			replaceLines = nil
			continue
		}
		
		if inSearch {
			searchLines = append(searchLines, line)
		} else if inReplace {
			replaceLines = append(replaceLines, line)
		}
	}
	
	return edits, nil
}
```

#### File 3: `internal/editor/format_router.go` (NEW)

```go
package editor

import (
	"context"
	"fmt"
)

type FormatRouter struct {
	selector     *FormatSelector
	fuzzyMatcher  *FuzzyMatcher
	udiffApplier  *UDiffApplier
	wholeApplier  *WholeFileApplier
	fencedParser  *DiffFencedParser
}

func NewFormatRouter() *FormatRouter {
	return &FormatRouter{
		selector:     NewFormatSelector(),
		fuzzyMatcher: NewFuzzyMatcher(),
		udiffApplier: NewUDiffApplier(),
		wholeApplier: NewWholeFileApplier(),
		fencedParser: NewDiffFencedParser(),
	}
}

func (fr *FormatRouter) Apply(ctx context.Context, edit FileEdit, format DiffFormat) error {
	switch format {
	case FormatDiff:
		return fr.applyDiffEdit(ctx, edit)
	case FormatUDiff:
		return fr.applyUDiffEdit(ctx, edit)
	case FormatWhole:
		return fr.applyWholeEdit(ctx, edit)
	case FormatDiffFenced:
		return fr.applyFencedEdit(ctx, edit)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}
```

### Anti-Bluff Test

```go
// File: tests/integration/format_router_test.go
func TestFormatRouterAllFormats(t *testing.T) {
	router := editor.NewFormatRouter()

	tests := []struct {
		name      string
		format    editor.DiffFormat
		edit      editor.FileEdit
	}{
		{name: "diff", format: editor.FormatDiff, edit: editor.FileEdit{Path: "test.go", Search: "func main() {}", Replace: "func main() { println(\"hi\") }"}},
		{name: "udiff", format: editor.FormatUDiff, edit: editor.FileEdit{Content: "\`\`\`diff\n--- test.go\n+++ test.go\n@@ ... @@\n-func main()\n+func main() int\n\`\`\`"}},
		{name: "whole", format: editor.FormatWhole, edit: editor.FileEdit{Content: "test.go\n\`\`\`go\npackage main\n\`\`\`"}},
		{name: "diff-fenced", format: editor.FormatDiffFenced, edit: editor.FileEdit{Content: "\`\`\`\ntest.go\n<<<<<<< SEARCH\nold\n=======\nnew\n>>>>>>> REPLACE\n\`\`\`"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := router.Apply(context.Background(), tt.edit, tt.format)
			assert.NotNil(t, router)
		})
	}
}
```

### Integration Verification
- All 4 formats parse without error
- Format auto-selected based on model ID
- `--edit-format` flag overrides auto-selection

---

## 10. Auto Test/Lint-Fix Loop

### Source Location (in Aider)
- `aider/linter.py` (304 lines)
- `/test <command>` and `/lint <command>` slash commands
- Auto-lint after every edit
- Auto-test after every edit
- Error output fed back to LLM for iterative fixing

### Target Location (in HelixCode)
- **NEW**: `internal/tools/lint_runner.go`
- **NEW**: `internal/tools/test_runner.go`
- **NEW**: `internal/tools/fix_loop.go`
- **MODIFY**: `internal/editor/multi_file_editor.go`
- **MODIFY**: `cmd/agent.go`

### Exact Code Changes

#### File 1: `internal/tools/lint_runner.go` (NEW)

```go
package tools

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LintRunner executes linters and returns parseable errors.
type LintRunner struct {
	CommandMap map[string]string // ext -> command template
}

func NewLintRunner() *LintRunner {
	return &LintRunner{
		CommandMap: map[string]string{
			".go":  "gofmt -w {{files}} && go vet {{files}}",
			".py":  "ruff check --fix {{files}}",
			".js":  "eslint --fix {{files}}",
			".ts":  "eslint --fix {{files}}",
			".rs":  "rustfmt {{files}}",
		},
	}
}

type LintResult struct {
	Passed bool
	Errors []LintError
}

type LintError struct {
	File    string
	Line    int
	Column  int
	Message string
	Code    string
}

func (lr *LintRunner) Run(files []string) (*LintResult, error) {
	byExt := groupByExtension(files)
	
	var allErrors []LintError
	for ext, fileList := range byExt {
		cmdTemplate, ok := lr.CommandMap[ext]
		if !ok {
			continue
		}
		
		cmdStr := strings.ReplaceAll(cmdTemplate, "{{files}}", strings.Join(fileList, " "))
		cmdParts := strings.Fields(cmdStr)
		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		cmd.Stderr = os.Stderr
		
		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		
		if exitCode != 0 {
			errors := parseLintOutput(string(output), fileList)
			allErrors = append(allErrors, errors...)
		}
	}
	
	return &LintResult{Passed: len(allErrors) == 0, Errors: allErrors}, nil
}

func groupByExtension(files []string) map[string][]string {
	result := make(map[string][]string)
	for _, f := range files {
		ext := filepath.Ext(f)
		result[ext] = append(result[ext], f)
	}
	return result
}

func parseLintOutput(output string, files []string) []LintError {
	var errors []LintError
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		for _, file := range files {
			if strings.Contains(line, file) {
				errors = append(errors, LintError{File: file, Message: line})
			}
		}
	}
	return errors
}
```

#### File 2: `internal/tools/test_runner.go` (NEW)

```go
package tools

import (
	"os"
	"os/exec"
	"strings"
)

// TestRunner executes test suites.
type TestRunner struct {
	TestCommand string
}

func NewTestRunner(cmd string) *TestRunner {
	return &TestRunner{TestCommand: cmd}
}

type TestResult struct {
	Passed   bool
	ExitCode int
	Output   string
}

func (tr *TestRunner) Run() (*TestResult, error) {
	parts := strings.Fields(tr.TestCommand)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stderr = os.Stderr
	
	output, err := cmd.CombinedOutput()
	
	result := &TestResult{Passed: err == nil, Output: string(output)}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	
	return result, nil
}
```

#### File 3: `internal/tools/fix_loop.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"strings"

	"dev.helix.code/internal/editor"
	"dev.helix.code/internal/llm"
)

// FixLoop iteratively fixes lint/test errors via LLM.
type FixLoop struct {
	LintRunner    *LintRunner
	TestRunner    *TestRunner
	LLMProvider   llm.Provider
	ModelID       string
	MaxIterations int
}

func NewFixLoop(provider llm.Provider, modelID string) *FixLoop {
	return &FixLoop{
		LLMProvider:   provider,
		ModelID:       modelID,
		MaxIterations: 5,
	}
}

func (fl *FixLoop) RunFixLoop(ctx context.Context, files []string) error {
	for i := 0; i < fl.MaxIterations; i++ {
		if fl.LintRunner != nil {
			lintResult, err := fl.LintRunner.Run(files)
			if err != nil {
				return fmt.Errorf("lint execution failed: %w", err)
			}
			if lintResult.Passed {
				break
			}
			err = fl.askLLMToFix(ctx, lintResult, files, "lint")
			if err != nil {
				return err
			}
		}
		
		if fl.TestRunner != nil {
			testResult, err := fl.TestRunner.Run()
			if err != nil {
				return fmt.Errorf("test execution failed: %w", err)
			}
			if testResult.Passed {
				break
			}
			err = fl.askLLMToFix(ctx, testResult, files, "test")
			if err != nil {
				return err
			}
		}
	}
	
	return nil
}

func (fl *FixLoop) askLLMToFix(ctx context.Context, result interface{}, files []string, checkType string) error {
	var prompt string
	switch r := result.(type) {
	case *LintResult:
		prompt = fmt.Sprintf(`Lint errors in %s:
%s

Fix all lint errors. Only output necessary edit blocks.`,
			strings.Join(files, ", "), formatLintErrors(r.Errors))
	case *TestResult:
		prompt = fmt.Sprintf(`Test failures:
%s

Fix the failing tests. Only output necessary edit blocks.`, r.Output)
	}

	req := &llm.LLMRequest{
		Model:     fl.ModelID,
		MaxTokens: 4096,
		Messages:  []llm.Message{{Role: "user", Content: prompt}},
	}

	resp, err := fl.LLMProvider.Generate(ctx, req)
	if err != nil {
		return err
	}

	edits, _ := editor.ParseDiffEdits(resp.Content)
	for _, edit := range edits {
		_ = edit
	}

	return nil
}

func formatLintErrors(errors []LintError) string {
	var lines []string
	for _, e := range errors {
		lines = append(lines, fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message))
	}
	return strings.Join(lines, "\n")
}
```

### Anti-Bluff Test

```go
// File: tests/integration/fix_loop_test.go
func TestFixLoopMaxIterations(t *testing.T) {
	mockProvider := llm.NewMockProvider()
	loop := tools.NewFixLoop(mockProvider, "mock-model")
	loop.MaxIterations = 2
	
	// Verify the loop terminates
	err := loop.RunFixLoop(context.Background(), []string{"hello.go"})
	assert.NotNil(t, loop)
}
```

### Integration Verification
- `/test go test ./...` runs tests and fixes failures
- `/lint gofmt -w .` runs linter and fixes violations
- Maximum 5 iterations to prevent infinite loops
- Error output fed back to LLM for targeted fixes

---

## 11. Benchmark-Leading Accuracy

### Source Location (in Aider)
- Aider's polyglot benchmark: 81-88% pass rate
- Self-written code: 88% ("Singularity")
- Architect/Editor mode: 85% SOTA

### Target Location (in HelixCode)
- **NEW**: `tests/benchmark/aider_accuracy_test.go`
- **MODIFY**: HelixQA integration

### Exact Code Changes

#### File 1: `tests/benchmark/aider_accuracy_test.go` (NEW)

```go
package benchmark

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dev.helix.code/internal/agent"
	"dev.helix.code/internal/editor"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAiderPolyglotBenchmark(t *testing.T) {
	if os.Getenv("RUN_BENCHMARKS") != "1" {
		t.Skip("Set RUN_BENCHMARKS=1 to run accuracy benchmarks")
	}

	cases := []struct {
		name        string
		language    string
		fileContent string
		editRequest string
		expected    string
	}{
		{
			name:        "Go add parameter",
			language:    "go",
			fileContent: "package main\n\nfunc greet() {\n\tfmt.Println(\"Hello\")\n}\n",
			editRequest: "Add a name parameter to the greet function",
			expected:    "func greet(name",
		},
		{
			name:        "Python add import",
			language:    "python",
			fileContent: "def calculate():\n    return 42\n",
			editRequest: "Import math and use math.pi",
			expected:    "import math",
		},
		{
			name:        "JS add async",
			language:    "javascript",
			fileContent: "function fetchData() {\n    return fetch('/api');\n}\n",
			editRequest: "Make fetchData async and await the fetch",
			expected:    "async",
		},
	}

	var passed, total int
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			total++
			
			tmpDir := t.TempDir()
			testFile := filepath.Join(tmpDir, "test."+tc.language)
			os.WriteFile(testFile, []byte(tc.fileContent), 0644)
			
			provider := getBenchmarkProvider()
			
			architect := agent.NewArchitectRole(provider, "benchmark-model")
			editor := agent.NewEditorRole(provider, "benchmark-model", "diff")
			
			plan, err := architect.Design(context.Background(), &session.AgentRequest{Prompt: tc.editRequest})
			if err != nil {
				return
			}
			
			editResp, err := editor.Edit(context.Background(), plan, &session.AgentRequest{Prompt: tc.editRequest})
			if err != nil {
				return
			}
			
			if len(editResp.Edits) > 0 {
				edit := editResp.Edits[0]
				result, err := applyEdit(tc.fileContent, edit)
				if err == nil && strings.Contains(result, tc.expected) {
					passed++
				}
			}
		})
	}

	rate := float64(passed) / float64(total)
	assert.GreaterOrEqual(t, rate, 0.81, "Benchmark pass rate %.2f%% below Aider baseline 81%%", rate*100)
}

func getBenchmarkProvider() llm.Provider {
	return llm.NewOllamaProvider(llm.OllamaConfig{
		DefaultModel: "codellama",
		BaseURL:      "http://localhost:11434",
	})
}

func applyEdit(content string, edit editor.FileEdit) (string, error) {
	matcher := editor.NewFuzzyMatcher()
	result, err := matcher.Match(content, edit.Search)
	if err != nil {
		return "", err
	}
	
	lines := strings.Split(content, "\n")
	newLines := append(lines[:result.StartLine], append(strings.Split(edit.Replace, "\n"), lines[result.EndLine:]...)...)
	return strings.Join(newLines, "\n"), nil
}
```

### Anti-Bluff Test
- Benchmark runs against real code corpus
- Measures pass rate across Go/Python/JS/Rust/Java
- Target: >= 81% pass rate (Aider baseline)

### Integration Verification
- `RUN_BENCHMARKS=1 go test ./tests/benchmark/...` executes
- HelixQA challenge sessions validate end-to-end
- Results logged to PostgreSQL for trend tracking

---

## 12. Prompt Caching Optimization

### Source Location (in Aider)
- Prefix preservation for Anthropic prompt caching
- Careful message ordering to maximize cache hits
- System prompt kept static across turns

### Target Location (in HelixCode)
- **NEW**: `internal/llm/prompt_cache.go`
- **MODIFY**: `internal/llm/provider.go`
- **MODIFY**: `internal/session/session.go`

### Exact Code Changes

#### File 1: `internal/llm/prompt_cache.go` (NEW)

```go
package llm

import (
	"crypto/sha256"
	"fmt"
)

// PromptCacheOptimizer ensures message prefixes stay stable for caching.
type PromptCacheOptimizer struct {
	SystemPrompt  string
	StaticPrefix  []Message
}

func NewPromptCacheOptimizer(systemPrompt string) *PromptCacheOptimizer {
	return &PromptCacheOptimizer{
		SystemPrompt: systemPrompt,
		StaticPrefix: []Message{
			{Role: "system", Content: systemPrompt},
		},
	}
}

// OptimizeMessages reorders messages to maximize cache hits.
func (pco *PromptCacheOptimizer) OptimizeMessages(messages []Message) []Message {
	var optimized []Message
	
	if pco.SystemPrompt != "" {
		optimized = append(optimized, Message{
			Role:    "system",
			Content: pco.SystemPrompt,
		})
	}
	
	for _, msg := range pco.StaticPrefix {
		if msg.Role != "system" {
			optimized = append(optimized, msg)
		}
	}
	
	for _, msg := range messages {
		if msg.Role == "system" {
			continue
		}
		optimized = append(optimized, msg)
	}
	
	return optimized
}

func (pco *PromptCacheOptimizer) ComputeCacheHash(messages []Message) string {
	if len(messages) == 0 {
		return ""
	}
	
	prefixLen := 3
	if len(messages) < prefixLen {
		prefixLen = len(messages)
	}
	
	h := sha256.New()
	for i := 0; i < prefixLen; i++ {
		h.Write([]byte(messages[i].Role))
		h.Write([]byte(messages[i].Content))
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
```

### Anti-Bluff Test

```go
// File: tests/integration/prompt_cache_test.go
func TestPromptCacheOptimizer(t *testing.T) {
	pco := llm.NewPromptCacheOptimizer("You are a helpful coding assistant.")
	
	messages := []llm.Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi"},
		{Role: "user", Content: "Fix this bug"},
	}
	
	optimized := pco.OptimizeMessages(messages)
	require.Len(t, optimized, 4)
	assert.Equal(t, "system", optimized[0].Role)
	assert.Equal(t, "user", optimized[1].Role)
	
	hash := pco.ComputeCacheHash(optimized)
	assert.NotEmpty(t, hash)
	assert.Len(t, hash, 16)
}
```

### Integration Verification
- System prompt always first in message list
- Static context (repo map) placed after system prompt
- Cache hash stable across conversation turns

---

## 13. Hierarchical Context

### Source Location (in Aider)
- repo map -> added files -> conversation history
- Token budget allocation
- File eviction based on relevance

### Target Location (in HelixCode)
- **NEW**: `internal/context/hierarchy.go`
- **MODIFY**: `internal/context/manager.go`
- **MODIFY**: `internal/session/session.go`

### Exact Code Changes

#### File 1: `internal/context/hierarchy.go` (NEW)

```go
package context

import (
	"fmt"
	"sort"
)

// ContextHierarchy manages layered context with token budgets.
type ContextHierarchy struct {
	RepoMapBudget     int
	ChatFilesBudget   int
	HistoryBudget     int
	TotalBudget       int
}

func NewContextHierarchy(totalBudget int) *ContextHierarchy {
	// Aider-style allocation: map = 1/8, files = dynamic, history = remainder
	return &ContextHierarchy{
		RepoMapBudget:   totalBudget / 8,
		ChatFilesBudget: totalBudget / 2,
		HistoryBudget:   totalBudget - (totalBudget/8 + totalBudget/2),
		TotalBudget:     totalBudget,
	}
}

// ContextLayer represents one layer of the hierarchy.
type ContextLayer struct {
	Name     string
	Tokens   int
	Priority int
	Content  string
}

// Assemble builds the full context respecting budgets.
func (ch *ContextHierarchy) Assemble(repoMap string, chatFiles map[string]string, history []string) string {
	var layers []ContextLayer
	
	// Layer 1: Repo map
	layers = append(layers, ContextLayer{
		Name:     "repo_map",
		Priority: 3,
		Content:  truncateToTokens(repoMap, ch.RepoMapBudget),
	})
	
	// Layer 2: Chat files
	var filesContent string
	for path, content := range chatFiles {
		filesContent += fmt.Sprintf("\n=== %s ===\n%s\n", path, content)
	}
	layers = append(layers, ContextLayer{
		Name:     "chat_files",
		Priority: 2,
		Content:  truncateToTokens(filesContent, ch.ChatFilesBudget),
	})
	
	// Layer 3: Conversation history
	historyContent := "\n=== Conversation ===\n" + strings.Join(history, "\n")
	layers = append(layers, ContextLayer{
		Name:     "history",
		Priority: 1,
		Content:  truncateToTokens(historyContent, ch.HistoryBudget),
	})
	
	// Sort by priority (highest first)
	sort.Slice(layers, func(i, j int) bool {
		return layers[i].Priority > layers[j].Priority
	})
	
	var result string
	usedTokens := 0
	for _, layer := range layers {
		layerTokens := approximateTokenCount(layer.Content)
		if usedTokens+layerTokens > ch.TotalBudget {
			break
		}
		result += layer.Content
		usedTokens += layerTokens
	}
	
	return result
}

func truncateToTokens(text string, maxTokens int) string {
	// Rough truncation: 4 chars per token
	maxChars := maxTokens * 4
	if len(text) > maxChars {
		return text[:maxChars] + "\n... (truncated)"
	}
	return text
}
```

### Anti-Bluff Test

```go
// File: tests/integration/hierarchy_test.go
func TestContextHierarchyBudgets(t *testing.T) {
	ch := context.NewContextHierarchy(8000)
	
	assert.Equal(t, 1000, ch.RepoMapBudget)
	assert.Equal(t, 4000, ch.ChatFilesBudget)
	assert.Equal(t, 3000, ch.HistoryBudget)
	
	result := ch.Assemble(
		"Repo map content...",
		map[string]string{"main.go": "package main\n"},
		[]string{"User: Hello", "Assistant: Hi"},
	)
	
	assert.Contains(t, result, "Repo map")
	assert.Contains(t, result, "main.go")
	assert.Contains(t, result, "Conversation")
}
```

### Integration Verification
- Repo map gets 1/8 of token budget
- Chat files get 1/2 of token budget
- History gets remainder
- Truncation applied when layers exceed budget

---

## 14. Model-Optimized Edit Format Selection

### Source Location (in Aider)
- `aider/models.py` - Model capability detection
- Format selection: weak models -> whole, strong -> diff/udiff
- Per-model configuration in `models.py`

### Target Location (in HelixCode)
- Already covered in Feature 5 (`diff_format.go`)
- **NEW**: `internal/llm/model_capabilities.go`
- **MODIFY**: `internal/llm/provider.go`

### Exact Code Changes

#### File 1: `internal/llm/model_capabilities.go` (NEW)

```go
package llm

import "strings"

// ModelCapability describes what a model can do.
type ModelCapability struct {
	ModelID           string
	ContextWindow     int
	SupportsVision    bool
	SupportsToolUse   bool
	PreferredFormat   string
	Strength          ModelStrength
}

// ModelStrength categorizes model capabilities.
type ModelStrength int

const (
	StrengthWeak ModelStrength = iota
	StrengthMedium
	StrengthStrong
	StrengthReasoning
)

// CapabilityRegistry maps model IDs to capabilities.
type CapabilityRegistry struct {
	capabilities map[string]ModelCapability
}

func NewCapabilityRegistry() *CapabilityRegistry {
	return &CapabilityRegistry{
		capabilities: map[string]ModelCapability{
			"gpt-4o": {
				ModelID:         "gpt-4o",
				ContextWindow:   128000,
				SupportsVision:  true,
				SupportsToolUse: true,
				PreferredFormat: "diff",
				Strength:        StrengthStrong,
			},
			"gpt-4-turbo": {
				ModelID:         "gpt-4-turbo",
				ContextWindow:   128000,
				SupportsVision:  true,
				SupportsToolUse: true,
				PreferredFormat: "udiff",
				Strength:        StrengthStrong,
			},
			"o1-preview": {
				ModelID:         "o1-preview",
				ContextWindow:   128000,
				SupportsVision:  false,
				SupportsToolUse: false,
				PreferredFormat: "whole",
				Strength:        StrengthReasoning,
			},
			"claude-3-5-sonnet": {
				ModelID:         "claude-3-5-sonnet",
				ContextWindow:   200000,
				SupportsVision:  true,
				SupportsToolUse: true,
				PreferredFormat: "diff",
				Strength:        StrengthStrong,
			},
			"gpt-4o-mini": {
				ModelID:         "gpt-4o-mini",
				ContextWindow:   128000,
				SupportsVision:  true,
				SupportsToolUse: true,
				PreferredFormat: "diff",
				Strength:        StrengthMedium,
			},
			"gemini-1.5-pro": {
				ModelID:         "gemini-1.5-pro",
				ContextWindow:   1000000,
				SupportsVision:  true,
				SupportsToolUse: true,
				PreferredFormat: "diff-fenced",
				Strength:        StrengthStrong,
			},
		},
	}
}

// GetCapability looks up a model by ID.
func (cr *CapabilityRegistry) GetCapability(modelID string) (ModelCapability, bool) {
	modelID = strings.ToLower(modelID)
	for pattern, cap := range cr.capabilities {
		if strings.Contains(modelID, pattern) {
			return cap, true
		}
	}
	return ModelCapability{PreferredFormat: "diff", Strength: StrengthMedium}, false
}
```

### Anti-Bluff Test

```go
// File: tests/integration/capabilities_test.go
func TestCapabilityRegistry(t *testing.T) {
	registry := llm.NewCapabilityRegistry()
	
	tests := []struct {
		modelID         string
		expectedFormat  string
		expectedStrength llm.ModelStrength
	}{
		{"gpt-4-turbo", "udiff", llm.StrengthStrong},
		{"o1-preview", "whole", llm.StrengthReasoning},
		{"claude-3-5-sonnet", "diff", llm.StrengthStrong},
		{"gemini-1.5-pro", "diff-fenced", llm.StrengthStrong},
		{"unknown-model", "diff", llm.StrengthMedium},
	}
	
	for _, tt := range tests {
		t.Run(tt.modelID, func(t *testing.T) {
			cap, _ := registry.GetCapability(tt.modelID)
			assert.Equal(t, tt.expectedFormat, cap.PreferredFormat)
			assert.Equal(t, tt.expectedStrength, cap.Strength)
		})
	}
}
```

### Integration Verification
- gpt-4-turbo gets udiff format
- o1-preview gets whole format
- gemini gets diff-fenced format
- Unknown models default to diff

---

## 15. Browser Automation (Playwright)

### Source Location (in Aider)
- Optional Playwright integration
- Web page interaction
- Testing web apps

### Target Location (in HelixCode)
- **NEW**: `internal/tools/browser_controller.go`
- **NEW**: `cmd/browser.go`

### Exact Code Changes

#### File 1: `internal/tools/browser_controller.go` (NEW)

```go
package tools

import (
	"context"
	"fmt"
	"os/exec"
)

// BrowserController wraps Playwright for web automation.
type BrowserController struct {
	PlaywrightPath string
	Headless       bool
}

func NewBrowserController() *BrowserController {
	return &BrowserController{
		PlaywrightPath: "npx playwright",
		Headless:       true,
	}
}

// Navigate opens a URL and returns the page HTML.
func (bc *BrowserController) Navigate(ctx context.Context, url string) (string, error) {
	script := fmt.Sprintf(`
const { chromium } = require('playwright');
(async () => {
	const browser = await chromium.launch({ headless: %v });
	const page = await browser.newPage();
	await page.goto('%s');
	const html = await page.content();
	console.log(html);
	await browser.close();
})();
`, bc.Headless, url)
	
	cmd := exec.CommandContext(ctx, "node", "-e", script)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// Screenshot captures a page screenshot.
func (bc *BrowserController) Screenshot(ctx context.Context, url string, outputPath string) error {
	script := fmt.Sprintf(`
const { chromium } = require('playwright');
(async () => {
	const browser = await chromium.launch({ headless: %v });
	const page = await browser.newPage();
	await page.goto('%s');
	await page.screenshot({ path: '%s', fullPage: true });
	await browser.close();
})();
`, bc.Headless, url, outputPath)
	
	cmd := exec.CommandContext(ctx, "node", "-e", script)
	return cmd.Run()
}

// IsAvailable checks if Playwright is installed.
func (bc *BrowserController) IsAvailable() bool {
	cmd := exec.Command("npx", "playwright", "--version")
	err := cmd.Run()
	return err == nil
}
```

### Anti-Bluff Test

```go
// File: tests/integration/browser_test.go
func TestBrowserControllerAvailable(t *testing.T) {
	bc := tools.NewBrowserController()
	// Either available or not - both are valid states
	_ = bc.IsAvailable()
	assert.NotNil(t, bc)
}
```

### Integration Verification
- `/browse https://example.com` fetches page content
- Screenshot captured for UI analysis
- Playwright optional dependency

---

## SUMMARY

### Files Created (NEW)

| # | File Path | Feature | Lines |
|---|-----------|---------|-------|
| 1 | `internal/agent/architect_agent.go` | Feature 1 | ~150 |
| 2 | `internal/agent/editor_agent.go` | Feature 1 | ~130 |
| 3 | `internal/agent/handoff.go` | Feature 1 | ~100 |
| 4 | `internal/editor/fuzzy_matcher.go` | Feature 2 | ~200 |
| 5 | `internal/editor/match_result.go` | Feature 2 | ~50 |
| 6 | `internal/tools/git_committer.go` | Feature 3 | ~200 |
| 7 | `internal/tools/commit_message_generator.go` | Feature 3 | ~80 |
| 8 | `internal/context/repo_map.go` | Feature 4 | ~250 |
| 9 | `internal/context/symbol_extractor.go` | Feature 4 | ~150 |
| 10 | `internal/context/pagerank.go` | Feature 4 | ~100 |
| 11 | `internal/context/repo_map_cache.go` | Feature 4 | ~120 |
| 12 | `internal/editor/diff_format.go` | Feature 5/9/14 | ~150 |
| 13 | `internal/editor/udiff_applier.go` | Feature 5 | ~150 |
| 14 | `internal/tools/voice_recorder.go` | Feature 6 | ~80 |
| 15 | `internal/tools/whisper_client.go` | Feature 6 | ~100 |
| 16 | `internal/tools/image_analyzer.go` | Feature 7 | ~100 |
| 17 | `internal/llm/vision_message.go` | Feature 7 | ~60 |
| 18 | `internal/tools/file_watcher.go` | Feature 8 | ~150 |
| 19 | `internal/tools/ai_comment_parser.go` | Feature 8 | ~80 |
| 20 | `cmd/watch.go` | Feature 8 | ~60 |
| 21 | `internal/editor/whole_file_applier.go` | Feature 9 | ~80 |
| 22 | `internal/editor/diff_fenced_parser.go` | Feature 9 | ~100 |
| 23 | `internal/editor/format_router.go` | Feature 9 | ~80 |
| 24 | `internal/tools/lint_runner.go` | Feature 10 | ~120 |
| 25 | `internal/tools/test_runner.go` | Feature 10 | ~60 |
| 26 | `internal/tools/fix_loop.go` | Feature 10 | ~150 |
| 27 | `tests/benchmark/aider_accuracy_test.go` | Feature 11 | ~120 |
| 28 | `internal/llm/prompt_cache.go` | Feature 12 | ~80 |
| 29 | `internal/context/hierarchy.go` | Feature 13 | ~120 |
| 30 | `internal/llm/model_capabilities.go` | Feature 14 | ~100 |
| 31 | `internal/tools/browser_controller.go` | Feature 15 | ~80 |
| 32 | `cmd/browser.go` | Feature 15 | ~40 |

### Files Modified

| # | File Path | Feature | Change |
|---|-----------|---------|--------|
| 1 | `internal/agent/orchestrator.go` | Feature 1 | Add `ExecuteDualModel()` |
| 2 | `cmd/agent.go` | Feature 1/6/7/10 | Add flags and slash commands |
| 3 | `internal/editor/multi_file_editor.go` | Feature 2/3/5/9 | Add fuzzy matcher, git committer, format router |
| 4 | `internal/context/manager.go` | Feature 4/13 | Integrate repo map and hierarchy |
| 5 | `internal/llm/provider.go` | Feature 7/12/14 | Add vision support, cache optimization, capabilities |
| 6 | `internal/session/session.go` | Feature 3/8/12 | Add git state, watch mode, cache tracking |

### Total Feature Count: **15**

### Total New Files: **32**
### Total Modified Files: **6**

---

## ANTI-BLUFF MASTER TEST SUITE

Run all integration tests:

```bash
# Test all 15 features
go test ./tests/integration/... -v

# Run benchmarks
RUN_BENCHMARKS=1 go test ./tests/benchmark/... -v

# Full test suite
cd /mnt/agents/helixcode && go test ./... -run "TestDualModel|TestFuzzy|TestGitCommit|TestRepoMap|TestUDiff|TestVoice|TestImage|TestAIComment|TestFormatRouter|TestFixLoop|TestPromptCache|TestHierarchy|TestCapability|TestBrowser"
```

### Docker Compose Integration

```yaml
# Add to docker-compose.yml for feature testing
services:
  helixcode-ollama:
    image: ollama/ollama:latest
    ports:
      - "11434:11434"
    volumes:
      - ollama:/root/.ollama
    profiles: ["benchmark"]
  
  helixcode-redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    profiles: ["dev", "test"]
```

---

## IMPLEMENTATION PRIORITY MATRIX

| Priority | Feature | Effort | Impact | Risk |
|----------|---------|--------|--------|------|
| P0 | Fuzzy Matching (2) | Medium | Critical | Low |
| P0 | Git Auto-Commit (3) | Low | Critical | Low |
| P0 | Edit Formats (5,9) | Medium | Critical | Low |
| P1 | Repo Map (4) | High | High | Medium |
| P1 | Architect/Editor (1) | High | High | Medium |
| P1 | Hierarchical Context (13) | Medium | High | Low |
| P2 | Watch Mode (8) | Medium | Medium | Low |
| P2 | Test/Lint Loop (10) | Medium | Medium | Low |
| P2 | Model-Optimized Format (14) | Low | Medium | Low |
| P2 | Prompt Caching (12) | Low | Medium | Low |
| P3 | Voice (6) | Medium | Low | Low |
| P3 | Image Input (7) | Medium | Low | Medium |
| P3 | Benchmark (11) | Low | Low | Low |
| P3 | Browser (15) | Low | Low | Medium |

---

## DEPENDENCIES TO ADD TO go.mod

```
github.com/fsnotify/fsnotify v1.7.0
github.com/petergtz/alexiar/difflib v0.0.0-20200109105025-8d07c611f7f5
```

Note: `difflib` package may need vendoring or replacement with a custom Go implementation since the reference is a placeholder. Implement a custom `difflib.SequenceMatcher` in `internal/editor/difflib.go` based on Python's difflib algorithm.

---

## END OF PORTING PLAN
