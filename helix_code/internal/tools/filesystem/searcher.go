package filesystem

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// FileSearcher provides methods for searching files
type FileSearcher interface {
	// Search searches for files matching criteria
	Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error)

	// SearchContent searches file contents for a pattern
	SearchContent(ctx context.Context, opts ContentSearchOptions) ([]ContentMatch, error)

	// Glob performs glob pattern matching
	Glob(ctx context.Context, pattern string) ([]string, error)

	// Walk walks a directory tree
	Walk(ctx context.Context, root string, fn WalkFunc) error
}

// SearchOptions configures file search
type SearchOptions struct {
	Root           string
	Pattern        string
	IncludePattern []string
	ExcludePattern []string
	MaxDepth       int
	FollowSymlinks bool
	IncludeDirs    bool
	IncludeHidden  bool
	MaxResults     int
	SortBy         SortType
}

// ContentSearchOptions configures content search
type ContentSearchOptions struct {
	Root          string
	Pattern       string
	IsRegex       bool
	CaseSensitive bool
	IncludeFiles  []string
	ExcludeFiles  []string
	MaxMatches    int
	ContextLines  int
	MaxFileSize   int64
}

// SearchResult represents a file search result
type SearchResult struct {
	Path    string
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
	Depth   int
	Match   string // What matched
}

// ContentMatch represents a content search match
type ContentMatch struct {
	Path         string
	LineNumber   int
	ColumnNumber int
	Line         string
	Match        string
	Context      []string // Surrounding lines
}

// WalkFunc is called for each file during directory walking
type WalkFunc func(path string, info FileInfo, err error) error

// SortType defines how to sort search results
type SortType int

const (
	SortByName SortType = iota
	SortBySize
	SortByModTime
	SortByDepth
)

// fileSearcher implements FileSearcher
type fileSearcher struct {
	fs *FileSystemTools
}

// Search searches for files matching criteria
func (s *fileSearcher) Search(ctx context.Context, opts SearchOptions) ([]SearchResult, error) {
	// Set defaults
	if opts.Root == "" {
		opts.Root = s.fs.config.WorkspaceRoot
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 100 // Default max depth
	}

	// Validate root path
	validationResult, err := s.fs.pathValidator.Validate(opts.Root)
	if err != nil {
		return nil, err
	}
	rootPath := validationResult.NormalizedPath

	// Check permissions
	if err := s.fs.permissionChecker.CheckPermission(rootPath, OpRead); err != nil {
		return nil, err
	}

	var results []SearchResult
	resultCount := 0

	// Walk directory tree.
	// P2-T01: filepath.WalkDir — lazy fs.DirEntry; d.Info() is resolved once
	// per visited entry because the result set needs size/mode/mtime.
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip files we can't access
			return nil
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Calculate depth
		rel, err := filepath.Rel(rootPath, path)
		if err != nil {
			return nil
		}
		depth := len(strings.Split(rel, string(os.PathSeparator)))
		if rel == "." {
			depth = 0
		}

		// Check max depth
		if depth > opts.MaxDepth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files/directories if not included
		if !opts.IncludeHidden && isHidden(path) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			// Skip entries we can't stat
			return nil
		}

		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 && !opts.FollowSymlinks {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip directories if not included
		if info.IsDir() && !opts.IncludeDirs {
			return nil
		}

		// Check if matches pattern
		matched := s.matchesPattern(path, info, opts)
		if !matched {
			return nil
		}

		// Add to results
		results = append(results, SearchResult{
			Path:    path,
			Name:    info.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Depth:   depth,
			Match:   opts.Pattern,
		})

		resultCount++
		if opts.MaxResults > 0 && resultCount >= opts.MaxResults {
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("failed to search files: %w", err)
	}

	// Sort results
	s.sortResults(results, opts.SortBy)

	return results, nil
}

// SearchContent searches file contents for a pattern
func (s *fileSearcher) SearchContent(ctx context.Context, opts ContentSearchOptions) ([]ContentMatch, error) {
	// Set defaults
	if opts.Root == "" {
		opts.Root = s.fs.config.WorkspaceRoot
	}
	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = s.fs.config.MaxFileSize
	}

	// Validate root path
	validationResult, err := s.fs.pathValidator.Validate(opts.Root)
	if err != nil {
		return nil, err
	}
	rootPath := validationResult.NormalizedPath

	// Compile regex if needed
	var re *regexp.Regexp
	if opts.IsRegex {
		flags := ""
		if !opts.CaseSensitive {
			flags = "(?i)"
		}
		re, err = regexp.Compile(flags + opts.Pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	// P2-T05: parallelise the SearchContent grep hot path (R1 bottleneck B08).
	//
	// The old implementation walked the tree and read+scanned every candidate
	// file inline on the single walk goroutine. This split the work into two
	// phases:
	//
	//  1. Enumerate — filepath.WalkDir (P2-T01, cheap fs.DirEntry) collects the
	//     candidate file paths, applying the directory/hidden/size/include-
	//     exclude filters. Walk stays single-threaded but does only I/O-light
	//     directory traversal.
	//  2. Scan — a bounded errgroup worker pool (limit = runtime.NumCPU())
	//     reads + scans the candidate files concurrently; each worker writes
	//     its result into its own pre-allocated slot, so there is no shared-
	//     slice mutation and the pool is -race clean.
	//
	// Results are then flattened and sorted (by path, then line number) so the
	// output is byte-identical run-to-run regardless of goroutine scheduling —
	// the no-regression contract. MaxMatches is applied AFTER the sort, so the
	// truncated result set is deterministic too (the serial path truncated in
	// walk order; the parallel path truncates in sorted order — both honour the
	// "first N matches" cap, the parallel one just makes "first" well-defined).

	// Phase 1: enumerate candidate paths.
	// P2-T01: filepath.WalkDir — lazy fs.DirEntry; d.Info() resolved only on
	// the file branch that needs size.
	var candidates []string
	err = filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Skip hidden files
		if isHidden(path) {
			return nil
		}

		// Check file size
		if opts.MaxFileSize > 0 {
			info, infoErr := d.Info()
			if infoErr != nil {
				return nil
			}
			if info.Size() > opts.MaxFileSize {
				return nil
			}
		}

		// Check include/exclude patterns
		if !s.matchesContentSearchPatterns(path, opts) {
			return nil
		}

		candidates = append(candidates, path)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search content: %w", err)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	// Phase 2: scan candidate files through a bounded worker pool. Each worker
	// writes into its own slot of perFile, so no synchronisation is needed for
	// the result collection itself.
	perFile := make([][]ContentMatch, len(candidates))
	g, gctx := errgroup.WithContext(ctx)

	limit := runtime.NumCPU()
	if limit < 1 {
		limit = 1
	}
	if limit > len(candidates) {
		limit = len(candidates)
	}
	g.SetLimit(limit)

	for i, path := range candidates {
		i, path := i, path
		g.Go(func() error {
			// Honour cancellation between dispatched units of work.
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			fileMatches, scanErr := s.searchFileContent(path, opts, re)
			if scanErr != nil {
				// Skip files we can't read — same semantics as the serial path.
				return nil
			}
			perFile[i] = fileMatches
			return nil
		})
	}

	if waitErr := g.Wait(); waitErr != nil {
		return nil, fmt.Errorf("failed to search content: %w", waitErr)
	}

	// Phase 3: flatten + deterministically order the result set.
	var matches []ContentMatch
	for _, fm := range perFile {
		matches = append(matches, fm...)
	}
	sortContentMatches(matches)

	// Apply MaxMatches cap after the sort so the truncated set is deterministic.
	if opts.MaxMatches > 0 && len(matches) > opts.MaxMatches {
		matches = matches[:opts.MaxMatches]
	}

	return matches, nil
}

// sortContentMatches orders content matches deterministically by path, then by
// line number, then by column. This makes SearchContent output byte-identical
// run-to-run regardless of the worker-pool goroutine scheduling (P2-T05).
func sortContentMatches(matches []ContentMatch) {
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Path != matches[j].Path {
			return matches[i].Path < matches[j].Path
		}
		if matches[i].LineNumber != matches[j].LineNumber {
			return matches[i].LineNumber < matches[j].LineNumber
		}
		return matches[i].ColumnNumber < matches[j].ColumnNumber
	})
}

// Glob performs glob pattern matching
func (s *fileSearcher) Glob(ctx context.Context, pattern string) ([]string, error) {
	// If pattern is not absolute, make it relative to workspace root
	if !filepath.IsAbs(pattern) {
		pattern = filepath.Join(s.fs.config.WorkspaceRoot, pattern)
	}

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to match glob pattern: %w", err)
	}

	// Validate all matched paths
	var validMatches []string
	for _, match := range matches {
		_, err := s.fs.pathValidator.Validate(match)
		if err == nil {
			validMatches = append(validMatches, match)
		}
	}

	return validMatches, nil
}

// Walk walks a directory tree
func (s *fileSearcher) Walk(ctx context.Context, root string, fn WalkFunc) error {
	// Validate root path
	validationResult, err := s.fs.pathValidator.Validate(root)
	if err != nil {
		return err
	}
	rootPath := validationResult.NormalizedPath

	// Check permissions
	if err := s.fs.permissionChecker.CheckPermission(rootPath, OpRead); err != nil {
		return err
	}

	// P2-T01: filepath.WalkDir — lazy fs.DirEntry; d.Info() resolved per entry
	// because the public WalkFunc contract surfaces size/mode/mtime.
	return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Convert to FileInfo
		var fileInfo FileInfo
		walkErr := err
		if d != nil {
			info, infoErr := d.Info()
			if infoErr != nil {
				if walkErr == nil {
					walkErr = infoErr
				}
			} else {
				fileInfo = FileInfo{
					Path:    path,
					Name:    info.Name(),
					Size:    info.Size(),
					Mode:    info.Mode(),
					ModTime: info.ModTime(),
					IsDir:   info.IsDir(),
				}
			}
		}

		return fn(path, fileInfo, walkErr)
	})
}

// matchesPattern checks if a file matches the search pattern
func (s *fileSearcher) matchesPattern(path string, info os.FileInfo, opts SearchOptions) bool {
	name := info.Name()

	// Check main pattern
	if opts.Pattern != "" {
		matched, err := filepath.Match(opts.Pattern, name)
		if err != nil || !matched {
			return false
		}
	}

	// Check include patterns
	if len(opts.IncludePattern) > 0 {
		matched := false
		for _, pattern := range opts.IncludePattern {
			if m, _ := filepath.Match(pattern, name); m {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range opts.ExcludePattern {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}

	return true
}

// matchesContentSearchPatterns checks if a file matches content search patterns
func (s *fileSearcher) matchesContentSearchPatterns(path string, opts ContentSearchOptions) bool {
	name := filepath.Base(path)

	// Check include patterns
	if len(opts.IncludeFiles) > 0 {
		matched := false
		for _, pattern := range opts.IncludeFiles {
			if m, _ := filepath.Match(pattern, name); m {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check exclude patterns
	for _, pattern := range opts.ExcludeFiles {
		if matched, _ := filepath.Match(pattern, name); matched {
			return false
		}
	}

	return true
}

// searchFileContent searches content in a single file
func (s *fileSearcher) searchFileContent(path string, opts ContentSearchOptions, re *regexp.Regexp) ([]ContentMatch, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var matches []ContentMatch
	scanner := bufio.NewScanner(file)
	lineNum := 0
	var contextBuffer []string

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Maintain context buffer
		if opts.ContextLines > 0 {
			contextBuffer = append(contextBuffer, line)
			if len(contextBuffer) > opts.ContextLines*2+1 {
				contextBuffer = contextBuffer[1:]
			}
		}

		// Check for match
		var matched bool
		var matchStr string
		var colNum int

		if opts.IsRegex {
			if re.MatchString(line) {
				matched = true
				matchStr = re.FindString(line)
				colNum = strings.Index(line, matchStr) + 1
			}
		} else {
			searchPattern := opts.Pattern
			searchLine := line
			if !opts.CaseSensitive {
				searchPattern = strings.ToLower(searchPattern)
				searchLine = strings.ToLower(line)
			}
			if strings.Contains(searchLine, searchPattern) {
				matched = true
				matchStr = opts.Pattern
				colNum = strings.Index(searchLine, searchPattern) + 1
			}
		}

		if matched {
			// Extract context
			var context []string
			if opts.ContextLines > 0 {
				start := len(contextBuffer) - opts.ContextLines - 1
				if start < 0 {
					start = 0
				}
				end := len(contextBuffer)
				context = make([]string, end-start)
				copy(context, contextBuffer[start:end])
			}

			matches = append(matches, ContentMatch{
				Path:         path,
				LineNumber:   lineNum,
				ColumnNumber: colNum,
				Line:         line,
				Match:        matchStr,
				Context:      context,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return matches, nil
}

// sortResults sorts search results by the specified criteria
func (s *fileSearcher) sortResults(results []SearchResult, sortBy SortType) {
	sort.Slice(results, func(i, j int) bool {
		switch sortBy {
		case SortByName:
			return results[i].Name < results[j].Name
		case SortBySize:
			return results[i].Size < results[j].Size
		case SortByModTime:
			return results[i].ModTime.Before(results[j].ModTime)
		case SortByDepth:
			return results[i].Depth < results[j].Depth
		default:
			return results[i].Name < results[j].Name
		}
	})
}

// isHidden checks if a file or directory is hidden
func isHidden(path string) bool {
	name := filepath.Base(path)
	return len(name) > 0 && name[0] == '.'
}
