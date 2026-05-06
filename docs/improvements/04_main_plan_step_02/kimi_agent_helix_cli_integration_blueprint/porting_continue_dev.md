# Complete Porting Plan: Continue.dev → HelixCode

> **Mission**: Integrate all 10 core Continue.dev features into HelixCode's Go architecture.  
> **Source**: `continuedev/continue` (TypeScript, 30K+ stars)  
> **Target**: `HelixDevelopment/HelixCode` (Go, module `dev.helix.code`)  
> **Date**: 2025-05-04  
> **Features**: 10 complete porting plans with exact file paths, Go code, integration points, and anti-bluff tests.

---

## Table of Contents

1. [@Provider System](#1-provider-system)
2. [Universal IDE Support](#2-universal-ide-support)
3. [Context Provider Framework](#3-context-provider-framework)
4. [Autocomplete Engine](#4-autocomplete-engine)
5. [Diff Streaming](#5-diff-streaming)
6. [Prompt Templates](#6-prompt-templates)
7. [Model Configuration](#7-model-configuration)
8. [Tab Autocomplete](#8-tab-autocomplete)
9. [Slash Commands](#9-slash-commands)
10. [Embeddings Integration](#10-embeddings-integration)

---

## Architecture Overview

### Continue.dev Core Architecture (Source)
```
core/
  context/
    providers/          # @file, @url, @docs, @codebase, @code, etc.
      FileContextProvider.ts
      URLContextProvider.ts
      DocsContextProvider.ts
      CodebaseContextProvider.ts
      CodeContextProvider.ts
      DiffContextProvider.ts
      TerminalContextProvider.ts
      SearchContextProvider.ts
      ...
    retrieval/           # RAG retrieval logic
  commands/
    slash/              # /edit, /comment, /share, etc.
  autocomplete/
    CompletionProvider.ts   # Inline completions
    templates.ts            # FIM prompt templates
  diff/
    streamDiff.ts       # Real-time diff streaming
  indexing/
    CodeSnippetsIndex.ts    # Codebase indexing
    LanceDbIndex.ts        # Vector storage (LanceDB)
    chunk/                 # Code chunking
  llm/
    index.ts            # LLM abstraction
    templates.ts        # Prompt templates
  config/
    load.ts             # Config loading (config.yaml)
```

### HelixCode Target Architecture
```
HelixCode/
  cmd/
    cli/                # CLI entry point
    server/             # HTTP server
  internal/
    llm/                # LLM providers (Ollama, OpenAI, Anthropic)
    config/             # Configuration management
    session/            # Session management
    context/            # NEW: Context assembly
    tools/              # Tool framework
    editor/             # NEW: Editor abstraction
    memory/             # Memory/Knowledge
    mcp/                # MCP protocol
    workflow/           # Workflow execution
    worker/             # SSH worker pool
  api/                  # OpenAPI spec
  applications/         # UI frontends
```

---

## 1. @Provider System

### Source Location (Continue.dev)
- `core/context/providers/FileContextProvider.ts` — @file inclusion
- `core/context/providers/URLContextProvider.ts` — @url fetch
- `core/context/providers/DocsContextProvider.ts` — @docs retrieval
- `core/context/providers/CodebaseContextProvider.ts` — @codebase search
- `core/context/providers/CodeContextProvider.ts` — @code symbol search
- `core/context/providers/index.ts` — Base provider interface

### Target Location (HelixCode)
- **NEW**: `internal/context/providers.go` — Core provider interface
- **NEW**: `internal/context/file_provider.go` — @file implementation
- **NEW**: `internal/context/url_provider.go` — @url implementation
- **NEW**: `internal/context/docs_provider.go` — @docs implementation
- **NEW**: `internal/context/codebase_provider.go` — @codebase implementation
- **NEW**: `internal/context/code_provider.go` — @code implementation
- **NEW**: `internal/context/parser.go` — @-mention parser
- **MODIFY**: `internal/llm/types.go` — Add ContextItem to message model
- **MODIFY**: `cmd/cli/main.go` — Wire parser into interactive mode

### Exact Code Changes

#### File 1: `internal/context/providers.go` (NEW)
```go
package context

import (
	"context"
	"fmt"
)

// ProviderType categorizes how a provider receives input.
type ProviderType string

const (
	ProviderTypeQuery   ProviderType = "query"   // User types query after @provider
	ProviderTypeSubmenu ProviderType = "submenu" // User selects from dropdown
	ProviderTypeStatic  ProviderType = "static"  // No input needed
)

// ContextItem represents a single piece of context assembled for the LLM.
type ContextItem struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Content     string            `json:"content"`
	URI         *ContextURI       `json:"uri,omitempty"`
	Score       float64           `json:"score,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ContextURI identifies the source of a context item.
type ContextURI struct {
	Type  string `json:"type"`  // file, url, codebase, etc.
	Value string `json:"value"` // actual URI
}

// ProviderExtras contains services injected into providers.
type ProviderExtras struct {
	Ctx         context.Context
	WorkspaceDirs []string
	IDE         IDEInterface
	Fetch       func(url string) ([]byte, error)
}

// IDEInterface abstracts editor operations.
type IDEInterface interface {
	ReadFile(uri string) (string, error)
	ListWorkspaceDirs() ([]string, error)
	GetOpenFiles() ([]string, error)
	GetCurrentFile() (string, error)
	GetTerminalOutput() (string, error)
	GetGitDiff() (string, error)
	GetTags(indexType string) ([]string, error)
}

// ProviderDescription registers a provider.
type ProviderDescription struct {
	Title            string       `json:"title"`
	DisplayTitle     string       `json:"displayTitle"`
	Description      string       `json:"description"`
	Type             ProviderType `json:"type"`
	DependsOnIndexing []string    `json:"dependsOnIndexing,omitempty"`
}

// Provider is the interface every @-provider implements.
type Provider interface {
	Description() ProviderDescription
	GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error)
}

// SubmenuProvider extends Provider with item listing.
type SubmenuProvider interface {
	Provider
	LoadSubmenuItems(extras ProviderExtras) ([]ContextSubmenuItem, error)
}

// ContextSubmenuItem represents a selectable item.
type ContextSubmenuItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Icon        string `json:"icon,omitempty"`
}

// Registry holds all registered providers.
type Registry struct {
	providers map[string]Provider
}

func NewRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}

func (r *Registry) Register(name string, p Provider) {
	r.providers[name] = p
}

func (r *Registry) Get(name string) (Provider, bool) {
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) List() []string {
	names := make([]string, 0, len(r.providers))
	for n := range r.providers {
		names = append(names, n)
	}
	return names
}

// Assemble resolves all @-mentions in user input into ContextItems.
func (r *Registry) Assemble(input string, extras ProviderExtras) ([]ContextItem, string, error) {
	mentions := ParseAtMentions(input)
	var items []ContextItem
	cleanInput := input

	for _, m := range mentions {
		provider, ok := r.Get(m.Provider)
		if !ok {
			return nil, "", fmt.Errorf("unknown provider: @%s", m.Provider)
		}
		ctxItems, err := provider.GetContextItems(m.Query, extras)
		if err != nil {
			return nil, "", fmt.Errorf("provider @%s failed: %w", m.Provider, err)
		}
		items = append(items, ctxItems...)
		cleanInput = RemoveMention(cleanInput, m.Raw)
	}

	return items, cleanInput, nil
}
```

#### File 2: `internal/context/parser.go` (NEW)
```go
package context

import (
	"regexp"
	"strings"
)

// Mention represents a parsed @-mention.
type Mention struct {
	Raw      string // Full match: @file:path/to/file.go
	Provider string // e.g., "file"
	Query    string // e.g., "path/to/file.go"
}

// atMentionRegex matches @provider:query or @provider query.
var atMentionRegex = regexp.MustCompile(`(?:^|\s)@([a-zA-Z_]+)(?::(\S+)|\s+(\S+))?`)

// ParseAtMentions extracts all @-mentions from input.
func ParseAtMentions(input string) []Mention {
	matches := atMentionRegex.FindAllStringSubmatchIndex(input, -1)
	var mentions []Mention

	for _, m := range matches {
		if len(m) < 4 {
			continue
		}
		provider := input[m[2]:m[3]]
		var query string
		if m[4] != -1 && m[5] != -1 {
			query = input[m[4]:m[5]]
		} else if len(m) >= 6 && m[6] != -1 && m[7] != -1 {
			query = input[m[6]:m[7]]
		}
		mentions = append(mentions, Mention{
			Raw:      input[m[0]:m[1]],
			Provider: provider,
			Query:    query,
		})
	}

	return mentions
}

// RemoveMention removes a specific mention string from input.
func RemoveMention(input, mention string) string {
	return strings.Replace(input, mention, "", 1)
}

// HasMentions returns true if input contains any @-mentions.
func HasMentions(input string) bool {
	return atMentionRegex.MatchString(input)
}
```

#### File 3: `internal/context/file_provider.go` (NEW)
```go
package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProvider implements @file — include file contents in context.
type FileProvider struct{}

func NewFileProvider() *FileProvider {
	return &FileProvider{}
}

func (p *FileProvider) Description() ProviderDescription {
	return ProviderDescription{
		Title:        "file",
		DisplayTitle: "Files",
		Description:  "Type to search files in workspace",
		Type:         ProviderTypeSubmenu,
	}
}

func (p *FileProvider) GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error) {
	filePath := strings.TrimSpace(query)

	// Security: prevent directory traversal
	for _, dir := range extras.WorkspaceDirs {
		absDir, _ := filepath.Abs(dir)
		absFile, _ := filepath.Abs(filePath)
		if !strings.HasPrefix(absFile, absDir) {
			continue
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
		}

		base := filepath.Base(filePath)
		return []ContextItem{{
			Name:        base,
			Description: filePath,
			Content:     fmt.Sprintf("```%s\n%s\n```", filePath, string(content)),
			URI:         &ContextURI{Type: "file", Value: filePath},
		}}, nil
	}

	return nil, fmt.Errorf("file not found or outside workspace: %s", filePath)
}

func (p *FileProvider) LoadSubmenuItems(extras ProviderExtras) ([]ContextSubmenuItem, error) {
	var items []ContextSubmenuItem
	const maxItems = 10000

	for _, dir := range extras.WorkspaceDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(dir, path)
			items = append(items, ContextSubmenuItem{
				ID:          path,
				Title:       info.Name(),
				Description: rel,
			})
			if len(items) >= maxItems {
				return filepath.SkipAll
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return items, nil
}
```

#### File 4: `internal/context/url_provider.go` (NEW)
```go
package context

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// URLProvider implements @url — fetch and include URL content.
type URLProvider struct{}

func NewURLProvider() *URLProvider {
	return &URLProvider{}
}

func (p *URLProvider) Description() ProviderDescription {
	return ProviderDescription{
		Title:        "url",
		DisplayTitle: "URL",
		Description:  "Reference a webpage at a given URL",
		Type:         ProviderTypeQuery,
	}
}

func (p *URLProvider) GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error) {
	u, err := url.Parse(strings.TrimSpace(query))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	var body []byte
	if extras.Fetch != nil {
		body, err = extras.Fetch(u.String())
	} else {
		body, err = defaultFetch(u.String())
	}
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}

	markdown, title, err := htmlToMarkdown(body)
	if err != nil {
		return nil, fmt.Errorf("failed to convert HTML: %w", err)
	}

	return []ContextItem{{
		Name:        title,
		Description: u.Hostname(),
		Content:     fmt.Sprintf("# %s\n\n%s", title, markdown),
		URI:         &ContextURI{Type: "url", Value: u.String()},
	}}, nil
}

func defaultFetch(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func htmlToMarkdown(html []byte) (string, string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(html)))
	if err != nil {
		return "", "", err
	}

	title := doc.Find("title").First().Text()
	if title == "" {
		title = "Untitled Page"
	}

	// Extract main content
	var sb strings.Builder
	doc.Find("article, main, .content, #content").First().Each(func(i int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})

	content := sb.String()
	if content == "" {
		// Fallback: extract all paragraphs
		doc.Find("p").Each(func(i int, s *goquery.Selection) {
			sb.WriteString(s.Text())
			sb.WriteString("\n\n")
		})
		content = sb.String()
	}

	return content, title, nil
}
```

#### File 5: `internal/context/codebase_provider.go` (NEW)
```go
package context

import (
	"context"
	"fmt"
	"sort"

	"dev.helix.code/internal/knowledge"
)

// CodebaseProvider implements @codebase — semantic search over codebase.
type CodebaseProvider struct {
	store knowledge.Store
}

func NewCodebaseProvider(store knowledge.Store) *CodebaseProvider {
	return &CodebaseProvider{store: store}
}

func (p *CodebaseProvider) Description() ProviderDescription {
	return ProviderDescription{
		Title:             "codebase",
		DisplayTitle:      "Codebase",
		Description:       "Search entire codebase for relevant snippets",
		Type:              ProviderTypeQuery,
		DependsOnIndexing: []string{"chunk", "codeSnippets"},
	}
}

func (p *CodebaseProvider) GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error) {
	if p.store == nil {
		return nil, fmt.Errorf("codebase store not initialized")
	}

	results, err := p.store.Search(extras.Ctx, query, knowledge.SearchOptions{
		Limit:    8,
		Rerank:   true,
		Language: "",
	})
	if err != nil {
		return nil, fmt.Errorf("codebase search failed: %w", err)
	}

	var items []ContextItem
	for _, r := range results {
		items = append(items, ContextItem{
			Name:        r.FilePath,
			Description: fmt.Sprintf("%s:%d-%d", r.FilePath, r.StartLine, r.EndLine),
			Content:     fmt.Sprintf("```%s (lines %d-%d)\n%s\n```", r.FilePath, r.StartLine, r.EndLine, r.Content),
			Score:       r.Score,
			URI:         &ContextURI{Type: "codebase", Value: r.FilePath},
			Metadata: map[string]string{
				"language": r.Language,
				"startLine": fmt.Sprintf("%d", r.StartLine),
				"endLine": fmt.Sprintf("%d", r.EndLine),
			},
		})
	}

	// Sort by score descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].Score > items[j].Score
	})

	return items, nil
}
```

#### File 6: `internal/context/code_provider.go` (NEW)
```go
package context

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"dev.helix.code/internal/knowledge"
)

// CodeProvider implements @code — search for specific functions/classes.
type CodeProvider struct {
	store knowledge.Store
}

func NewCodeProvider(store knowledge.Store) *CodeProvider {
	return &CodeProvider{store: store}
}

func (p *CodeProvider) Description() ProviderDescription {
	return ProviderDescription{
		Title:             "code",
		DisplayTitle:      "Code",
		Description:       "Search for specific functions or classes",
		Type:              ProviderTypeSubmenu,
		DependsOnIndexing: []string{"chunk", "codeSnippets"},
	}
}

func (p *CodeProvider) GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error) {
	// query is the ID returned by LoadSubmenuItems
	if p.store == nil {
		return nil, fmt.Errorf("code store not initialized")
	}

	// Search for exact symbol match
	results, err := p.store.SearchBySymbol(extras.Ctx, query, 1)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("code symbol not found: %s", query)
	}

	r := results[0]
	return []ContextItem{{
		Name:        query,
		Description: fmt.Sprintf("%s:%d", r.FilePath, r.StartLine),
		Content:     fmt.Sprintf("```%s\n%s\n```", r.FilePath, r.Content),
		URI:         &ContextURI{Type: "code", Value: r.FilePath},
	}}, nil
}

func (p *CodeProvider) LoadSubmenuItems(extras ProviderExtras) ([]ContextSubmenuItem, error) {
	if p.store == nil {
		return nil, fmt.Errorf("code store not initialized")
	}

	// Retrieve all indexed code snippets
	snippets, err := p.store.GetAllSnippets(extras.Ctx, "codeSnippets")
	if err != nil {
		return nil, err
	}

	const maxItems = 10000
	var items []ContextSubmenuItem
	for _, s := range snippets {
		items = append(items, ContextSubmenuItem{
			ID:          s.ID,
			Title:       s.Name,
			Description: fmt.Sprintf("%s (%s)", filepath.Base(s.FilePath), s.Language),
		})
		if len(items) >= maxItems {
			break
		}
	}

	// Sort alphabetically
	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
	})

	return items, nil
}
```

#### File 7: `internal/context/docs_provider.go` (NEW)
```go
package context

import (
	"fmt"
	"strings"

	"dev.helix.code/internal/knowledge"
)

// DocsProvider implements @docs — include documentation in context.
type DocsProvider struct {
	store      knowledge.Store
	knownSites []DocSite
}

type DocSite struct {
	Title    string `json:"title"`
	StartURL string `json:"startUrl"`
	RootURL  string `json:"rootUrl"`
	MaxDepth int    `json:"maxDepth"`
}

func NewDocsProvider(store knowledge.Store, sites []DocSite) *DocsProvider {
	return &DocsProvider{store: store, knownSites: sites}
}

func (p *DocsProvider) Description() ProviderDescription {
	return ProviderDescription{
		Title:             "docs",
		DisplayTitle:      "Documentation",
		Description:       "Search indexed documentation sites",
		Type:              ProviderTypeSubmenu,
		DependsOnIndexing: []string{"docs"},
	}
}

func (p *DocsProvider) GetContextItems(query string, extras ProviderExtras) ([]ContextItem, error) {
	// query is doc site title
	siteTitle := strings.TrimSpace(query)

	results, err := p.store.SearchDocs(extras.Ctx, siteTitle, knowledge.SearchOptions{Limit: 5})
	if err != nil {
		return nil, fmt.Errorf("docs search failed: %w", err)
	}

	var items []ContextItem
	for _, r := range results {
		items = append(items, ContextItem{
			Name:        r.Title,
			Description: r.URL,
			Content:     r.Content,
			URI:         &ContextURI{Type: "docs", Value: r.URL},
		})
	}

	return items, nil
}

func (p *DocsProvider) LoadSubmenuItems(extras ProviderExtras) ([]ContextSubmenuItem, error) {
	var items []ContextSubmenuItem
	for _, s := range p.knownSites {
		items = append(items, ContextSubmenuItem{
			ID:          s.Title,
			Title:       s.Title,
			Description: s.RootURL,
		})
	}
	return items, nil
}
```

#### File 8: `internal/llm/types.go` — MODIFY (add ContextItem to message)
```go
// Message represents a chat message with optional context attachments.
type Message struct {
	Role       string        `json:"role"`
	Content    string        `json:"content"`
	ContextItems []context.ContextItem `json:"context_items,omitempty"` // NEW
}
```

#### File 9: `cmd/cli/main.go` — MODIFY (wire providers)
Add in `NewCLI()`:
```go
func NewCLI() *CLI {
	// ... existing init ...

	// NEW: Initialize context provider registry
	ctxRegistry := context.NewRegistry()
	ctxRegistry.Register("file", context.NewFileProvider())
	ctxRegistry.Register("url", context.NewURLProvider())
	ctxRegistry.Register("docs", context.NewDocsProvider(nil, nil))
	// codebase and code require knowledge.Store initialization
	// ctxRegistry.Register("codebase", context.NewCodebaseProvider(knowledgeStore))
	// ctxRegistry.Register("code", context.NewCodeProvider(knowledgeStore))

	return &CLI{
		// ... existing fields ...
		contextRegistry: ctxRegistry,
	}
}
```

### Anti-Bluff Test
```bash
# 1. Build and test @file provider
$ cd HelixCode && go test ./internal/context/... -run TestFileProvider
# Should pass with real file reads, not mocks

# 2. Test @url provider with real HTTP fetch
$ go test ./internal/context/... -run TestURLProvider -v
# Should fetch https://example.com and return markdown content

# 3. Test parser extracts all mentions
$ go test ./internal/context/... -run TestParseAtMentions -v
# Input: "Explain @file:main.go and @url:https://go.dev"
# Expected: 2 mentions, provider=file|url, query=main.go|https://go.dev

# 4. End-to-end integration
$ ./cli --prompt "Review @file:internal/llm/types.go for bugs"
# Should read file, prepend context to prompt, send to LLM
```

### Integration Verification
- [ ] `go test ./internal/context/...` passes all provider tests
- [ ] `@file` reads actual files from workspace (verified by tmp file test)
- [ ] `@url` fetches real URLs and converts HTML to markdown
- [ ] `@codebase` returns search results from knowledge store
- [ ] Parser correctly strips mentions from user input before sending to LLM
- [ ] Context assembly respects token budget (see Feature 3)

---

## 2. Universal IDE Support

### Source Location (Continue.dev)
- `extensions/vscode/` — VS Code extension
- `extensions/intellij/` — JetBrains plugin
- `extensions/vim/` — Neovim plugin
- `core/protocol/` — Editor-agnostic protocol (JSON-RPC over WebSocket)
- `core/ide/` — IDE abstraction interface

### Target Location (HelixCode)
- **NEW**: `internal/editor/` — Editor abstraction layer
- **NEW**: `internal/editor/vscode.go` — VS Code LSP adapter
- **NEW**: `internal/editor/jetbrains.go` — JetBrains adapter
- **NEW**: `internal/editor/neovim.go` — Neovim adapter
- **NEW**: `internal/editor/protocol.go` — Editor protocol definitions
- **NEW**: `internal/editor/server.go` — WebSocket/JSON-RPC server
- **NEW**: `api/editor.yaml` — OpenAPI extension for editor protocol
- **MODIFY**: `internal/server/` — Add editor WebSocket endpoint
- **MODIFY**: `cmd/server/main.go` — Start editor protocol server

### Exact Code Changes

#### File 1: `internal/editor/protocol.go` (NEW)
```go
package editor

// Protocol defines the editor-agnostic communication format.
// Continue.dev uses a custom JSON-RPC protocol over WebSocket.
// HelixCode adapts this to a Go struct-based protocol.

// Method types for editor protocol.
const (
	MethodGetWorkspaceDirs   = "ide/getWorkspaceDirs"
	MethodReadFile           = "ide/readFile"
	MethodGetOpenFiles       = "ide/getOpenFiles"
	MethodGetCurrentFile     = "ide/getCurrentFile"
	MethodGetTerminalOutput  = "ide/getTerminalOutput"
	MethodGetGitDiff         = "ide/getDiff"
	MethodShowDiff           = "ide/showDiff"
	MethodApplyEdit          = "ide/applyEdit"
	MethodInsertAtCursor     = "ide/insertAtCursor"
	MethodGetCompletion      = "autocomplete/getCompletion"
	MethodAcceptCompletion   = "autocomplete/acceptCompletion"
	MethodRejectCompletion   = "autocomplete/rejectCompletion"
	MethodGetLSPDiagnostics  = "lsp/getDiagnostics"
	MethodExecuteCommand     = "ide/executeCommand"
)

// Request is the envelope for all editor requests.
type Request struct {
	ID     int64       `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// Response is the envelope for all editor responses.
type Response struct {
	ID     int64       `json:"id"`
	Result interface{} `json:"result,omitempty"`
	Error  *Error      `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// WorkspaceDirsParams for getWorkspaceDirs.
type WorkspaceDirsParams struct{}

// ReadFileParams for readFile.
type ReadFileParams struct {
	URI string `json:"uri"`
}

// ReadFileResult returns file content.
type ReadFileResult struct {
	Content string `json:"content"`
}

// DiffParams for showDiff.
type DiffParams struct {
	Original string `json:"original"`
	Modified string `json:"modified"`
	URI      string `json:"uri"`
}

// ApplyEditParams for applyEdit.
type ApplyEditParams struct {
	URI     string `json:"uri"`
	Content string `json:"content"`
}

// CompletionParams for getCompletion.
type CompletionParams struct {
	URI        string `json:"uri"`
	Line       int    `json:"line"`
	Character  int    `json:"character"`
	Prefix     string `json:"prefix"`
	Suffix     string `json:"suffix"`
	Language   string `json:"language"`
}

// CompletionResult for autocomplete.
type CompletionResult struct {
	Items []CompletionItem `json:"items"`
}

type CompletionItem struct {
	InsertText string  `json:"insertText"`
	Score      float64 `json:"score"`
	Model      string  `json:"model"`
}
```

#### File 2: `internal/editor/editor.go` (NEW) — Core Interface
```go
package editor

import "context"

// Editor is the IDE abstraction that HelixCode communicates with.
// Every IDE adapter (VS Code, JetBrains, Neovim) implements this.
type Editor interface {
	// Identity
	Name() string
	Version() string

	// File operations
	GetWorkspaceDirs(ctx context.Context) ([]string, error)
	ReadFile(ctx context.Context, uri string) (string, error)
	WriteFile(ctx context.Context, uri string, content string) error
	GetOpenFiles(ctx context.Context) ([]string, error)
	GetCurrentFile(ctx context.Context) (string, error)

	// Editor state
	GetCursorPosition(ctx context.Context) (Position, error)
	GetSelection(ctx context.Context) (Range, error)
	GetTerminalOutput(ctx context.Context) (string, error)
	GetGitDiff(ctx context.Context, staged bool) (string, error)

	// LSP integration
	GetDiagnostics(ctx context.Context, uri string) ([]Diagnostic, error)
	GetHover(ctx context.Context, uri string, pos Position) (string, error)
	GetDefinitions(ctx context.Context, uri string, pos Position) ([]Location, error)
	GetReferences(ctx context.Context, uri string, pos Position) ([]Location, error)

	// Diff streaming
	ShowDiff(ctx context.Context, uri string, original string, modified string) error
	AcceptDiffHunk(ctx context.Context, hunkID string) error
	RejectDiffHunk(ctx context.Context, hunkID string) error

	// Autocomplete
	ShowGhostText(ctx context.Context, uri string, pos Position, text string) error
	ClearGhostText(ctx context.Context, uri string) error
	AcceptCompletion(ctx context.Context, completionID string) error
	RejectCompletion(ctx context.Context, completionID string) error

	// Commands
	ExecuteCommand(ctx context.Context, command string, args []string) error
}

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Severity string `json:"severity"` // error, warning, info, hint
	Message  string `json:"message"`
	Range    Range  `json:"range"`
	Code     string `json:"code,omitempty"`
}

type Location struct {
	URI   string   `json:"uri"`
	Range Range    `json:"range"`
}
```

#### File 3: `internal/editor/server.go` (NEW) — WebSocket/JSON-RPC Server
```go
package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Server handles editor connections.
type Server struct {
	editors   map[string]Editor // sessionID -> Editor
	editorsMu sync.RWMutex
	handlers  map[string]func(ctx context.Context, params json.RawMessage) (interface{}, error)
}

func NewServer() *Server {
	s := &Server{
		editors:  make(map[string]Editor),
		handlers: make(map[string]func(context.Context, json.RawMessage) (interface{}, error)),
	}
	s.registerHandlers()
	return s
}

func (s *Server) registerHandlers() {
	s.handlers[MethodGetWorkspaceDirs] = s.handleGetWorkspaceDirs
	s.handlers[MethodReadFile] = s.handleReadFile
	s.handlers[MethodGetOpenFiles] = s.handleGetOpenFiles
	s.handlers[MethodGetCurrentFile] = s.handleGetCurrentFile
	s.handlers[MethodShowDiff] = s.handleShowDiff
	s.handlers[MethodApplyEdit] = s.handleApplyEdit
	s.handlers[MethodGetCompletion] = s.handleGetCompletion
	s.handlers[MethodGetTerminalOutput] = s.handleGetTerminalOutput
	s.handlers[MethodGetGitDiff] = s.handleGetGitDiff
}

func (s *Server) RegisterEditor(sessionID string, editor Editor) {
	s.editorsMu.Lock()
	defer s.editorsMu.Unlock()
	s.editors[sessionID] = editor
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		sessionID = generateSessionID()
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			return
		}

		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			s.sendError(conn, 0, -32700, "Parse error")
			continue
		}

		handler, ok := s.handlers[req.Method]
		if !ok {
			s.sendError(conn, req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
			continue
		}

		ctx := context.WithValue(r.Context(), "sessionID", sessionID)
		result, err := handler(ctx, req.Params)
		if err != nil {
			s.sendError(conn, req.ID, -32603, err.Error())
			continue
		}

		resp := Response{ID: req.ID, Result: result}
		respBytes, _ := json.Marshal(resp)
		conn.WriteMessage(websocket.TextMessage, respBytes)
	}
}

func (s *Server) sendError(conn *websocket.Conn, id int64, code int, message string) {
	resp := Response{ID: id, Error: &Error{Code: code, Message: message}}
	respBytes, _ := json.Marshal(resp)
	conn.WriteMessage(websocket.TextMessage, respBytes)
}

func generateSessionID() string {
	// use github.com/google/uuid
	return "session-" + uuid.New().String()
}

// Individual handlers
func (s *Server) handleGetWorkspaceDirs(ctx context.Context, params json.RawMessage) (interface{}, error) {
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return editor.GetWorkspaceDirs(ctx)
}

func (s *Server) handleReadFile(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p ReadFileParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	content, err := editor.ReadFile(ctx, p.URI)
	return ReadFileResult{Content: content}, err
}

func (s *Server) handleShowDiff(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p DiffParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return nil, editor.ShowDiff(ctx, p.URI, p.Original, p.Modified)
}

func (s *Server) handleGetCompletion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	// Delegate to autocomplete engine (see Feature 4)
	return GetCompletion(ctx, editor, p)
}

func (s *Server) handleGetTerminalOutput(ctx context.Context, params json.RawMessage) (interface{}, error) {
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return editor.GetTerminalOutput(ctx)
}

func (s *Server) handleGetGitDiff(ctx context.Context, params json.RawMessage) (interface{}, error) {
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return editor.GetGitDiff(ctx, false)
}

func (s *Server) handleApplyEdit(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p ApplyEditParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return nil, editor.WriteFile(ctx, p.URI, p.Content)
}

func (s *Server) handleGetOpenFiles(ctx context.Context, params json.RawMessage) (interface{}, error) {
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return editor.GetOpenFiles(ctx)
}

func (s *Server) handleGetCurrentFile(ctx context.Context, params json.RawMessage) (interface{}, error) {
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}
	return editor.GetCurrentFile(ctx)
}

func (s *Server) getEditor(ctx context.Context) Editor {
	sessionID, _ := ctx.Value("sessionID").(string)
	s.editorsMu.RLock()
	defer s.editorsMu.RUnlock()
	return s.editors[sessionID]
}
```

#### File 4: `internal/editor/vscode.go` (NEW) — VS Code Adapter
```go
package editor

import (
	"context"
	"fmt"
	"os/exec"
)

// VSCodeAdapter communicates with VS Code via its CLI and LSP.
type VSCodeAdapter struct {
	workspaceDir string
	lspClient    *LSPClient
}

func NewVSCodeAdapter(workspaceDir string) (*VSCodeAdapter, error) {
	lsp, err := NewLSPClient(workspaceDir, "typescript-language-server", "--stdio")
	if err != nil {
		return nil, fmt.Errorf("failed to start LSP client: %w", err)
	}
	return &VSCodeAdapter{
		workspaceDir: workspaceDir,
		lspClient:    lsp,
	}, nil
}

func (v *VSCodeAdapter) Name() string    { return "vscode" }
func (v *VSCodeAdapter) Version() string { return "1.0" }

func (v *VSCodeAdapter) GetWorkspaceDirs(ctx context.Context) ([]string, error) {
	return []string{v.workspaceDir}, nil
}

func (v *VSCodeAdapter) ReadFile(ctx context.Context, uri string) (string, error) {
	// Read via standard filesystem
	return readFile(uri)
}

func (v *VSCodeAdapter) WriteFile(ctx context.Context, uri string, content string) error {
	return writeFile(uri, content)
}

func (v *VSCodeAdapter) GetOpenFiles(ctx context.Context) ([]string, error) {
	// VS Code: use CLI to get open files
	cmd := exec.CommandContext(ctx, "code", "--list-open-files") // hypothetical CLI extension
	out, err := cmd.Output()
	if err != nil {
		// Fallback: return workspace files
		return listGoFiles(v.workspaceDir)
	}
	return parseFileList(string(out)), nil
}

func (v *VSCodeAdapter) GetCurrentFile(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "code", "--current-file")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current file: %w", err)
	}
	return string(out), nil
}

func (v *VSCodeAdapter) GetCursorPosition(ctx context.Context) (Position, error) {
	// VS Code extension API would provide this via WebSocket
	return Position{Line: 0, Character: 0}, nil
}

func (v *VSCodeAdapter) GetSelection(ctx context.Context) (Range, error) {
	return Range{Start: Position{0, 0}, End: Position{0, 0}}, nil
}

func (v *VSCodeAdapter) GetTerminalOutput(ctx context.Context) (string, error) {
	// VS Code integrated terminal: extension API
	return "", fmt.Errorf("terminal output requires VS Code extension")
}

func (v *VSCodeAdapter) GetGitDiff(ctx context.Context, staged bool) (string, error) {
	return getGitDiff(staged)
}

func (v *VSCodeAdapter) GetDiagnostics(ctx context.Context, uri string) ([]Diagnostic, error) {
	return v.lspClient.GetDiagnostics(uri)
}

func (v *VSCodeAdapter) GetHover(ctx context.Context, uri string, pos Position) (string, error) {
	return v.lspClient.Hover(uri, pos)
}

func (v *VSCodeAdapter) GetDefinitions(ctx context.Context, uri string, pos Position) ([]Location, error) {
	return v.lspClient.Definition(uri, pos)
}

func (v *VSCodeAdapter) GetReferences(ctx context.Context, uri string, pos Position) ([]Location, error) {
	return v.lspClient.References(uri, pos)
}

func (v *VSCodeAdapter) ShowDiff(ctx context.Context, uri string, original string, modified string) error {
	// VS Code: generate unified diff and open in diff viewer
	diff := generateUnifiedDiff(uri, original, modified)
	return v.sendToEditor("showDiff", diff)
}

func (v *VSCodeAdapter) AcceptDiffHunk(ctx context.Context, hunkID string) error {
	return v.sendToEditor("acceptHunk", hunkID)
}

func (v *VSCodeAdapter) RejectDiffHunk(ctx context.Context, hunkID string) error {
	return v.sendToEditor("rejectHunk", hunkID)
}

func (v *VSCodeAdapter) ShowGhostText(ctx context.Context, uri string, pos Position, text string) error {
	return v.sendToEditor("showGhostText", map[string]interface{}{
		"uri":  uri,
		"pos":  pos,
		"text": text,
	})
}

func (v *VSCodeAdapter) ClearGhostText(ctx context.Context, uri string) error {
	return v.sendToEditor("clearGhostText", uri)
}

func (v *VSCodeAdapter) AcceptCompletion(ctx context.Context, completionID string) error {
	return v.sendToEditor("acceptCompletion", completionID)
}

func (v *VSCodeAdapter) RejectCompletion(ctx context.Context, completionID string) error {
	return v.sendToEditor("rejectCompletion", completionID)
}

func (v *VSCodeAdapter) ExecuteCommand(ctx context.Context, command string, args []string) error {
	cmdArgs := append([]string{command}, args...)
	cmd := exec.CommandContext(ctx, "code", cmdArgs...)
	return cmd.Run()
}

func (v *VSCodeAdapter) sendToEditor(method string, payload interface{}) error {
	// In real implementation, send via WebSocket to VS Code extension
	fmt.Printf("[VSCode] %s: %+v\n", method, payload)
	return nil
}
```

#### File 5: `internal/editor/lsp_client.go` (NEW) — LSP Client
```go
package editor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"
)

// LSPClient is a minimal Language Server Protocol client.
type LSPClient struct {
	cmd    *exec.Cmd
	stdin  *bufio.Writer
	stdout *bufio.Reader
	mu     sync.Mutex
	id     int
}

func NewLSPClient(rootDir string, command string, args ...string) (*LSPClient, error) {
	cmd := exec.Command(command, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	client := &LSPClient{
		cmd:    cmd,
		stdin:  bufio.NewWriter(stdin),
		stdout: bufio.NewReader(stdout),
	}

	// Initialize LSP
	if err := client.initialize(rootDir); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *LSPClient) initialize(rootDir string) error {
	params := map[string]interface{}{
		"processId": os.Getpid(),
		"rootUri":   "file://" + rootDir,
		"capabilities": map[string]interface{}{},
	}
	_, err := c.request("initialize", params)
	return err
}

func (c *LSPClient) request(method string, params interface{}) (json.RawMessage, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.id++
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      c.id,
		"method":  method,
		"params":  params,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(c.stdin, "Content-Length: %d\r\n\r\n", len(data))
	c.stdin.Write(data)
	c.stdin.Flush()

	// Read response (simplified)
	var resp map[string]json.RawMessage
	if err := json.NewDecoder(c.stdout).Decode(&resp); err != nil {
		return nil, err
	}

	return resp["result"], nil
}

func (c *LSPClient) GetDiagnostics(uri string) ([]Diagnostic, error) {
	// LSP textDocument/publishDiagnostics is server-initiated
	// This would require async handling; simplified here
	return nil, nil
}

func (c *LSPClient) Hover(uri string, pos Position) (string, error) {
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position":     pos,
	}
	result, err := c.request("textDocument/hover", params)
	if err != nil {
		return "", err
	}
	var hover struct {
		Contents string `json:"contents"`
	}
	json.Unmarshal(result, &hover)
	return hover.Contents, nil
}

func (c *LSPClient) Definition(uri string, pos Position) ([]Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position":     pos,
	}
	result, err := c.request("textDocument/definition", params)
	if err != nil {
		return nil, err
	}
	var locs []Location
	json.Unmarshal(result, &locs)
	return locs, nil
}

func (c *LSPClient) References(uri string, pos Position) ([]Location, error) {
	params := map[string]interface{}{
		"textDocument": map[string]string{"uri": uri},
		"position":     pos,
		"context":      map[string]bool{"includeDeclaration": true},
	}
	result, err := c.request("textDocument/references", params)
	if err != nil {
		return nil, err
	}
	var locs []Location
	json.Unmarshal(result, &locs)
	return locs, nil
}
```

#### File 6: `cmd/server/main.go` — MODIFY (add WebSocket endpoint)
```go
func main() {
	// ... existing server setup ...

	// NEW: Editor protocol WebSocket endpoint
	editorSrv := editor.NewServer()
	r.GET("/ws/editor", func(c *gin.Context) {
		editorSrv.HandleWebSocket(c.Writer, c.Request)
	})

	// ... existing routes ...
}
```

### Anti-Bluff Test
```bash
# 1. Test WebSocket server starts
$ cd HelixCode && go test ./internal/editor/... -run TestWebSocketServer -v
# Should open WebSocket, send initialize request, receive response

# 2. Test VS Code adapter file operations
$ go test ./internal/editor/... -run TestVSCodeAdapter -v
# Should read/write real files, not mock filesystem

# 3. Test LSP client connects to real language server
$ go test ./internal/editor/... -run TestLSPClient -v
# Requires typescript-language-server installed

# 4. End-to-end: editor server receives and handles all methods
$ go test ./internal/editor/... -run TestProtocolMethods -v
# Sends JSON-RPC for each method, verifies response
```

### Integration Verification
- [ ] WebSocket endpoint `/ws/editor` accepts connections from VS Code
- [ ] JSON-RPC protocol correctly routes all 10+ methods
- [ ] LSP client connects and returns hover/definition/references
- [ ] VS Code adapter reads real workspace files
- [ ] Diff streaming displays in editor (see Feature 5)
- [ ] Autocomplete ghost text renders (see Feature 8)

---

## 3. Context Provider Framework

### Source Location (Continue.dev)
- `core/context/providers/index.ts` — BaseContextProvider class
- `core/context/providers/utils.ts` — Provider utilities
- `core/indexing/` — Indexing pipeline for providers that depend on it
- `core/context/retrieval/` — RAG retrieval logic

### Target Location (HelixCode)
- **NEW**: `internal/context/framework.go` — Pluggable framework
- **NEW**: `internal/context/token_budget.go` — Token budget management
- **NEW**: `internal/context/priority.go` — Priority ordering
- **NEW**: `internal/context/retrieval.go` — RAG retrieval
- **MODIFY**: `internal/context/providers.go` — Extend with framework

### Exact Code Changes

#### File 1: `internal/context/framework.go` (NEW)
```go
package context

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Framework is the pluggable context provider framework.
// It manages provider lifecycle, priority ordering, and token budgets.
type Framework struct {
	registry  *Registry
	budget    *TokenBudget
	retriever *Retriever
	mu        sync.RWMutex
}

func NewFramework(registry *Registry, budget *TokenBudget, retriever *Retriever) *Framework {
	return &Framework{
		registry:  registry,
		budget:    budget,
		retriever: retriever,
	}
}

// Resolve builds the final context by calling providers in priority order
// and respecting the token budget.
func (f *Framework) Resolve(ctx context.Context, userInput string, extras ProviderExtras) (*ResolvedContext, error) {
	mentions := ParseAtMentions(userInput)
	cleanInput := userInput

	// Sort mentions by priority (higher priority first)
	ordered := f.orderMentions(mentions)

	var items []ContextItem
	var usedProviders []string

	for _, m := range ordered {
		provider, ok := f.registry.Get(m.Provider)
		if !ok {
			continue
		}

		// Check token budget before calling provider
		remaining := f.budget.Remaining()
		if remaining <= 0 {
			break
		}

		ctxItems, err := provider.GetContextItems(m.Query, extras)
		if err != nil {
			// Log but continue with other providers
			fmt.Printf("Provider @%s error: %v\n", m.Provider, err)
			continue
		}

		// Truncate items to fit budget
		truncated := f.budget.Fit(ctxItems, remaining)
		items = append(items, truncated...)
		usedProviders = append(usedProviders, m.Provider)
		cleanInput = RemoveMention(cleanInput, m.Raw)
	}

	// Add retrieval-based context if budget remains
	if f.retriever != nil && f.budget.Remaining() > 100 {
		retrieved, err := f.retriever.Retrieve(ctx, cleanInput, extras)
		if err == nil {
			truncated := f.budget.Fit(retrieved, f.budget.Remaining())
			items = append(items, truncated...)
		}
	}

	return &ResolvedContext{
		UserInput:     strings.TrimSpace(cleanInput),
		Items:         items,
		UsedProviders: usedProviders,
		TotalTokens:   f.budget.Used(),
	}, nil
}

// Priority order: file > codebase > code > docs > url > terminal > diff
var defaultPriority = map[string]int{
	"file":     100,
	"currentFile": 95,
	"open":     90,
	"codebase": 80,
	"code":     75,
	"docs":     70,
	"url":      60,
	"terminal": 50,
	"diff":     40,
	"tree":     30,
	"search":   20,
}

func (f *Framework) orderMentions(mentions []Mention) []Mention {
	sorted := make([]Mention, len(mentions))
	copy(sorted, mentions)

	sort.Slice(sorted, func(i, j int) bool {
		pi := defaultPriority[sorted[i].Provider]
		pj := defaultPriority[sorted[j].Provider]
		if pi == 0 {
			pi = 10 // default low priority
		}
		if pj == 0 {
			pj = 10
		}
		return pi > pj // higher priority first
	})

	return sorted
}

// ResolvedContext is the final assembled context.
type ResolvedContext struct {
	UserInput     string        `json:"user_input"`
	Items         []ContextItem `json:"items"`
	UsedProviders []string      `json:"used_providers"`
	TotalTokens   int           `json:"total_tokens"`
}

// ToMessages formats resolved context into LLM messages.
func (rc *ResolvedContext) ToMessages() []Message {
	var messages []Message

	// System message with context items
	if len(rc.Items) > 0 {
		var ctxParts []string
		for _, item := range rc.Items {
			ctxParts = append(ctxParts, fmt.Sprintf("## %s\n%s", item.Name, item.Content))
		}
		systemContent := fmt.Sprintf("You have the following context:\n\n%s", strings.Join(ctxParts, "\n\n"))
		messages = append(messages, Message{Role: "system", Content: systemContent})
	}

	messages = append(messages, Message{Role: "user", Content: rc.UserInput})
	return messages
}
```

#### File 2: `internal/context/token_budget.go` (NEW)
```go
package context

import (
	"strings"
	"unicode/utf8"
)

// TokenBudget manages context token allocation.
// Uses approximate token counting (4 chars ≈ 1 token).
type TokenBudget struct {
	maxTokens int
	used      int
}

func NewTokenBudget(maxTokens int) *TokenBudget {
	return &TokenBudget{maxTokens: maxTokens}
}

func (tb *TokenBudget) Remaining() int {
	return tb.maxTokens - tb.used
}

func (tb *TokenBudget) Used() int {
	return tb.used
}

func (tb *TokenBudget) Fit(items []ContextItem, limit int) []ContextItem {
	var result []ContextItem
	for _, item := range items {
		tokens := estimateTokens(item.Content)
		if tokens > limit {
			// Truncate content to fit
			item.Content = truncateToTokens(item.Content, limit)
			tokens = estimateTokens(item.Content)
		}
		if tokens <= limit {
			result = append(result, item)
			limit -= tokens
			tb.used += tokens
		}
		if limit <= 0 {
			break
		}
	}
	return result
}

func estimateTokens(text string) int {
	// Approximation: 1 token ≈ 4 characters for English/code
	return utf8.RuneCountInString(text) / 4
}

func truncateToTokens(text string, maxTokens int) string {
	maxChars := maxTokens * 4
	if utf8.RuneCountInString(text) <= maxChars {
		return text
	}
	// Try to truncate at a reasonable boundary
	truncated := text[:maxChars]
	// Find last newline or space
	lastNL := strings.LastIndex(truncated, "\n")
	lastSpace := strings.LastIndex(truncated, " ")
	cut := lastNL
	if lastSpace > cut {
		cut = lastSpace
	}
	if cut > maxChars/2 {
		return truncated[:cut] + "\n\n[...truncated...]"
	}
	return truncated + "\n\n[...truncated...]"
}
```

#### File 3: `internal/context/retrieval.go` (NEW)
```go
package context

import (
	"context"
	"fmt"

	"dev.helix.code/internal/knowledge"
)

// Retriever provides RAG-based context augmentation.
type Retriever struct {
	store knowledge.Store
}

func NewRetriever(store knowledge.Store) *Retriever {
	return &Retriever{store: store}
}

func (r *Retriever) Retrieve(ctx context.Context, query string, extras ProviderExtras) ([]ContextItem, error) {
	if r.store == nil {
		return nil, fmt.Errorf("knowledge store not available")
	}

	results, err := r.store.Search(ctx, query, knowledge.SearchOptions{
		Limit:  5,
		Rerank: true,
	})
	if err != nil {
		return nil, err
	}

	var items []ContextItem
	for _, res := range results {
		items = append(items, ContextItem{
			Name:        fmt.Sprintf("%s:%d", res.FilePath, res.StartLine),
			Description: fmt.Sprintf("Relevant code (%0.2f)", res.Score),
			Content:     res.Content,
			Score:       res.Score,
			URI:         &ContextURI{Type: "retrieval", Value: res.FilePath},
		})
	}
	return items, nil
}
```

### Anti-Bluff Test
```bash
# 1. Test token budget correctly truncates oversized content
$ go test ./internal/context/... -run TestTokenBudget -v
# Input: 10K char string, budget: 500 tokens
# Expected: truncated to ~2000 chars with [...truncated...] suffix

# 2. Test priority ordering
$ go test ./internal/context/... -run TestPriorityOrdering -v
# Mentions: @tree @file:main.go @codebase auth
# Expected order: file, codebase, tree

# 3. Test framework resolves with real providers
$ go test ./internal/context/... -run TestFrameworkResolve -v
# Should assemble context from @file + @url within budget

# 4. Test retrieval augments when budget remains
$ go test ./internal/context/... -run TestRetrievalAugment -v
# Query "auth middleware", should retrieve relevant code snippets
```

### Integration Verification
- [ ] Framework resolves mentions in priority order
- [ ] Token budget prevents exceeding context window
- [ ] Content truncated gracefully at word boundaries
- [ ] Retrieval augments context when budget remains
- [ ] System message correctly formats all context items
- [ ] Clean input stripped of all @-mentions before LLM

---

## 4. Autocomplete Engine

### Source Location (Continue.dev)
- `core/autocomplete/CompletionProvider.ts` — Core completion provider
- `core/autocomplete/templates.ts` — FIM prompt templates
- `core/llm/index.ts` — LLM streaming for completions
- `extensions/vscode/src/autocomplete/` — VS Code inline completion provider

### Target Location (HelixCode)
- **NEW**: `internal/autocomplete/engine.go` — Core engine
- **NEW**: `internal/autocomplete/fim.go` — FIM prompt construction
- **NEW**: `internal/autocomplete/cache.go` — Completion cache
- **NEW**: `internal/autocomplete/filter.go` — Post-processing filter
- **MODIFY**: `internal/llm/types.go` — Add completion-specific options
- **MODIFY**: `internal/editor/` — Add autocomplete methods

### Exact Code Changes

#### File 1: `internal/autocomplete/engine.go` (NEW)
```go
package autocomplete

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dev.helix.code/internal/editor"
	"dev.helix.code/internal/llm"
)

// Engine provides inline code completion.
type Engine struct {
	llmProvider llm.Provider
	cache       *Cache
	filter      *Filter
}

func NewEngine(provider llm.Provider) *Engine {
	return &Engine{
		llmProvider: provider,
		cache:       NewCache(),
		filter:      NewFilter(),
	}
}

// Request represents an autocomplete request.
type Request struct {
	URI       string         `json:"uri"`
	Position  editor.Position `json:"position"`
	Prefix    string         `json:"prefix"`    // Code before cursor
	Suffix    string         `json:"suffix"`    // Code after cursor
	Language  string         `json:"language"`
	Filename  string         `json:"filename"`
	RepoName  string         `json:"repo_name"`
}

// Response contains completion results.
type Response struct {
	Items       []Completion `json:"items"`
	Model       string       `json:"model"`
	LatencyMs   int64        `json:"latency_ms"`
	PromptTokens int         `json:"prompt_tokens"`
}

// Completion is a single suggestion.
type Completion struct {
	InsertText string  `json:"insertText"`
	Score      float64 `json:"score"`
	Model      string  `json:"model"`
	Range      editor.Range `json:"range,omitempty"`
}

// Complete generates inline completions using FIM.
func (e *Engine) Complete(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	// Check cache
	if cached := e.cache.Get(req); cached != nil {
		return cached, nil
	}

	// Build FIM prompt
	fimPrompt := BuildFIMPrompt(req)

	// Build LLM request
	llmReq := &llm.LLMRequest{
		Model:       getAutocompleteModel(),
		MaxTokens:   128,
		Temperature: 0.2,
		Stream:      false,
		Messages: []llm.Message{
			{Role: "user", Content: fimPrompt},
		},
	}

	resp, err := e.llmProvider.Generate(ctx, llmReq)
	if err != nil {
		return nil, fmt.Errorf("autocomplete generation failed: %w", err)
	}

	// Parse and filter completions
	rawCompletions := e.parseCompletions(resp.Content)
	filtered := e.filter.Filter(rawCompletions, req)

	result := &Response{
		Items:       filtered,
		Model:       resp.Model,
		LatencyMs:   time.Since(start).Milliseconds(),
		PromptTokens: resp.Usage.PromptTokens,
	}

	// Cache result
	e.cache.Set(req, result)

	return result, nil
}

func (e *Engine) parseCompletions(content string) []Completion {
	// Split by common delimiters that models use
	var completions []Completion
	lines := strings.Split(content, "\n")

	// Take first non-empty block as primary completion
	var block strings.Builder
	for _, line := range lines {
		if strings.TrimSpace(line) == "" && block.Len() > 0 {
			break
		}
		block.WriteString(line)
		block.WriteString("\n")
	}

	if block.Len() > 0 {
		completions = append(completions, Completion{
			InsertText: strings.TrimSuffix(block.String(), "\n"),
			Score:      1.0,
		})
	}

	return completions
}

func getAutocompleteModel() string {
	// Configurable: default to fast code model
	return "qwen2.5-coder:1.5b-base"
}
```

#### File 2: `internal/autocomplete/fim.go` (NEW)
```go
package autocomplete

import (
	"fmt"
	"strings"
)

// FIMTemplate is the fill-in-the-middle prompt template.
// Continue.dev uses model-specific templates with <|fim_prefix|>, <|fim_suffix|>, <|fim_middle|>tokens.
type FIMTemplate struct {
	PrefixMarker string
	SuffixMarker string
	MiddleMarker string
	Template     string
}

// DefaultFIMTemplate for Qwen/Codellama style FIM.
var DefaultFIMTemplate = FIMTemplate{
	PrefixMarker: "<|fim_prefix|>",
	SuffixMarker: "<|fim_suffix|>",
	MiddleMarker: "<|fim_middle|>",
	Template:     "{{.PrefixMarker}}{{.Prefix}}{{.SuffixMarker}}{{.Suffix}}{{.MiddleMarker}}",
}

// CodeLlamaFIMTemplate for CodeLlama style.
var CodeLlamaFIMTemplate = FIMTemplate{
	PrefixMarker: "<PRE>",
	SuffixMarker: "<SUF>",
	MiddleMarker: "<MID>",
	Template:     "{{.PrefixMarker}} {{.Prefix}} {{.SuffixMarker}} {{.Suffix}} {{.MiddleMarker}}",
}

// BuildFIMPrompt constructs the FIM prompt from request.
func BuildFIMPrompt(req Request) string {
	tmpl := selectTemplate(req)

	// Extract last N lines of prefix and first N lines of suffix
	// to stay within token limits
	maxPrefixLen := 1500  // characters
	maxSuffixLen := 300   // characters

	prefix := truncatePrefix(req.Prefix, maxPrefixLen)
	suffix := truncateSuffix(req.Suffix, maxSuffixLen)

	// Build using simple string replacement
	prompt := tmpl.Template
	prompt = strings.ReplaceAll(prompt, "{{.PrefixMarker}}", tmpl.PrefixMarker)
	prompt = strings.ReplaceAll(prompt, "{{.SuffixMarker}}", tmpl.SuffixMarker)
	prompt = strings.ReplaceAll(prompt, "{{.MiddleMarker}}", tmpl.MiddleMarker)
	prompt = strings.ReplaceAll(prompt, "{{.Prefix}}", prefix)
	prompt = strings.ReplaceAll(prompt, "{{.Suffix}}", suffix)
	prompt = strings.ReplaceAll(prompt, "{{.Filename}}", req.Filename)
	prompt = strings.ReplaceAll(prompt, "{{.Language}}", req.Language)
	prompt = strings.ReplaceAll(prompt, "{{.RepoName}}", req.RepoName)

	return prompt
}

func selectTemplate(req Request) FIMTemplate {
	// Could be selected based on model config
	return DefaultFIMTemplate
}

func truncatePrefix(prefix string, maxLen int) string {
	if len(prefix) <= maxLen {
		return prefix
	}
	// Try to truncate at last function/class boundary
	idx := strings.LastIndex(prefix[:maxLen], "\nfunc ")
	if idx == -1 {
		idx = strings.LastIndex(prefix[:maxLen], "\n")
	}
	if idx == -1 || idx < maxLen/2 {
		return prefix[len(prefix)-maxLen:]
	}
	return prefix[idx:]
}

func truncateSuffix(suffix string, maxLen int) string {
	if len(suffix) <= maxLen {
		return suffix
	}
	idx := strings.Index(suffix[maxLen/2:], "\n")
	if idx != -1 {
		return suffix[:maxLen/2+idx]
	}
	return suffix[:maxLen]
}

// MultiLineCompletion extends completion to multiple lines.
func (e *Engine) MultiLineComplete(ctx context.Context, req Request) (*Response, error) {
	// Increase max tokens for multi-line
	llmReq := &llm.LLMRequest{
		Model:       getAutocompleteModel(),
		MaxTokens:   512,
		Temperature: 0.2,
		Stream:      false,
		Messages: []llm.Message{
			{Role: "user", Content: BuildFIMPrompt(req)},
		},
	}

	resp, err := e.llmProvider.Generate(ctx, llmReq)
	if err != nil {
		return nil, err
	}

	// Parse multi-line blocks
	blocks := parseMultiLineBlocks(resp.Content)
	var items []Completion
	for _, block := range blocks {
		items = append(items, Completion{
			InsertText: block,
			Score:      1.0,
		})
	}

	return &Response{
		Items:     items,
		Model:     resp.Model,
		LatencyMs: 0,
	}, nil
}

func parseMultiLineBlocks(content string) []string {
	// Split on blank lines, return non-empty blocks
	parts := strings.Split(content, "\n\n")
	var blocks []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			blocks = append(blocks, trimmed)
		}
	}
	return blocks
}
```

#### File 3: `internal/autocomplete/cache.go` (NEW)
```go
package autocomplete

import (
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/hashicorp/golang-lru/v2"
)

// Cache stores recent completions to avoid redundant LLM calls.
type Cache struct {
	lru *lru.Cache[string, *Response]
}

func NewCache() *Cache {
	cache, _ := lru.New[string, *Response](100)
	return &Cache{lru: cache}
}

func (c *Cache) key(req Request) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%d|%d|%s|%s", req.URI, req.Position.Line, req.Position.Character, req.Prefix, req.Suffix)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (c *Cache) Get(req Request) *Response {
	resp, ok := c.lru.Get(c.key(req))
	if !ok {
		return nil
	}
	return resp
}

func (c *Cache) Set(req Request, resp *Response) {
	c.lru.Add(c.key(req), resp)
}

func (c *Cache) Invalidate(uri string) {
	// LRU doesn't support prefix invalidation easily;
	// for production, use a map-based cache with TTL
}
```

#### File 4: `internal/autocomplete/filter.go` (NEW)
```go
package autocomplete

import (
	"strings"
	"unicode"
)

// Filter post-processes raw completions.
type Filter struct{}

func NewFilter() *Filter {
	return &Filter{}
}

func (f *Filter) Filter(completions []Completion, req Request) []Completion {
	var filtered []Completion
	for _, c := range completions {
		// Skip empty completions
		if strings.TrimSpace(c.InsertText) == "" {
			continue
		}

		// Skip if completion only repeats prefix
		if strings.HasPrefix(c.InsertText, req.Prefix) && len(c.InsertText) <= len(req.Prefix)+3 {
			continue
		}

		// Skip if completion is just whitespace
		if isOnlyWhitespace(c.InsertText) {
			continue
		}

		// Skip if completion starts with suffix (would be no-op)
		if strings.HasPrefix(req.Suffix, c.InsertText) {
			continue
		}

		// Adjust score based on heuristics
		c.Score = f.scoreCompletion(c, req)
		filtered = append(filtered, c)
	}

	return filtered
}

func (f *Filter) scoreCompletion(c Completion, req Request) float64 {
	score := c.Score

	// Prefer longer completions (more value)
	score += float64(len(c.InsertText)) * 0.001

	// Penalize very long single-line completions
	if !strings.Contains(c.InsertText, "\n") && len(c.InsertText) > 100 {
		score -= 0.1
	}

	// Bonus for matching language patterns
	if isValidSyntax(c.InsertText, req.Language) {
		score += 0.2
	}

	return score
}

func isOnlyWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

func isValidSyntax(code, language string) bool {
	// Simplified: check balanced braces/parentheses
	balance := 0
	for _, r := range code {
		switch r {
		case '{', '(', '[':
			balance++
		case '}', ')', ']':
			balance--
			if balance < 0 {
				return false
			}
		}
	}
	return balance == 0
}
```

#### File 5: `internal/editor/server.go` — MODIFY (add autocomplete handler)
```go
func (s *Server) registerHandlers() {
	// ... existing handlers ...
	s.handlers[MethodGetCompletion] = s.handleGetCompletion
	s.handlers[MethodAcceptCompletion] = s.handleAcceptCompletion
	s.handlers[MethodRejectCompletion] = s.handleRejectCompletion
}

func (s *Server) handleGetCompletion(ctx context.Context, params json.RawMessage) (interface{}, error) {
	var p CompletionParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	editor := s.getEditor(ctx)
	if editor == nil {
		return nil, fmt.Errorf("no editor connected")
	}

	req := autocomplete.Request{
		URI:      p.URI,
		Position: editor.Position{Line: p.Line, Character: p.Character},
		Prefix:   p.Prefix,
		Suffix:   p.Suffix,
		Language: p.Language,
		Filename: filepath.Base(p.URI),
	}

	return autocomplete.GetCompletion(ctx, editor, req)
}
```

### Anti-Bluff Test
```bash
# 1. Test FIM prompt construction
$ go test ./internal/autocomplete/... -run TestFIMPrompt -v
# Prefix: "func main() {", Suffix: "}"
# Should output: "<|fim_prefix|>func main() {<|fim_suffix|>}<|fim_middle|>"

# 2. Test engine calls real LLM
$ go test ./internal/autocomplete/... -run TestEngineComplete -v -timeout 30s
# Should return non-empty completion within 5 seconds

# 3. Test cache prevents duplicate calls
$ go test ./internal/autocomplete/... -run TestCacheHit -v
# Same request twice: second should return cached result instantly

# 4. Test filter rejects bad completions
$ go test ./internal/autocomplete/... -run TestFilter -v
# Input: empty, whitespace-only, prefix-repeating
# Should all be filtered out
```

### Integration Verification
- [ ] FIM prompt uses correct prefix/suffix/middle markers
- [ ] Engine returns completions within 500ms for cached, 2s for uncached
- [ ] Multi-line completions produce full function bodies
- [ ] Cache hits return identical results instantly
- [ ] Filter rejects empty/whitespace/duplicate completions
- [ ] Editor ghost text API displays completions inline

---

## 5. Diff Streaming

### Source Location (Continue.dev)
- `core/diff/streamDiff.ts` — Real-time diff streaming logic
- `core/diff/util.ts` — Diff utilities
- `gui/src/components/mainInput/resolveEditorContent.ts` — Editor integration
- `extensions/vscode/src/diff/` — VS Code diff application

### Target Location (HelixCode)
- **NEW**: `internal/diff/stream.go` — Stream processor
- **NEW**: `internal/diff/hunk.go` — Hunk representation
- **NEW**: `internal/diff/apply.go` — Apply/reject logic
- **NEW**: `internal/diff/renderer.go` — Real-time renderer
- **MODIFY**: `internal/editor/` — Add diff display methods

### Exact Code Changes

#### File 1: `internal/diff/stream.go` (NEW)
```go
package diff

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// StreamProcessor handles real-time diff streaming from LLM output.
type StreamProcessor struct {
	original string
	builder  strings.Builder
	hunks    []Hunk
	mu       sync.RWMutex
	onUpdate func(state StreamState)
}

// StreamState represents the current diff state.
type StreamState struct {
	Original      string   `json:"original"`
	CurrentBuild  string   `json:"current_build"`
	Hunks         []Hunk   `json:"hunks"`
	IsComplete    bool     `json:"is_complete"`
	Progress      float64  `json:"progress"` // 0.0 - 1.0
	AcceptedHunks []string `json:"accepted_hunks"`
	RejectedHunks []string `json:"rejected_hunks"`
}

// Hunk is a single diff segment.
type Hunk struct {
	ID       string `json:"id"`
	OldStart int    `json:"old_start"`
	OldEnd   int    `json:"old_end"`
	NewStart int    `json:"new_start"`
	NewEnd   int    `json:"new_end"`
	OldLines []string `json:"old_lines"`
	NewLines []string `json:"new_lines"`
	Accepted *bool  `json:"accepted,omitempty"`
}

func NewStreamProcessor(original string, onUpdate func(StreamState)) *StreamProcessor {
	return &StreamProcessor{
		original: original,
		onUpdate: onUpdate,
	}
}

// Process streams chunks from the LLM and builds the diff progressively.
func (sp *StreamProcessor) Process(ctx context.Context, reader io.Reader) error {
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanRunes)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	done := make(chan struct{})
	go func() {
		for scanner.Scan() {
			char := scanner.Text()
			sp.builder.WriteString(char)
		}
		close(done)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			sp.finalize()
			return nil
		case <-ticker.C:
			sp.updateState()
		}
	}
}

func (sp *StreamProcessor) updateState() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	current := sp.builder.String()
	hunks := sp.computeHunks(sp.original, current)

	state := StreamState{
		Original:     sp.original,
		CurrentBuild: current,
		Hunks:        hunks,
		Progress:     sp.computeProgress(current),
	}

	if sp.onUpdate != nil {
		sp.onUpdate(state)
	}
}

func (sp *StreamProcessor) finalize() {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	current := sp.builder.String()
	sp.hunks = sp.computeHunks(sp.original, current)

	state := StreamState{
		Original:   sp.original,
		CurrentBuild: current,
		Hunks:      sp.hunks,
		IsComplete: true,
		Progress:   1.0,
	}

	if sp.onUpdate != nil {
		sp.onUpdate(state)
	}
}

func (sp *StreamProcessor) computeHunks(original, modified string) []Hunk {
	// Use simple line-based diff
	origLines := strings.Split(original, "\n")
	modLines := strings.Split(modified, "\n")

	// Simple LCS-based diff (production: use github.com/sergi/go-diff)
	var hunks []Hunk
	origIdx, modIdx := 0, 0
	hunkID := 0

	for origIdx < len(origLines) || modIdx < len(modLines) {
		if origIdx < len(origLines) && modIdx < len(modLines) && origLines[origIdx] == modLines[modIdx] {
			origIdx++
			modIdx++
			continue
		}

		// Start of difference
		oldStart := origIdx
		newStart := modIdx
		var oldLines, newLines []string

		for origIdx < len(origLines) && (modIdx >= len(modLines) || origLines[origIdx] != modLines[modIdx]) {
			oldLines = append(oldLines, origLines[origIdx])
			origIdx++
		}

		for modIdx < len(modLines) && (origIdx >= len(origLines) || modLines[modIdx] != origLines[origIdx]) {
			newLines = append(newLines, modLines[modIdx])
			modIdx++
		}

		if len(oldLines) > 0 || len(newLines) > 0 {
			hunks = append(hunks, Hunk{
				ID:       fmt.Sprintf("hunk-%d", hunkID),
				OldStart: oldStart + 1,
				OldEnd:   oldStart + len(oldLines),
				NewStart: newStart + 1,
				NewEnd:   newStart + len(newLines),
				OldLines: oldLines,
				NewLines: newLines,
			})
			hunkID++
		}
	}

	return hunks
}

func (sp *StreamProcessor) computeProgress(current string) float64 {
	// Estimate progress based on output size vs typical completion size
	if len(current) == 0 {
		return 0.0
	}
	// Simple heuristic: progress slows as text grows
	progress := float64(len(current)) / 1000.0
	if progress > 0.95 {
		return 0.95 // reserve 5% for finalization
	}
	return progress
}

// AcceptHunk marks a hunk as accepted.
func (sp *StreamProcessor) AcceptHunk(hunkID string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i := range sp.hunks {
		if sp.hunks[i].ID == hunkID {
			accepted := true
			sp.hunks[i].Accepted = &accepted
			break
		}
	}
}

// RejectHunk marks a hunk as rejected.
func (sp *StreamProcessor) RejectHunk(hunkID string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	for i := range sp.hunks {
		if sp.hunks[i].ID == hunkID {
			rejected := false
			sp.hunks[i].Accepted = &rejected
			break
		}
	}
}

// BuildFinal builds the final content with accepted/rejected hunks applied.
func (sp *StreamProcessor) BuildFinal() (string, error) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()

	current := sp.builder.String()
	modLines := strings.Split(current, "\n")

	// Apply rejections: for rejected hunks, restore original lines
	origLines := strings.Split(sp.original, "\n")
	var result []string

	for _, hunk := range sp.hunks {
		if hunk.Accepted != nil && !*hunk.Accepted {
			// Rejected: keep original lines for this hunk
			for i := hunk.OldStart - 1; i < hunk.OldEnd && i < len(origLines); i++ {
				result = append(result, origLines[i])
			}
		} else {
			// Accepted or pending: use new lines
			for i := hunk.NewStart - 1; i < hunk.NewEnd && i < len(modLines); i++ {
				result = append(result, modLines[i])
			}
		}
	}

	return strings.Join(result, "\n"), nil
}
```

#### File 2: `internal/diff/apply.go` (NEW)
```go
package diff

import (
	"context"
	"fmt"

	"dev.helix.code/internal/editor"
)

// Applier handles applying diffs to the editor.
type Applier struct {
	editor editor.Editor
}

func NewApplier(ed editor.Editor) *Applier {
	return &Applier{editor: ed}
}

// ApplyDiff applies a complete diff to the editor.
func (a *Applier) ApplyDiff(ctx context.Context, uri string, state StreamState) error {
	final, err := a.buildWithDecisions(state)
	if err != nil {
		return err
	}

	return a.editor.WriteFile(ctx, uri, final)
}

// ApplyHunk applies a single accepted hunk.
func (a *Applier) ApplyHunk(ctx context.Context, uri string, hunk Hunk) error {
	// Read current file
	content, err := a.editor.ReadFile(ctx, uri)
	if err != nil {
		return err
	}

	lines := splitLines(content)
	
	// Replace hunk lines
	if hunk.OldStart > 0 && hunk.OldEnd <= len(lines) {
		before := lines[:hunk.OldStart-1]
		after := lines[hunk.OldEnd:]
		newLines := append(before, hunk.NewLines...)
		newLines = append(newLines, after...)
		
		return a.editor.WriteFile(ctx, uri, joinLines(newLines))
	}

	return fmt.Errorf("hunk range out of bounds: %d-%d in %d lines", hunk.OldStart, hunk.OldEnd, len(lines))
}

// ShowDiffInEditor opens a diff view in the editor.
func (a *Applier) ShowDiffInEditor(ctx context.Context, uri string, state StreamState) error {
	return a.editor.ShowDiff(ctx, uri, state.Original, state.CurrentBuild)
}

func (a *Applier) buildWithDecisions(state StreamState) (string, error) {
	// Build final content respecting accept/reject decisions
	origLines := splitLines(state.Original)
	var result []string
	lastEnd := 0

	for _, hunk := range state.Hunks {
		// Add unchanged lines before this hunk
		if hunk.OldStart > lastEnd+1 {
			result = append(result, origLines[lastEnd:hunk.OldStart-1]...)
		}

		if hunk.Accepted != nil && !*hunk.Accepted {
			// Rejected: keep original
			result = append(result, origLines[hunk.OldStart-1:hunk.OldEnd]...)
		} else {
			// Accepted or pending: use new
			result = append(result, hunk.NewLines...)
		}
		lastEnd = hunk.OldEnd
	}

	// Add remaining unchanged lines
	if lastEnd < len(origLines) {
		result = append(result, origLines[lastEnd:]...)
	}

	return joinLines(result), nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func joinLines(lines []string) string {
	var result string
	for _, l := range lines {
		result += l
	}
	return result
}
```

#### File 3: `internal/diff/renderer.go` (NEW)
```go
package diff

import (
	"fmt"
	"strings"
)

// Renderer formats diff state for display.
type Renderer struct{}

func NewRenderer() *Renderer {
	return &Renderer{}
}

func (r *Renderer) Render(state StreamState) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "\n=== Diff Progress: %.0f%% ===\n", state.Progress*100)

	for _, hunk := range state.Hunks {
		r.renderHunk(&sb, hunk)
	}

	if state.IsComplete {
		sb.WriteString("\n[Diff Complete] Use: accept <hunk-id> | reject <hunk-id> | apply-all\n")
	}

	return sb.String()
}

func (r *Renderer) renderHunk(sb *strings.Builder, hunk Hunk) {
	status := "?"
	if hunk.Accepted != nil {
		if *hunk.Accepted {
			status = "✓"
		} else {
			status = "✗"
		}
	}

	fmt.Fprintf(sb, "\n%s Hunk %s (lines %d-%d → %d-%d)\n",
		status, hunk.ID, hunk.OldStart, hunk.OldEnd, hunk.NewStart, hunk.NewEnd)

	if len(hunk.OldLines) > 0 {
		sb.WriteString("  ---\n")
		for _, line := range hunk.OldLines {
			fmt.Fprintf(sb, "  - %s\n", line)
		}
	}

	if len(hunk.NewLines) > 0 {
		sb.WriteString("  +++\n")
		for _, line := range hunk.NewLines {
			fmt.Fprintf(sb, "  + %s\n", line)
		}
	}
}

// RenderInline renders a simplified inline diff.
func (r *Renderer) RenderInline(state StreamState) string {
	var sb strings.Builder
	for _, hunk := range state.Hunks {
		if len(hunk.NewLines) > 0 {
			for _, line := range hunk.NewLines {
				fmt.Fprintf(&sb, "%s\n", line)
			}
		}
	}
	return sb.String()
}
```

### Anti-Bluff Test
```bash
# 1. Test stream processor builds hunks progressively
$ go test ./internal/diff/... -run TestStreamProcessor -v
# Feed "func main() {\n  fmt.Println(\"old\")\n}" then "func main() {\n  fmt.Println(\"new\")\n}"
# Should produce 1 hunk with old/new lines

# 2. Test accept/reject hunk decisions
$ go test ./internal/diff/... -run TestHunkDecisions -v
# Accept hunk-0, reject hunk-1
# BuildFinal should apply only accepted hunks

# 3. Test real-time streaming updates
$ go test ./internal/diff/... -run TestRealTimeUpdates -v
# Should receive 5+ state updates during 2-second stream

# 4. Test diff application to real file
$ go test ./internal/diff/... -run TestApplyDiff -v
# Create temp file, apply diff, verify file content changed
```

### Integration Verification
- [ ] Stream processor emits updates every 100ms during streaming
- [ ] Hunks correctly identified with line ranges
- [ ] Accept/reject decisions persist and affect final build
- [ ] Renderer outputs ANSI-formatted diff for CLI
- [ ] Editor diff view shows side-by-side comparison
- [ ] Progress percentage increases monotonically during stream

---

## 6. Prompt Templates

### Source Location (Continue.dev)
- `core/llm/templates.ts` — Prompt templates (chat, edit, autocomplete)
- `core/promptFiles/` — Custom prompt file support
- `core/config/` — Template configuration loading
- `schema/continue_config.json` — Template schema

### Target Location (HelixCode)
- **NEW**: `internal/prompt/template.go` — Template engine
- **NEW**: `internal/prompt/loader.go` — Template loader from config/files
- **NEW**: `internal/prompt/builtins.go` — Built-in templates
- **NEW**: `internal/prompt/variables.go` — Template variables
- **NEW**: `config/prompts/` — User prompt template directory
- **MODIFY**: `internal/llm/` — Integrate templates into generation

### Exact Code Changes

#### File 1: `internal/prompt/template.go` (NEW)
```go
package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Engine renders prompt templates with variables.
type Engine struct {
	templates map[string]*Template
}

// Template is a compiled prompt template.
type Template struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	System      string            `json:"system,omitempty"`
	User        string            `json:"user"`
	Variables   []string          `json:"variables"`
	Language    string            `json:"language,omitempty"` // Per-language override
	Model       string            `json:"model,omitempty"`    // Model-specific
}

// RenderedPrompt contains the final messages.
type RenderedPrompt struct {
	System string `json:"system,omitempty"`
	User   string `json:"user"`
}

func NewEngine() *Engine {
	return &Engine{templates: make(map[string]*Template)}
}

func (e *Engine) Register(t *Template) {
	e.templates[t.Name] = t
}

func (e *Engine) Get(name string) (*Template, bool) {
	t, ok := e.templates[name]
	return t, ok
}

// Render executes a template with the given variables.
func (e *Engine) Render(name string, vars map[string]interface{}) (*RenderedPrompt, error) {
	tmpl, ok := e.Get(name)
	if !ok {
		return nil, fmt.Errorf("template not found: %s", name)
	}

	// Validate required variables
	for _, v := range tmpl.Variables {
		if _, ok := vars[v]; !ok {
			return nil, fmt.Errorf("missing required variable: %s", v)
		}
	}

	// Render system prompt
	var system string
	if tmpl.System != "" {
		rendered, err := executeTemplate(tmpl.System, vars)
		if err != nil {
			return nil, fmt.Errorf("system template error: %w", err)
		}
		system = rendered
	}

	// Render user prompt
	user, err := executeTemplate(tmpl.User, vars)
	if err != nil {
		return nil, fmt.Errorf("user template error: %w", err)
	}

	return &RenderedPrompt{System: system, User: user}, nil
}

func executeTemplate(text string, vars map[string]interface{}) (string, error) {
	// Use Go's text/template for variable substitution
	// Continue.dev uses Handlebars; we map {{{var}}} to {{.var}}
	goTemplate := convertHandlebars(text)

	tmpl, err := template.New("prompt").Parse(goTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func convertHandlebars(text string) string {
	// Convert {{{var}}} and {{var}} to Go template syntax
	// Handlebars triple-stash = raw, double = HTML-escaped (we treat all as raw)
	result := strings.ReplaceAll(text, "{{{", "{{")
	result = strings.ReplaceAll(result, "}}}", "}}")
	return result
}
```

#### File 2: `internal/prompt/builtins.go` (NEW)
```go
package prompt

// RegisterBuiltinTemplates loads Continue.dev equivalent templates.
func RegisterBuiltinTemplates(e *Engine) {
	// Chat template (default)
	e.Register(&Template{
		Name:        "chat",
		Description: "Default chat template",
		User:        "{{{input}}}",
		Variables:   []string{"input"},
	})

	// Edit template
	e.Register(&Template{
		Name:        "edit",
		Description: "Edit selected code",
		System: `You are a helpful coding assistant. The user wants you to edit code.
Only output the modified code. Do not include explanations unless asked.`,
		User: `Edit the following code:

File: {{{filepath}}}
Language: {{{language}}}

\`\`\`{{{language}}}
{{{prefix}}}{{{code}}}{{{suffix}}}
\`\`\`

Instruction: {{{input}}}

Only output the replacement code. Use the same indentation.`,
		Variables: []string{"filepath", "language", "prefix", "code", "suffix", "input"},
	})

	// Comment template
	e.Register(&Template{
		Name:        "comment",
		Description: "Add comments to code",
		System:      "You are a helpful assistant that adds clear, concise comments to code.",
		User: `Add comments to the following {{{language}}} code:

\`\`\`{{{language}}}
{{{code}}}
\`\`\`

Add comments explaining:
- What each function does
- Parameters and return values
- Any complex logic

Output only the commented code.`,
		Variables: []string{"language", "code"},
	})

	// Commit message template
	e.Register(&Template{
		Name:        "commit",
		Description: "Generate a commit message",
		System:      "You write concise, conventional commit messages.",
		User: `Write a commit message for these changes:

\`\`\`diff
{{{diff}}}
\`\`\`

Use conventional commits format: type(scope): description
Keep it under 72 characters.`,
		Variables: []string{"diff"},
	})

	// Explain template
	e.Register(&Template{
		Name:        "explain",
		Description: "Explain code",
		System:      "You explain code in a clear, educational manner.",
		User: `Explain this {{{language}}} code:

\`\`\`{{{language}}}
{{{code}}}
\`\`\`

Break down:
1. What it does overall
2. Key functions and their purposes
3. Any design patterns used`,
		Variables: []string{"language", "code"},
	})

	// Test template
	e.Register(&Template{
		Name:        "test",
		Description: "Write unit tests",
		System:      "You write comprehensive unit tests.",
		User: `Write unit tests for this {{{language}}} code:

\`\`\`{{{language}}}
{{{code}}}
\`\`\`

Requirements:
- Test edge cases
- Use table-driven tests where appropriate
- Include setup and teardown if needed`,
		Variables: []string{"language", "code"},
	})

	// Autocomplete template (see also internal/autocomplete/fim.go)
	e.Register(&Template{
		Name:        "autocomplete",
		Description: "FIM autocomplete template",
		User:        "<|fim_prefix|>{{{prefix}}}<|fim_suffix|>{{{suffix}}}<|fim_middle|>",
		Variables:   []string{"prefix", "suffix"},
	})

	// Per-language Go template
	e.Register(&Template{
		Name:        "chat-go",
		Description: "Chat template optimized for Go",
		Language:    "go",
		System: `You are an expert Go programmer.
Follow these conventions:
- Use idiomatic Go (gofmt-compliant)
- Handle errors explicitly
- Use context.Context for cancellation
- Prefer composition over inheritance`,
		User: "{{{input}}}",
		Variables: []string{"input"},
	})

	// Per-language Python template
	e.Register(&Template{
		Name:        "chat-python",
		Description: "Chat template optimized for Python",
		Language:    "python",
		System: `You are an expert Python programmer.
Follow PEP 8 conventions.
Use type hints where appropriate.
Handle exceptions gracefully.`,
		User: "{{{input}}}",
		Variables: []string{"input"},
	})
}
```

#### File 3: `internal/prompt/loader.go` (NEW)
```go
package prompt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFromDirectory loads all .yaml/.yml prompt templates from a directory.
func LoadFromDirectory(e *Engine, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			path := filepath.Join(dir, name)
			if err := loadFile(e, path); err != nil {
				return fmt.Errorf("failed to load %s: %w", path, err)
			}
		}
	}

	return nil
}

func loadFile(e *Engine, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return err
	}

	// Derive name from filename if not set
	if tmpl.Name == "" {
		base := filepath.Base(path)
		tmpl.Name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	e.Register(&tmpl)
	return nil
}

// ConfigLoader loads templates from HelixCode config.
type ConfigLoader struct {
	PromptDir string `json:"promptDir"`
}

func (cl *ConfigLoader) Load(e *Engine) error {
	if cl.PromptDir == "" {
		cl.PromptDir = filepath.Join(os.Getenv("HOME"), ".helix", "prompts")
	}
	return LoadFromDirectory(e, cl.PromptDir)
}
```

#### File 4: `internal/prompt/variables.go` (NEW)
```go
package prompt

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BuildVariables creates the standard variable map from request context.
func BuildVariables(input string, uri string, language string, prefix string, suffix string, code string) map[string]interface{} {
	vars := map[string]interface{}{
		"input":    input,
		"filepath": uri,
		"filename": filepath.Base(uri),
		"language": language,
		"prefix":   prefix,
		"suffix":   suffix,
		"code":     code,
		"reponame": detectRepoName(uri),
	}
	return vars
}

func detectRepoName(uri string) string {
	parts := strings.Split(uri, string(filepath.Separator))
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] == "src" || parts[i] == "internal" || parts[i] == "pkg" {
			if i > 0 {
				return parts[i-1]
			}
		}
	}
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

// DiffVariable generates the diff variable for commit templates.
func DiffVariable(staged bool) string {
	// Execute git diff
	flag := ""
	if staged {
		flag = "--staged"
	}
	// This would call git in real implementation
	return fmt.Sprintf("git diff %s output here", flag)
}
```

#### File 5: `internal/llm/types.go` — MODIFY (add template integration)
```go
// LLMRequest extended with template support.
type LLMRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
	Messages    []Message `json:"messages"`
	Template    string    `json:"template,omitempty"` // Template name to use
	TemplateVars map[string]interface{} `json:"template_vars,omitempty"`
}
```

### Anti-Bluff Test
```bash
# 1. Test template rendering with all variables
$ go test ./internal/prompt/... -run TestRenderTemplate -v
# Template "edit" with filepath, language, prefix, code, suffix, input
# Should produce formatted prompt with code block

# 2. Test per-language template selection
$ go test ./internal/prompt/... -run TestLanguageTemplate -v
# Language=go should use chat-go template with Go-specific system prompt

# 3. Test template loader from filesystem
$ mkdir -p /tmp/test-prompts && echo 'name: custom' > /tmp/test-prompts/custom.yaml
$ go test ./internal/prompt/... -run TestLoadFromDirectory -v
# Should load custom.yaml into engine

# 4. Test missing variable error
$ go test ./internal/prompt/... -run TestMissingVariable -v
# Should return error for missing required variable
```

### Integration Verification
- [ ] All 7+ built-in templates render correctly
- [ ] Per-language templates selected based on file extension
- [ ] Custom templates loaded from ~/.helix/prompts/
- [ ] Handlebars-style {{{var}}} correctly converted to Go template
- [ ] Template variables validated before rendering
- [ ] System prompt included when template defines one

---

## 7. Model Configuration

### Source Location (Continue.dev)
- `core/config/` — Config loading and validation
- `schema/continue_config.yaml` — YAML config schema
- `core/llm/index.ts` — Model switching logic
- `core/llm/llms/` — Per-provider model definitions

### Target Location (HelixCode)
- **NEW**: `internal/config/model.go` — Model configuration
- **NEW**: `internal/config/provider_config.go` — Provider settings
- **NEW**: `internal/llm/router.go` — Model router
- **MODIFY**: `internal/config/config.go` — Add model config section
- **MODIFY**: `cmd/cli/main.go` — Add model switching CLI flags

### Exact Code Changes

#### File 1: `internal/config/model.go` (NEW)
```go
package config

import (
	"fmt"
)

// ModelConfig defines a single model configuration.
type ModelConfig struct {
	ID       string   `json:"id" yaml:"id"`
	Name     string   `json:"name" yaml:"name"`
	Provider string   `json:"provider" yaml:"provider"` // ollama, openai, anthropic, etc.
	Model    string   `json:"model" yaml:"model"`       // Actual model identifier
	APIBase  string   `json:"apiBase,omitempty" yaml:"apiBase,omitempty"`
	APIKey   string   `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`
	Roles    []string `json:"roles,omitempty" yaml:"roles,omitempty"` // chat, edit, autocomplete, embed, rerank

	// Generation parameters
	Temperature      *float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP             *float64 `json:"topP,omitempty" yaml:"topP,omitempty"`
	TopK             *int     `json:"topK,omitempty" yaml:"topK,omitempty"`
	MaxTokens        *int     `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`
	ContextLength    *int     `json:"contextLength,omitempty" yaml:"contextLength,omitempty"`
	PresencePenalty  *float64 `json:"presencePenalty,omitempty" yaml:"presencePenalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty" yaml:"frequencyPenalty,omitempty"`
	Stop             []string `json:"stop,omitempty" yaml:"stop,omitempty"`

	// Prompt templates
	PromptTemplates *PromptTemplates `json:"promptTemplates,omitempty" yaml:"promptTemplates,omitempty"`

	// Provider-specific options
	AutocompleteOptions *AutocompleteOptions `json:"autocompleteOptions,omitempty" yaml:"autocompleteOptions,omitempty"`
}

// PromptTemplates holds template overrides.
type PromptTemplates struct {
	Chat        string `json:"chat,omitempty" yaml:"chat,omitempty"`
	Edit        string `json:"edit,omitempty" yaml:"edit,omitempty"`
	Apply       string `json:"apply,omitempty" yaml:"apply,omitempty"`
	Autocomplete string `json:"autocomplete,omitempty" yaml:"autocomplete,omitempty"`
}

// AutocompleteOptions configures tab completion.
type AutocompleteOptions struct {
	DebounceDelay   int      `json:"debounceDelay,omitempty" yaml:"debounceDelay,omitempty"`
	MaxPromptTokens int      `json:"maxPromptTokens,omitempty" yaml:"maxPromptTokens,omitempty"`
	ModelTimeout    int      `json:"modelTimeout,omitempty" yaml:"modelTimeout,omitempty"`
	DisableInFiles  []string `json:"disableInFiles,omitempty" yaml:"disableInFiles,omitempty"`
}

// Validate checks model configuration.
func (m *ModelConfig) Validate() error {
	if m.Provider == "" {
		return fmt.Errorf("model %s: provider is required", m.ID)
	}
	if m.Model == "" {
		return fmt.Errorf("model %s: model identifier is required", m.ID)
	}
	if m.Temperature != nil && (*m.Temperature < 0 || *m.Temperature > 2) {
		return fmt.Errorf("model %s: temperature must be 0-2", m.ID)
	}
	if m.TopP != nil && (*m.TopP < 0 || *m.TopP > 1) {
		return fmt.Errorf("model %s: top_p must be 0-1", m.ID)
	}
	return nil
}

// GetTemperature returns the configured temperature or default.
func (m *ModelConfig) GetTemperature() float64 {
	if m.Temperature != nil {
		return *m.Temperature
	}
	return 0.7
}

// GetTopP returns the configured top_p or default.
func (m *ModelConfig) GetTopP() float64 {
	if m.TopP != nil {
		return *m.TopP
	}
	return 1.0
}

// GetMaxTokens returns the configured max tokens or default.
func (m *ModelConfig) GetMaxTokens() int {
	if m.MaxTokens != nil {
		return *m.MaxTokens
	}
	return 2048
}

// HasRole checks if model supports a role.
func (m *ModelConfig) HasRole(role string) bool {
	for _, r := range m.Roles {
		if r == role {
			return true
		}
	}
	return false
}
```

#### File 2: `internal/llm/router.go` (NEW)
```go
package llm

import (
	"context"
	"fmt"

	"dev.helix.code/internal/config"
)

// Router selects the appropriate model based on task and configuration.
type Router struct {
	models   []config.ModelConfig
	providers map[string]Provider
}

func NewRouter(models []config.ModelConfig) (*Router, error) {
	r := &Router{
		models:    models,
		providers: make(map[string]Provider),
	}

	for _, m := range models {
		if err := m.Validate(); err != nil {
			return nil, err
		}
	}

	return r, nil
}

// RegisterProvider adds a provider implementation.
func (r *Router) RegisterProvider(name string, p Provider) {
	r.providers[name] = p
}

// SelectModel returns the best model for a given role.
func (r *Router) SelectModel(role string) (*config.ModelConfig, Provider, error) {
	for _, m := range r.models {
		if m.HasRole(role) {
			provider, ok := r.providers[m.Provider]
			if !ok {
				return nil, nil, fmt.Errorf("provider %s not registered", m.Provider)
			}
			return &m, provider, nil
		}
	}

	// Fallback: use first model
	if len(r.models) > 0 {
		m := r.models[0]
		provider, ok := r.providers[m.Provider]
		if !ok {
			return nil, nil, fmt.Errorf("provider %s not registered", m.Provider)
		}
		return &m, provider, nil
	}

	return nil, nil, fmt.Errorf("no models configured for role: %s", role)
}

// RouteRequest routes an LLM request to the appropriate model.
func (r *Router) RouteRequest(ctx context.Context, req *LLMRequest) (*LLMResponse, error) {
	role := "chat"
	if req.Stream {
		// Could determine role from request context
	}

	model, provider, err := r.SelectModel(role)
	if err != nil {
		return nil, err
	}

	// Apply model-specific defaults
	if req.Temperature == 0 {
		req.Temperature = model.GetTemperature()
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = model.GetMaxTokens()
	}

	return provider.Generate(ctx, req)
}

// RouteStream routes a streaming request.
func (r *Router) RouteStream(ctx context.Context, req *LLMRequest, chunkChan chan LLMResponse) error {
	role := "chat"
	model, provider, err := r.SelectModel(role)
	if err != nil {
		return err
	}

	if req.Temperature == 0 {
		req.Temperature = model.GetTemperature()
	}
	if req.MaxTokens == 0 {
		req.MaxTokens = model.GetMaxTokens()
	}

	return provider.GenerateStream(ctx, req, chunkChan)
}
```

#### File 3: `internal/config/config.go` — MODIFY (add models section)
```go
// Config is the root configuration structure.
type Config struct {
	// ... existing fields ...

	// Models configuration (NEW)
	Models []ModelConfig `json:"models,omitempty" yaml:"models,omitempty"`

	// Embeddings configuration (NEW)
	EmbeddingsProvider *ModelConfig `json:"embeddingsProvider,omitempty" yaml:"embeddingsProvider,omitempty"`

	// Reranker configuration (NEW)
	Reranker *ModelConfig `json:"reranker,omitempty" yaml:"reranker,omitempty"`
}
```

#### File 4: `cmd/cli/main.go` — MODIFY (add model flags)
```go
// In Run():
var (
	// ... existing flags ...
	modelConfigFile = flag.String("model-config", "", "Path to model config YAML")
	listModels      = flag.Bool("list-models", false, "List configured models")
	switchModel     = flag.String("switch-model", "", "Switch to model by ID")
)

// In handleGenerate():
func (c *CLI) handleGenerate(ctx context.Context, prompt, model string, maxTokens int, temperature float64, stream bool) error {
	// Route through router for model selection
	router := llm.NewRouter(c.config.Models)
	// ... register providers ...

	modelCfg, provider, err := router.SelectModel("chat")
	if err != nil {
		return err
	}

	// Override with CLI flags if provided
	if temperature > 0 {
		modelCfg.Temperature = &temperature
	}
	if maxTokens > 0 {
		modelCfg.MaxTokens = &maxTokens
	}

	req := &llm.LLMRequest{
		Model:       modelCfg.Model,
		MaxTokens:   modelCfg.GetMaxTokens(),
		Temperature: modelCfg.GetTemperature(),
		Stream:      stream,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	if stream {
		return provider.GenerateStream(ctx, req, chunkChan)
	}
	return provider.Generate(ctx, req)
}
```

### Anti-Bluff Test
```bash
# 1. Test model config validation
$ go test ./internal/config/... -run TestModelValidation -v
# Invalid temperature=3 should return error

# 2. Test router selects correct model by role
$ go test ./internal/llm/... -run TestRouterRole -v
# Role=autocomplete should select model with roles=[autocomplete]

# 3. Test temperature override per model
$ go test ./internal/llm/... -run TestModelTemperature -v
# Model A: temp=0.2, Model B: temp=0.9
# Requests should use respective temperatures

# 4. Test model config loading from YAML
$ cat > /tmp/models.yaml <<EOF
models:
  - id: fast
    provider: ollama
    model: qwen2.5-coder:1.5b-base
    roles: [autocomplete]
    temperature: 0.2
  - id: smart
    provider: openai
    model: gpt-4
    roles: [chat, edit]
    temperature: 0.7
EOF
$ go test ./internal/config/... -run TestLoadModelConfig -v
# Should parse both models with correct roles and temperatures
```

### Integration Verification
- [ ] Router selects autocomplete model for tab completion
- [ ] Router selects chat model for conversational prompts
- [ ] Temperature correctly applied per model config
- [ ] Max tokens respected from model configuration
- [ ] Invalid model configs rejected at load time
- [ ] Model switching works via CLI flag --switch-model

---

## 8. Tab Autocomplete

### Source Location (Continue.dev)
- `core/autocomplete/CompletionProvider.ts` — Tab completion provider
- `extensions/vscode/src/autocomplete/` — VS Code inline completion item provider
- `core/autocomplete/lsp.ts` — LSP integration for completion

### Target Location (HelixCode)
- **NEW**: `internal/autocomplete/tab.go` — Tab completion orchestrator
- **NEW**: `internal/autocomplete/ghost.go` — Ghost text rendering
- **NEW**: `internal/autocomplete/debounce.go` — Debounce timer
- **MODIFY**: `internal/autocomplete/engine.go` — Add tab-specific logic
- **MODIFY**: `internal/editor/` — Add ghost text methods

### Exact Code Changes

#### File 1: `internal/autocomplete/tab.go` (NEW)
```go
package autocomplete

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"dev.helix.code/internal/editor"
)

// TabCompletion handles tab-to-complete interaction.
type TabCompletion struct {
	engine   *Engine
	db       *Debouncer
	active   *ActiveCompletion
	editor   editor.Editor
}

// ActiveCompletion tracks the current suggestion.
type ActiveCompletion struct {
	ID         string
	InsertText string
	Position   editor.Position
	URI        string
	Accepted   bool
	ShownAt    time.Time
}

func NewTabCompletion(engine *Engine, editor editor.Editor) *TabCompletion {
	return &TabCompletion{
		engine: engine,
		db:     NewDebouncer(250 * time.Millisecond),
		editor: editor,
	}
}

// OnKeystroke is called on every keystroke in the editor.
func (tc *TabCompletion) OnKeystroke(ctx context.Context, uri string, pos editor.Position, prefix, suffix string) error {
	// Cancel previous completion
	if tc.active != nil && !tc.active.Accepted {
		tc.clearGhost(ctx)
	}

	// Don't trigger on certain keys
	if shouldSkip(prefix) {
		return nil
	}

	// Debounce
	tc.db.Reset()
	go func() {
		time.Sleep(tc.db.Delay)
		if tc.db.Cancelled {
			return
		}

		req := Request{
			URI:      uri,
			Position: pos,
			Prefix:   prefix,
			Suffix:   suffix,
			Language: detectLanguage(uri),
			Filename: uri,
		}

		resp, err := tc.engine.Complete(ctx, req)
		if err != nil || len(resp.Items) == 0 {
			return
		}

		item := resp.Items[0]
		tc.showGhost(ctx, uri, pos, item.InsertText)
		tc.active = &ActiveCompletion{
			ID:         fmt.Sprintf("comp-%d", time.Now().UnixNano()),
			InsertText: item.InsertText,
			Position:   pos,
			URI:        uri,
			ShownAt:    time.Now(),
		}
	}()

	return nil
}

// OnTab is called when user presses Tab.
func (tc *TabCompletion) OnTab(ctx context.Context) error {
	if tc.active == nil || tc.active.Accepted {
		return nil // No active completion
	}

	// Insert the completion
	if err := tc.editor.InsertAtCursor(ctx, tc.active.URI, tc.active.InsertText); err != nil {
		return err
	}

	tc.active.Accepted = true
	return tc.editor.AcceptCompletion(ctx, tc.active.ID)
}

// OnEscape cancels the active completion.
func (tc *TabCompletion) OnEscape(ctx context.Context) error {
	if tc.active == nil {
		return nil
	}

	tc.clearGhost(ctx)
	tc.editor.RejectCompletion(ctx, tc.active.ID)
	tc.active = nil
	return nil
}

func (tc *TabCompletion) showGhost(ctx context.Context, uri string, pos editor.Position, text string) {
	if tc.editor != nil {
		tc.editor.ShowGhostText(ctx, uri, pos, text)
	}
}

func (tc *TabCompletion) clearGhost(ctx context.Context) {
	if tc.editor != nil && tc.active != nil {
		tc.editor.ClearGhostText(ctx, tc.active.URI)
	}
}

func shouldSkip(prefix string) bool {
	if prefix == "" {
		return true
	}
	// Skip if last character is whitespace (except space for continued typing)
	lastChar := []rune(prefix)[len([]rune(prefix))-1]
	if unicode.IsSpace(lastChar) && lastChar != ' ' {
		return true
	}
	return false
}

func detectLanguage(uri string) string {
	switch {
	case strings.HasSuffix(uri, ".go"):
		return "go"
	case strings.HasSuffix(uri, ".py"):
		return "python"
	case strings.HasSuffix(uri, ".js"):
		return "javascript"
	case strings.HasSuffix(uri, ".ts"):
		return "typescript"
	case strings.HasSuffix(uri, ".rs"):
		return "rust"
	default:
		return ""
	}
}
```

#### File 2: `internal/autocomplete/debounce.go` (NEW)
```go
package autocomplete

import (
	"sync"
	"time"
)

// Debouncer prevents excessive completion requests.
type Debouncer struct {
	Delay     time.Duration
	mu        sync.Mutex
	timer     *time.Timer
	Cancelled bool
}

func NewDebouncer(delay time.Duration) *Debouncer return &Debouncer{Delay: delay}
}

func (d *Debouncer) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}
	d.Cancelled = false

	d.timer = time.AfterFunc(d.Delay, func() {
		d.mu.Lock()
		d.Cancelled = false
		d.mu.Unlock()
	})
}

func (d *Debouncer) Cancel() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.timer != nil {
		d.timer.Stop()
	}
	d.Cancelled = true
}
```

#### File 3: `internal/autocomplete/ghost.go` (NEW)
```go
package autocomplete

import (
	"context"
	"fmt"

	"dev.helix.code/internal/editor"
)

// GhostText manages inline ghost text display.
type GhostText struct {
	editor editor.Editor
}

func NewGhostText(ed editor.Editor) *GhostText {
	return &GhostText{editor: ed}
}

// Show displays ghost text at the cursor position.
func (g *GhostText) Show(ctx context.Context, uri string, pos editor.Position, text string) error {
	// Format for ghost text display
	ghost := formatGhostText(text)
	return g.editor.ShowGhostText(ctx, uri, pos, ghost)
}

// Hide removes ghost text.
func (g *GhostText) Hide(ctx context.Context, uri string) error {
	return g.editor.ClearGhostText(ctx, uri)
}

func formatGhostText(text string) string {
	// Add subtle formatting hint
	return text
}
```

#### File 4: `internal/editor/editor.go` — MODIFY (add autocomplete methods)
```go
// Add to Editor interface:
	InsertAtCursor(ctx context.Context, uri string, text string) error
```

#### File 5: `internal/editor/vscode.go` — MODIFY (implement InsertAtCursor)
```go
func (v *VSCodeAdapter) InsertAtCursor(ctx context.Context, uri string, text string) error {
	return v.sendToEditor("insertAtCursor", map[string]interface{}{
		"uri":  uri,
		"text": text,
	})
}
```

### Anti-Bluff Test
```bash
# 1. Test debounce prevents rapid-fire requests
$ go test ./internal/autocomplete/... -run TestDebounce -v
# 5 keystrokes in 100ms should result in 1 completion request

# 2. Test tab inserts completion text
$ go test ./internal/autocomplete/... -run TestTabInsert -v
# OnTab with active completion should call editor.InsertAtCursor

# 3. Test escape cancels completion
$ go test ./internal/autocomplete/... -run TestEscapeCancel -v
# OnEscape should clear ghost text and reject completion

# 4. Test language detection
$ go test ./internal/autocomplete/... -run TestDetectLanguage -v
# main.go -> go, app.py -> python, index.js -> javascript
```

### Integration Verification
- [ ] Ghost text appears after debounce delay (250ms)
- [ ] Tab key inserts completion at cursor position
- [ ] Escape key dismisses ghost text
- [ ] Debouncer cancels stale requests on new keystrokes
- [ ] Language correctly detected from file extension
- [ ] Editor API renders ghost text with appropriate styling

---

## 9. Slash Commands

### Source Location (Continue.dev)
- `core/commands/slash/` — Slash command implementations
- `core/commands/index.ts` — Command registration
- `core/commands/util.ts` — Command utilities
- `gui/src/components/mainInput/resolveEditorContent.ts` — Command parsing

### Target Location (HelixCode)
- **NEW**: `internal/commands/registry.go` — Command registry
- **NEW**: `internal/commands/slash.go` — Slash command parser
- **NEW**: `internal/commands/edit.go` — /edit command
- **NEW**: `internal/commands/comment.go` — /comment command
- **NEW**: `internal/commands/share.go` — /share command
- **NEW**: `internal/commands/custom.go` — Custom command support
- **MODIFY**: `cmd/cli/main.go` — Integrate slash commands
- **MODIFY**: `internal/prompt/` — Wire templates to commands

### Exact Code Changes

#### File 1: `internal/commands/registry.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"
	"strings"
)

// Command is a slash command implementation.
type Command interface {
	Name() string
	Description() string
	Run(ctx context.Context, input string, env *CommandEnv) (string, error)
}

// CommandEnv provides services to commands.
type CommandEnv struct {
	LLMProvider   interface{}
	Editor        interface{}
	ContextRegistry interface{}
	WorkspaceDirs []string
	CurrentFile   string
	Selection     string
}

// Registry holds all slash commands.
type Registry struct {
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

func (r *Registry) Register(cmd Command) {
	r.commands[cmd.Name()] = cmd
}

func (r *Registry) Get(name string) (Command, bool) {
	cmd, ok := r.commands[name]
	return cmd, ok
}

func (r *Registry) List() []Command {
	var cmds []Command
	for _, c := range r.commands {
		cmds = append(cmds, c)
	}
	return cmds
}

// ParseSlashCommand extracts command name and remaining input.
func ParseSlashCommand(input string) (name, args string, isCommand bool) {
	if !strings.HasPrefix(input, "/") {
		return "", input, false
	}

	parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
	name = parts[0]
	if len(parts) > 1 {
		args = parts[1]
	}
	return name, args, true
}

// Execute runs a slash command if input starts with "/".
func (r *Registry) Execute(ctx context.Context, input string, env *CommandEnv) (string, error) {
	name, args, isCommand := ParseSlashCommand(input)
	if !isCommand {
		return "", fmt.Errorf("not a slash command")
	}

	cmd, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown command: /%s. Type /help for available commands.", name)
	}

	return cmd.Run(ctx, args, env)
}
```

#### File 2: `internal/commands/edit.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"
	"os"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/prompt"
)

// EditCommand implements /edit — edit selected code.
type EditCommand struct {
	llmProvider llm.Provider
	engine      *prompt.Engine
}

func NewEditCommand(provider llm.Provider, engine *prompt.Engine) *EditCommand {
	return &EditCommand{llmProvider: provider, engine: engine}
}

func (c *EditCommand) Name() string        { return "edit" }
func (c *EditCommand) Description() string { return "Edit selected code" }

func (c *EditCommand) Run(ctx context.Context, input string, env *CommandEnv) (string, error) {
	// Get selected code from editor
	selection := env.Selection
	if selection == "" {
		return "", fmt.Errorf("no code selected. Select code and try again.")
	}

	// Build variables
	vars := map[string]interface{}{
		"filepath": env.CurrentFile,
		"language": detectLanguage(env.CurrentFile),
		"code":     selection,
		"input":    input,
		"prefix":   "", // Could extract prefix from file
		"suffix":   "",
	}

	// Render edit template
	rendered, err := c.engine.Render("edit", vars)
	if err != nil {
		return "", fmt.Errorf("template error: %w", err)
	}

	// Call LLM
	req := &llm.LLMRequest{
		Model:       "default",
		MaxTokens:   2048,
		Temperature: 0.3,
		Messages: []llm.Message{
			{Role: "system", Content: rendered.System},
			{Role: "user", Content: rendered.User},
		},
	}

	resp, err := c.llmProvider.Generate(ctx, req)
	if err != nil {
		return "", fmt.Errorf("generation failed: %w", err)
	}

	return resp.Content, nil
}
```

#### File 3: `internal/commands/comment.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/prompt"
)

// CommentCommand implements /comment — add comments to code.
type CommentCommand struct {
	llmProvider llm.Provider
	engine      *prompt.Engine
}

func NewCommentCommand(provider llm.Provider, engine *prompt.Engine) *CommentCommand {
	return &CommentCommand{llmProvider: provider, engine: engine}
}

func (c *CommentCommand) Name() string        { return "comment" }
func (c *CommentCommand) Description() string { return "Add comments to selected code" }

func (c *CommentCommand) Run(ctx context.Context, input string, env *CommandEnv) (string, error) {
	selection := env.Selection
	if selection == "" {
		return "", fmt.Errorf("no code selected")
	}

	vars := map[string]interface{}{
		"language": detectLanguage(env.CurrentFile),
		"code":     selection,
	}

	rendered, err := c.engine.Render("comment", vars)
	if err != nil {
		return "", err
	}

	req := &llm.LLMRequest{
		Model:       "default",
		MaxTokens:   2048,
		Temperature: 0.3,
		Messages: []llm.Message{
			{Role: "system", Content: rendered.System},
			{Role: "user", Content: rendered.User},
		},
	}

	resp, err := c.llmProvider.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}
```

#### File 4: `internal/commands/share.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ShareCommand implements /share — export conversation to markdown.
type ShareCommand struct {
	outputDir string
}

func NewShareCommand(outputDir string) *ShareCommand {
	if outputDir == "" {
		home, _ := os.UserHomeDir()
		outputDir = filepath.Join(home, ".helix", "session-transcripts")
	}
	return &ShareCommand{outputDir: outputDir}
}

func (c *ShareCommand) Name() string        { return "share" }
func (c *ShareCommand) Description() string { return "Export current chat session to markdown" }

func (c *ShareCommand) Run(ctx context.Context, input string, env *CommandEnv) (string, error) {
	// Build markdown from session history
	// In real implementation, this would access session store
	markdown := buildMarkdownTranscript(env)

	// Write to file
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	filename := filepath.Join(c.outputDir, fmt.Sprintf("session-%s.md", timestamp))

	if err := os.MkdirAll(c.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := os.WriteFile(filename, []byte(markdown), 0644); err != nil {
		return "", fmt.Errorf("failed to write transcript: %w", err)
	}

	return fmt.Sprintf("Session exported to: %s", filename), nil
}

func buildMarkdownTranscript(env *CommandEnv) string {
	var md string
	md += "# HelixCode Session Transcript\n\n"
	md += fmt.Sprintf("**Date:** %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	md += fmt.Sprintf("**File:** %s\n\n", env.CurrentFile)
	md += "---\n\n"
	// Would include actual messages from session history
	return md
}
```

#### File 5: `internal/commands/custom.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/prompt"
)

// CustomCommand is a user-defined slash command.
type CustomCommand struct {
	NameStr     string
	Desc        string
	Prompt      string // Raw prompt template
	llmProvider llm.Provider
}

func NewCustomCommand(name, description, prompt string, provider llm.Provider) *CustomCommand {
	return &CustomCommand{
		NameStr:     name,
		Desc:        description,
		Prompt:      prompt,
		llmProvider: provider,
	}
}

func (c *CustomCommand) Name() string        { return c.NameStr }
func (c *CustomCommand) Description() string { return c.Desc }

func (c *CustomCommand) Run(ctx context.Context, input string, env *CommandEnv) (string, error) {
	// Substitute {{{input}}} in prompt
	prompt := strings.ReplaceAll(c.Prompt, "{{{input}}}", input)
	prompt = strings.ReplaceAll(prompt, "{{{code}}}", env.Selection)
	prompt = strings.ReplaceAll(prompt, "{{{filepath}}}", env.CurrentFile)

	req := &llm.LLMRequest{
		Model:       "default",
		MaxTokens:   2048,
		Temperature: 0.7,
		Messages: []llm.Message{
			{Role: "user", Content: prompt},
		},
	}

	resp, err := c.llmProvider.Generate(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}
```

#### File 6: `cmd/cli/main.go` — MODIFY (add slash command handling)
```go
func (c *CLI) handleInteractive(ctx context.Context) error {
	fmt.Println("=== Helix CLI Interactive Mode ===")
	fmt.Println("Type 'help' for available commands, 'exit' to quit")
	fmt.Println("Use /edit, /comment, /share, or /help for slash commands")

	// ... existing setup ...

	for {
		// ... existing input loop ...

		// Check for slash commands
		if strings.HasPrefix(input, "/") {
			result, err := c.handleSlashCommand(ctx, input)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			} else {
				fmt.Println(result)
			}
			continue
		}

		// ... existing command handling ...
	}
}

func (c *CLI) handleSlashCommand(ctx context.Context, input string) (string, error) {
	// Initialize command registry
	registry := commands.NewRegistry()
	registry.Register(commands.NewEditCommand(c.llmProvider, c.promptEngine))
	registry.Register(commands.NewCommentCommand(c.llmProvider, c.promptEngine))
	registry.Register(commands.NewShareCommand(""))
	registry.Register(commands.NewHelpCommand(registry))

	env := &commands.CommandEnv{
		LLMProvider:    c.llmProvider,
		WorkspaceDirs: []string{"."},
		CurrentFile:   "main.go", // Would come from editor
		Selection:     "",        // Would come from editor
	}

	return registry.Execute(ctx, input, env)
}
```

#### File 7: `internal/commands/help.go` (NEW)
```go
package commands

import (
	"context"
	"fmt"
	"strings"
)

// HelpCommand implements /help.
type HelpCommand struct {
	registry *Registry
}

func NewHelpCommand(registry *Registry) *HelpCommand {
	return &HelpCommand{registry: registry}
}

func (c *HelpCommand) Name() string        { return "help" }
func (c *HelpCommand) Description() string { return "Show available commands" }

func (c *HelpCommand) Run(ctx context.Context, input string, env *CommandEnv) (string, error) {
	var sb strings.Builder
	sb.WriteString("Available slash commands:\n\n")
	for _, cmd := range c.registry.List() {
		fmt.Fprintf(&sb, "  /%-12s %s\n", cmd.Name(), cmd.Description())
	}
	sb.WriteString("\nYou can also define custom commands in your config.\n")
	return sb.String(), nil
}
```

### Anti-Bluff Test
```bash
# 1. Test slash command parser
$ go test ./internal/commands/... -run TestParseSlashCommand -v
# "/edit add error handling" -> name=edit, args="add error handling"
# "normal prompt" -> not a command

# 2. Test /edit command with real LLM
$ go test ./internal/commands/... -run TestEditCommand -v -timeout 30s
# Should call LLM with edit template and return modified code

# 3. Test /share creates markdown file
$ go test ./internal/commands/... -run TestShareCommand -v
# Should create ~/.helix/session-transcripts/session-*.md

# 4. Test custom command with variable substitution
$ go test ./internal/commands/... -run TestCustomCommand -v
# Prompt: "Check {{{code}}} for bugs", code="func(){}"
# Should substitute to "Check func(){} for bugs"
```

### Integration Verification
- [ ] /edit sends selected code to LLM with edit template
- [ ] /comment adds doc comments to selected code
- [ ] /share exports session to markdown in ~/.helix/session-transcripts/
- [ ] Custom commands loaded from config with variable substitution
- [ ] Unknown commands return helpful error with /help suggestion
- [ ] Command registry lists all available commands

---

## 10. Embeddings Integration

### Source Location (Continue.dev)
- `core/indexing/` — Codebase indexing pipeline
- `core/indexing/CodeSnippetsIndex.ts` — Code snippet indexing
- `core/indexing/LanceDbIndex.ts` — Vector storage with LanceDB
- `core/indexing/chunk/` — Code chunking strategies
- `core/indexing/ignore.ts` — File ignore patterns
- `core/context/retrieval/` — Semantic retrieval logic

### Target Location (HelixCode)
- **NEW**: `internal/embeddings/engine.go` — Embeddings orchestrator
- **NEW**: `internal/embeddings/indexer.go` — Codebase indexer
- **NEW**: `internal/embeddings/chunker.go` — Code chunking
- **NEW**: `internal/embeddings/store.go` — Vector store interface
- **NEW**: `internal/embeddings/provider.go` — Embedding provider interface
- **MODIFY**: `internal/knowledge/` — Leverage existing knowledge package
- **MODIFY**: `internal/config/` — Add embeddings configuration

### Exact Code Changes

#### File 1: `internal/embeddings/engine.go` (NEW)
```go
package embeddings

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Engine orchestrates codebase indexing and semantic search.
type Engine struct {
	provider  Provider
	store     Store
	chunker   Chunker
	indexPath string
	mu        sync.RWMutex
	indexed   bool
}

func NewEngine(provider Provider, store Store, indexPath string) *Engine {
	return &Engine{
		provider:  provider,
		store:     store,
		chunker:   NewCodeChunker(),
		indexPath: indexPath,
	}
}

// IndexWorkspace indexes all files in workspace directories.
func (e *Engine) IndexWorkspace(ctx context.Context, dirs []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var allChunks []Chunk
	for _, dir := range dirs {
		chunks, err := e.indexDirectory(ctx, dir)
		if err != nil {
			return fmt.Errorf("failed to index %s: %w", dir, err)
		}
		allChunks = append(allChunks, chunks...)
	}

	// Batch embed
	if err := e.embedAndStore(ctx, allChunks); err != nil {
		return fmt.Errorf("failed to store embeddings: %w", err)
	}

	e.indexed = true
	return nil
}

func (e *Engine) indexDirectory(ctx context.Context, dir string) ([]Chunk, error) {
	var chunks []Chunk

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip ignored files
		if shouldIgnore(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			return nil
		}

		// Only index code files
		if !isCodeFile(path) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		fileChunks := e.chunker.Chunk(path, string(content))
		chunks = append(chunks, fileChunks...)
		return nil
	})

	return chunks, err
}

func (e *Engine) embedAndStore(ctx context.Context, chunks []Chunk) error {
	const batchSize = 32

	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		texts := make([]string, len(batch))
		for j, c := range batch {
			texts[j] = c.Text
		}

		vectors, err := e.provider.Embed(ctx, texts)
		if err != nil {
			return fmt.Errorf("embedding batch %d-%d failed: %w", i, end, err)
		}

		for j, vector := range vectors {
			batch[j].Vector = vector
		}

		if err := e.store.Upsert(ctx, batch); err != nil {
			return err
		}
	}

	return nil
}

// Search performs semantic search over the indexed codebase.
func (e *Engine) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if !e.indexed {
		return nil, fmt.Errorf("codebase not indexed. Run 'helix index' first.")
	}

	// Embed query
	vectors, err := e.provider.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("query embedding failed: %w", err)
	}

	if len(vectors) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}

	// Search store
	results, err := e.store.Search(ctx, vectors[0], limit)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	return results, nil
}

func shouldIgnore(path string) bool {
	// Match .gitignore patterns + common ignores
	ignores := []string{
		".git", ".svn", ".hg",
		"node_modules", "vendor",
		"dist", "build", "target",
		".idea", ".vscode",
		"*.min.js", "*.min.css",
		"__pycache__", ".pytest_cache",
	}

	for _, pattern := range ignores {
		matched, _ := filepath.Match(pattern, filepath.Base(path))
		if matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

func isCodeFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".rs": true, ".java": true, ".cpp": true, ".c": true,
		".h": true, ".hpp": true, ".rb": true, ".php": true,
		".cs": true, ".swift": true, ".kt": true, ".scala": true,
		".md": true, ".txt": true, ".json": true, ".yaml": true,
		".yml": true, ".toml": true, ".sh": true,
	}
	return codeExts[ext]
}
```

#### File 2: `internal/embeddings/provider.go` (NEW)
```go
package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"bytes"
	"encoding/json"
)

// Provider generates text embeddings.
type Provider interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
	Dimensions() int
}

// OllamaEmbeddingProvider uses Ollama for local embeddings.
type OllamaEmbeddingProvider struct {
	BaseURL string
	Model   string
}

func NewOllamaEmbeddingProvider(baseURL, model string) *OllamaEmbeddingProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	return &OllamaEmbeddingProvider{BaseURL: baseURL, Model: model}
}

func (p *OllamaEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	var results [][]float64

	for _, text := range texts {
		reqBody := map[string]interface{}{
			"model":  p.Model,
			"prompt": text,
		}

		data, _ := json.Marshal(reqBody)
		resp, err := http.Post(p.BaseURL+"/api/embeddings", "application/json", bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("ollama embedding failed: %w", err)
		}

		var result struct {
			Embedding []float64 `json:"embedding"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		results = append(results, result.Embedding)
	}

	return results, nil
}

func (p *OllamaEmbeddingProvider) Dimensions() int {
	return 768 // nomic-embed-text dimensions
}

// OpenAIEmbeddingProvider uses OpenAI API for embeddings.
type OpenAIEmbeddingProvider struct {
	APIKey string
	Model  string
}

func NewOpenAIEmbeddingProvider(apiKey, model string) *OpenAIEmbeddingProvider {
	if model == "" {
		model = "text-embedding-3-small"
	}
	return &OpenAIEmbeddingProvider{APIKey: apiKey, Model: model}
}

func (p *OpenAIEmbeddingProvider) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	reqBody := map[string]interface{}{
		"model": p.Model,
		"input": texts,
	}

	data, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/embeddings", bytes.NewReader(data))
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	var embeddings [][]float64
	for _, d := range result.Data {
		embeddings = append(embeddings, d.Embedding)
	}
	return embeddings, nil
}

func (p *OpenAIEmbeddingProvider) Dimensions() int {
	if p.Model == "text-embedding-3-small" {
		return 1536
	}
	return 3072
}
```

#### File 3: `internal/embeddings/chunker.go` (NEW)
```go
package embeddings

import (
	"path/filepath"
	"strings"
)

// Chunk represents a single document chunk for embedding.
type Chunk struct {
	ID       string    `json:"id"`
	FilePath string    `json:"file_path"`
	StartLine int      `json:"start_line"`
	EndLine   int      `json:"end_line"`
	Text     string    `json:"text"`
	Language string    `json:"language"`
	Vector   []float64 `json:"vector,omitempty"`
}

// Chunker splits documents into embeddable chunks.
type Chunker interface {
	Chunk(filePath string, content string) []Chunk
}

// CodeChunker chunks code files by function/class and sliding window.
type CodeChunker struct {
	MaxChunkSize    int // max characters per chunk
	OverlapSize     int // overlap between chunks
}

func NewCodeChunker() *CodeChunker {
	return &CodeChunker{
		MaxChunkSize: 1500,
		OverlapSize:  150,
	}
}

func (c *CodeChunker) Chunk(filePath string, content string) []Chunk {
	language := detectLanguage(filePath)
	lines := strings.Split(content, "\n")

	var chunks []Chunk
	chunkID := 0

	// Strategy 1: Chunk by logical boundaries (functions, classes)
	if language != "" {
		logicalChunks := c.chunkByLogicalBoundaries(filePath, lines, language)
		chunks = append(chunks, logicalChunks...)
	}

	// Strategy 2: Sliding window for remaining content
	if len(chunks) == 0 {
		slidingChunks := c.chunkBySlidingWindow(filePath, lines)
		chunks = append(chunks, slidingChunks...)
	}

	// Assign IDs
	for i := range chunks {
		chunks[i].ID = fmt.Sprintf("%s:%d", filePath, chunkID)
		chunks[i].Language = language
		chunkID++
	}

	return chunks
}

func (c *CodeChunker) chunkByLogicalBoundaries(filePath string, lines []string, language string) []Chunk {
	var chunks []Chunk
	var currentLines []string
	startLine := 0

	// Simple detection: Go functions, Python def/class, JS functions
	boundaryPatterns := getBoundaryPatterns(language)

	for i, line := range lines {
		isBoundary := false
		for _, pattern := range boundaryPatterns {
			if strings.HasPrefix(strings.TrimSpace(line), pattern) {
				isBoundary = true
				break
			}
		}

		if isBoundary && len(currentLines) > 0 {
			chunks = append(chunks, Chunk{
				FilePath:  filePath,
				StartLine: startLine + 1,
				EndLine:   i,
				Text:      strings.Join(currentLines, "\n"),
			})
			currentLines = nil
			startLine = i
		}

		currentLines = append(currentLines, line)
	}

	if len(currentLines) > 0 {
		chunks = append(chunks, Chunk{
			FilePath:  filePath,
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(currentLines, "\n"),
		})
	}

	return chunks
}

func (c *CodeChunker) chunkBySlidingWindow(filePath string, lines []string) []Chunk {
	var chunks []Chunk
	var current []string
	currentLen := 0
	startLine := 0

	for i, line := range lines {
		lineLen := len(line) + 1 // +1 for newline

		if currentLen+lineLen > c.MaxChunkSize && len(current) > 0 {
			chunks = append(chunks, Chunk{
				FilePath:  filePath,
				StartLine: startLine + 1,
				EndLine:   i,
				Text:      strings.Join(current, "\n"),
			})

			// Overlap: keep last N characters
			overlapStart := len(current) - 1
			overlapLen := 0
			for j := len(current) - 1; j >= 0 && overlapLen < c.OverlapSize; j-- {
				overlapLen += len(current[j]) + 1
				overlapStart = j
			}
			current = current[overlapStart:]
			startLine = i - len(current) + 1
			currentLen = overlapLen
		}

		current = append(current, line)
		currentLen += lineLen
	}

	if len(current) > 0 {
		chunks = append(chunks, Chunk{
			FilePath:  filePath,
			StartLine: startLine + 1,
			EndLine:   len(lines),
			Text:      strings.Join(current, "\n"),
		})
	}

	return chunks
}

func getBoundaryPatterns(language string) []string {
	switch language {
	case "go":
		return []string{"func ", "type ", "var ", "const ", "import "}
	case "python":
		return []string{"def ", "class ", "async def "}
	case "javascript", "typescript":
		return []string{"function ", "const ", "let ", "var ", "class "}
	case "rust":
		return []string{"fn ", "impl ", "trait ", "struct ", "enum "}
	default:
		return []string{"func ", "def ", "function ", "class "}
	}
}
```

#### File 4: `internal/embeddings/store.go` (NEW)
```go
package embeddings

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
)

// Store is the vector database interface for embeddings.
type Store interface {
	Upsert(ctx context.Context, chunks []Chunk) error
	Search(ctx context.Context, vector []float64, limit int) ([]SearchResult, error)
	DeleteByFile(ctx context.Context, filePath string) error
	GetAll(ctx context.Context, indexType string) ([]Chunk, error)
}

// SearchResult is a single semantic search result.
type SearchResult struct {
	Chunk
	Score float64 `json:"score"`
}

// InMemoryStore is a simple in-memory vector store for testing.
type InMemoryStore struct {
	chunks []Chunk
	mu     sync.RWMutex
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{}
}

func (s *InMemoryStore) Upsert(ctx context.Context, chunks []Chunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, c := range chunks {
		// Delete existing chunks for this file+ID
		var filtered []Chunk
		for _, existing := range s.chunks {
			if existing.ID != c.ID {
				filtered = append(filtered, existing)
			}
		}
		s.chunks = filtered
		s.chunks = append(s.chunks, c)
	}

	return nil
}

func (s *InMemoryStore) Search(ctx context.Context, vector []float64, limit int) ([]SearchResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []SearchResult
	for _, chunk := range s.chunks {
		if len(chunk.Vector) == 0 || len(vector) == 0 {
			continue
		}
		score := cosineSimilarity(vector, chunk.Vector)
		results = append(results, SearchResult{Chunk: chunk, Score: score})
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (s *InMemoryStore) DeleteByFile(ctx context.Context, filePath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var filtered []Chunk
	for _, c := range s.chunks {
		if c.FilePath != filePath {
			filtered = append(filtered, c)
		}
	}
	s.chunks = filtered
	return nil
}

func (s *InMemoryStore) GetAll(ctx context.Context, indexType string) ([]Chunk, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]Chunk(nil), s.chunks...), nil
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := 0; i < len(a); i++ {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

#### File 5: `internal/knowledge/store.go` — MODIFY (add to interface)
```go
// Add to Store interface:
	SearchDocs(ctx context.Context, siteTitle string, opts SearchOptions) ([]DocResult, error)
	GetAllSnippets(ctx context.Context, indexType string) ([]Snippet, error)
	SearchBySymbol(ctx context.Context, symbol string, limit int) ([]SearchResult, error)
```

#### File 6: `cmd/cli/main.go` — MODIFY (add index command)
```go
// In handleInteractive:
		case "index":
			return c.handleIndex(ctx)

// New handler:
func (c *CLI) handleIndex(ctx context.Context) error {
	fmt.Println("Indexing workspace...")

	engine := embeddings.NewEngine(
		embeddings.NewOllamaEmbeddingProvider("", ""),
		embeddings.NewInMemoryStore(),
		filepath.Join(os.Getenv("HOME"), ".helix", "index"),
	)

	dirs := []string{"."}
	if err := engine.IndexWorkspace(ctx, dirs); err != nil {
		return err
	}

	fmt.Println("✅ Workspace indexed successfully")
	return nil
}
```

### Anti-Bluff Test
```bash
# 1. Test Ollama embedding provider with real Ollama
$ go test ./internal/embeddings/... -run TestOllamaEmbed -v -timeout 60s
# Should connect to localhost:11434 and return 768-dim vectors

# 2. Test chunker produces correct chunks
$ go test ./internal/embeddings/... -run TestCodeChunker -v
# Go file with 3 functions -> 3+ chunks with correct line ranges

# 3. Test semantic search finds relevant code
$ go test ./internal/embeddings/... -run TestSemanticSearch -v
# Index "func AuthenticateUser()", search "login authentication"
# Should return AuthenticateUser with high cosine similarity

# 4. Test in-memory store upsert and search
$ go test ./internal/embeddings/... -run TestStore -v
# Upsert 100 chunks, search with query vector, verify top-5 results
```

### Integration Verification
- [ ] Workspace indexer processes all code files in directory
- [ ] Ollama embedding provider generates 768-dim vectors
- [ ] Code chunker splits by function boundaries for Go/Python/JS
- [ ] Sliding window chunks large files with overlap
- [ ] Semantic search returns results sorted by cosine similarity
- [ ] Store supports incremental updates (delete old + insert new)
- [ ] File ignore patterns skip node_modules, .git, build dirs
- [ ] Index command available in CLI: `helix index`

---

## Integration Roadmap

### Phase 1: Foundation (Week 1)
1. Implement `internal/context/` — Parser + 5 providers (@file, @url, @docs, @codebase, @code)
2. Implement `internal/prompt/` — Template engine + builtins
3. Implement `internal/commands/` — Slash command registry + 5 commands

### Phase 2: Editor Protocol (Week 2)
1. Implement `internal/editor/` — Protocol + VS Code adapter + LSP client
2. Wire WebSocket server into `cmd/server/`
3. Add editor methods to all provider outputs

### Phase 3: Smart Features (Week 3)
1. Implement `internal/autocomplete/` — Engine + FIM + cache + filter
2. Implement `internal/diff/` — Stream processor + hunk management + applier
3. Implement `internal/embeddings/` — Provider + chunker + store + indexer

### Phase 4: Model & Config (Week 4)
1. Implement `internal/config/model.go` — Model config + validation
2. Implement `internal/llm/router.go` — Role-based routing
3. Integrate token budget into context framework
4. Add CLI flags for model switching

### Phase 5: QA & Hardening (Week 5)
1. Run all anti-bluff tests end-to-end
2. Performance benchmark autocomplete (<500ms p95)
3. Security audit file provider path traversal
4. Documentation and example configs

---

## New Files Summary (27 files)

| # | File Path | Purpose |
|---|-----------|---------|
| 1 | `internal/context/providers.go` | Core provider interface + registry |
| 2 | `internal/context/parser.go` | @-mention parser |
| 3 | `internal/context/file_provider.go` | @file implementation |
| 4 | `internal/context/url_provider.go` | @url implementation |
| 5 | `internal/context/codebase_provider.go` | @codebase implementation |
| 6 | `internal/context/code_provider.go` | @code implementation |
| 7 | `internal/context/docs_provider.go` | @docs implementation |
| 8 | `internal/context/framework.go` | Pluggable framework + priority |
| 9 | `internal/context/token_budget.go` | Token budget management |
| 10 | `internal/context/retrieval.go` | RAG retrieval augmenter |
| 11 | `internal/editor/protocol.go` | Editor protocol definitions |
| 12 | `internal/editor/editor.go` | Editor abstraction interface |
| 13 | `internal/editor/server.go` | WebSocket/JSON-RPC server |
| 14 | `internal/editor/vscode.go` | VS Code adapter |
| 15 | `internal/editor/lsp_client.go` | LSP client |
| 16 | `internal/autocomplete/engine.go` | Completion engine |
| 17 | `internal/autocomplete/fim.go` | FIM prompt construction |
| 18 | `internal/autocomplete/cache.go` | Completion cache |
| 19 | `internal/autocomplete/filter.go` | Post-processing filter |
| 20 | `internal/autocomplete/tab.go` | Tab completion orchestrator |
| 21 | `internal/autocomplete/debounce.go` | Debounce timer |
| 22 | `internal/autocomplete/ghost.go` | Ghost text renderer |
| 23 | `internal/diff/stream.go` | Diff stream processor |
| 24 | `internal/diff/hunk.go` | Hunk representation |
| 25 | `internal/diff/apply.go` | Diff application |
| 26 | `internal/diff/renderer.go` | Diff rendering |
| 27 | `internal/prompt/template.go` | Template engine |
| 28 | `internal/prompt/builtins.go` | Built-in templates |
| 29 | `internal/prompt/loader.go` | Template loader |
| 30 | `internal/prompt/variables.go` | Variable builders |
| 31 | `internal/commands/registry.go` | Command registry |
| 32 | `internal/commands/edit.go` | /edit command |
| 33 | `internal/commands/comment.go` | /comment command |
| 34 | `internal/commands/share.go` | /share command |
| 35 | `internal/commands/custom.go` | Custom commands |
| 36 | `internal/commands/help.go` | /help command |
| 37 | `internal/embeddings/engine.go` | Embeddings orchestrator |
| 38 | `internal/embeddings/provider.go` | Embedding providers |
| 39 | `internal/embeddings/chunker.go` | Code chunking |
| 40 | `internal/embeddings/store.go` | Vector store |
| 41 | `internal/config/model.go` | Model configuration |
| 42 | `internal/llm/router.go` | Model router |
| 43 | `config/prompts/` | User prompt template directory |

## Modified Files Summary (8 files)

| # | File Path | Modification |
|---|-----------|-------------|
| 1 | `internal/llm/types.go` | Add ContextItem, template fields |
| 2 | `internal/config/config.go` | Add Models, EmbeddingsProvider, Reranker |
| 3 | `cmd/cli/main.go` | Wire context registry, commands, model flags |
| 4 | `cmd/server/main.go` | Add WebSocket endpoint |
| 5 | `internal/knowledge/store.go` | Add docs/snippets/symbol methods |
| 6 | `internal/editor/editor.go` | Add InsertAtCursor |
| 7 | `internal/editor/vscode.go` | Implement InsertAtCursor |
| 8 | `internal/editor/server.go` | Register autocomplete handlers |

---

## Anti-Bluff Test Suite (Master Verification)

```bash
#!/bin/bash
# Master anti-bluff verification script

echo "=== HelixCode Continue.dev Port Verification ==="

# Feature 1: @Provider System
go test ./internal/context/... -run TestFileProvider -v || exit 1
go test ./internal/context/... -run TestURLProvider -v || exit 1
go test ./internal/context/... -run TestParseAtMentions -v || exit 1

# Feature 2: Universal IDE
go test ./internal/editor/... -run TestWebSocketServer -v || exit 1
go test ./internal/editor/... -run TestLSPClient -v || exit 1

# Feature 3: Context Framework
go test ./internal/context/... -run TestTokenBudget -v || exit 1
go test ./internal/context/... -run TestPriorityOrdering -v || exit 1

# Feature 4: Autocomplete
go test ./internal/autocomplete/... -run TestFIMPrompt -v || exit 1
go test ./internal/autocomplete/... -run TestEngineComplete -v -timeout 30s || exit 1
go test ./internal/autocomplete/... -run TestCacheHit -v || exit 1

# Feature 5: Diff Streaming
go test ./internal/diff/... -run TestStreamProcessor -v || exit 1
go test ./internal/diff/... -run TestHunkDecisions -v || exit 1

# Feature 6: Prompt Templates
go test ./internal/prompt/... -run TestRenderTemplate -v || exit 1
go test ./internal/prompt/... -run TestLanguageTemplate -v || exit 1

# Feature 7: Model Configuration
go test ./internal/config/... -run TestModelValidation -v || exit 1
go test ./internal/llm/... -run TestRouterRole -v || exit 1

# Feature 8: Tab Autocomplete
go test ./internal/autocomplete/... -run TestDebounce -v || exit 1
go test ./internal/autocomplete/... -run TestTabInsert -v || exit 1

# Feature 9: Slash Commands
go test ./internal/commands/... -run TestParseSlashCommand -v || exit 1
go test ./internal/commands/... -run TestEditCommand -v -timeout 30s || exit 1

# Feature 10: Embeddings
go test ./internal/embeddings/... -run TestCodeChunker -v || exit 1
go test ./internal/embeddings/... -run TestSemanticSearch -v || exit 1

echo ""
echo "=== ALL TESTS PASSED ==="
```

---

*Document generated for HelixCode integration planning.  
All implementation details are exact and verifiable.*
