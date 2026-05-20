package multiedit

import (
	"context"
	"encoding/json"
	"fmt"
)

// Editor is the main interface for multi-file editing
type Editor interface {
	// BeginEdit starts a new edit transaction
	BeginEdit(ctx context.Context, opts EditOptions) (*EditTransaction, error)

	// AddEdit adds a file edit to the transaction
	AddEdit(ctx context.Context, tx *EditTransaction, edit *FileEdit) error

	// Preview generates a preview of changes
	Preview(ctx context.Context, tx *EditTransaction) (*PreviewResult, error)

	// Commit applies all changes atomically
	Commit(ctx context.Context, tx *EditTransaction) error

	// Rollback reverts all changes
	Rollback(ctx context.Context, tx *EditTransaction) error

	// GetTransaction retrieves a transaction by ID
	GetTransaction(ctx context.Context, txID string) (*EditTransaction, error)
}

// BackupProvider handles file backups
type BackupProvider interface {
	Backup(ctx context.Context, filePath string) (string, error)
	Restore(ctx context.Context, backupPath, targetPath string) error
	Cleanup(ctx context.Context) error
}

// DiffProvider generates and applies diffs
type DiffProvider interface {
	GenerateDiff(old, new []byte, path string) (*Diff, error)
	ApplyDiff(diff *Diff) ([]byte, error)
	ParseDiff(diffText string) (*Diff, error)
}

// ConflictDetector detects conflicts
type ConflictDetector interface {
	DetectConflicts(ctx context.Context, tx *EditTransaction) ([]*Conflict, error)
	Resolve(ctx context.Context, conflict *Conflict, strategy ConflictStrategy) error
}

// PreviewEngine generates previews of changes
type PreviewEngine struct {
	diffManager  *DiffManager
	formatter    *PreviewFormatter
	contextLines int
}

// NewPreviewEngine creates a new preview engine
func NewPreviewEngine(diffManager *DiffManager, contextLines int) *PreviewEngine {
	return &PreviewEngine{
		diffManager:  diffManager,
		formatter:    NewPreviewFormatter(FormatPlain),
		contextLines: contextLines,
	}
}

// Preview generates a preview of all changes
func (pe *PreviewEngine) Preview(ctx context.Context, tx *EditTransaction) (*PreviewResult, error) {
	result := &PreviewResult{
		TransactionID: tx.ID,
		Files:         make([]*FilePreview, 0, len(tx.Files)),
		Summary:       &PreviewSummary{},
	}

	for _, edit := range tx.Files {
		preview, err := pe.previewFile(ctx, edit)
		if err != nil {
			return nil, fmt.Errorf("failed to preview file %s: %w", edit.FilePath, err)
		}
		result.Files = append(result.Files, preview)

		// Update summary
		pe.updateSummary(result.Summary, preview)
	}

	return result, nil
}

// previewFile generates preview for a single file
func (pe *PreviewEngine) previewFile(ctx context.Context, edit *FileEdit) (*FilePreview, error) {
	preview := &FilePreview{
		FilePath:  edit.FilePath,
		Operation: edit.Operation,
		Status:    StatusOK,
	}

	// Generate diff
	switch edit.Operation {
	case OpUpdate:
		diff, err := pe.diffManager.GenerateDiff(edit.OldContent, edit.NewContent, edit.FilePath)
		if err != nil {
			preview.Status = StatusError
			return preview, fmt.Errorf("failed to generate diff: %w", err)
		}
		preview.Diff = diff
		preview.Stats = FileStats{
			LinesAdded:   diff.Stats.LinesAdded,
			LinesDeleted: diff.Stats.LinesDeleted,
			LinesChanged: diff.Stats.LinesAdded + diff.Stats.LinesDeleted,
			SizeChange:   int64(len(edit.NewContent)) - int64(len(edit.OldContent)),
		}
	case OpCreate:
		preview.Stats = FileStats{
			LinesAdded: len(splitLines(string(edit.NewContent))),
			SizeChange: int64(len(edit.NewContent)),
		}
	case OpDelete:
		preview.Stats = FileStats{
			LinesDeleted: len(splitLines(string(edit.OldContent))),
			SizeChange:   -int64(len(edit.OldContent)),
		}
	case OpRename:
		preview.TargetPath = edit.TargetPath
		// If content is also being modified during rename
		if len(edit.NewContent) > 0 && len(edit.OldContent) > 0 {
			diff, err := pe.diffManager.GenerateDiff(edit.OldContent, edit.NewContent, edit.FilePath)
			if err == nil {
				preview.Diff = diff
				preview.Stats = FileStats{
					LinesAdded:   diff.Stats.LinesAdded,
					LinesDeleted: diff.Stats.LinesDeleted,
					LinesChanged: diff.Stats.LinesAdded + diff.Stats.LinesDeleted,
					SizeChange:   int64(len(edit.NewContent)) - int64(len(edit.OldContent)),
				}
			}
		}
	}

	return preview, nil
}

// updateSummary updates the preview summary
func (pe *PreviewEngine) updateSummary(summary *PreviewSummary, preview *FilePreview) {
	summary.TotalFiles++

	switch preview.Operation {
	case OpCreate:
		summary.FilesCreated++
	case OpUpdate:
		summary.FilesModified++
	case OpDelete:
		summary.FilesDeleted++
	case OpRename:
		summary.FilesRenamed++
	}

	summary.TotalLinesAdded += preview.Stats.LinesAdded
	summary.TotalLinesDeleted += preview.Stats.LinesDeleted
}

// PreviewResult contains preview information
type PreviewResult struct {
	TransactionID string
	Files         []*FilePreview
	Summary       *PreviewSummary
	Conflicts     []*Conflict
}

// FilePreview contains preview for a single file
type FilePreview struct {
	FilePath   string
	TargetPath string // Used for rename operations - the new path
	Operation  EditOperation
	Diff       *Diff
	Stats      FileStats
	Status     PreviewStatus
}

// FileStats contains file statistics
type FileStats struct {
	LinesAdded   int
	LinesDeleted int
	LinesChanged int
	SizeChange   int64
}

// PreviewStatus indicates preview status
type PreviewStatus int

const (
	StatusOK PreviewStatus = iota
	StatusConflict
	StatusError
)

// String returns the string representation of the status
func (ps PreviewStatus) String() string {
	switch ps {
	case StatusOK:
		return "ok"
	case StatusConflict:
		return "conflict"
	case StatusError:
		return "error"
	default:
		return "unknown"
	}
}

// PreviewSummary summarizes all changes
type PreviewSummary struct {
	TotalFiles        int
	FilesCreated      int
	FilesModified     int
	FilesDeleted      int
	FilesRenamed      int
	TotalLinesAdded   int
	TotalLinesDeleted int
	HasConflicts      bool
}

// PreviewFormatter formats preview output
type PreviewFormatter struct {
	format OutputFormat
}

// OutputFormat specifies the output format
type OutputFormat int

const (
	FormatPlain OutputFormat = iota
	FormatMarkdown
	FormatJSON
	FormatHTML
)

// String returns the string representation of the format
func (of OutputFormat) String() string {
	switch of {
	case FormatPlain:
		return "plain"
	case FormatMarkdown:
		return "markdown"
	case FormatJSON:
		return "json"
	case FormatHTML:
		return "html"
	default:
		return "unknown"
	}
}

// NewPreviewFormatter creates a new preview formatter
func NewPreviewFormatter(format OutputFormat) *PreviewFormatter {
	return &PreviewFormatter{
		format: format,
	}
}

// Format formats a preview result
func (pf *PreviewFormatter) Format(result *PreviewResult) (string, error) {
	switch pf.format {
	case FormatPlain:
		return pf.formatPlain(result)
	case FormatMarkdown:
		return pf.formatMarkdown(result)
	case FormatJSON:
		return pf.formatJSON(result)
	case FormatHTML:
		return pf.formatHTML(result)
	default:
		return "", fmt.Errorf("unsupported format: %v", pf.format)
	}
}

// formatPlain formats as plain text
func (pf *PreviewFormatter) formatPlain(result *PreviewResult) (string, error) {
	var output string

	output += trc("internal_tools_multiedit_preview_transaction_line",
		map[string]any{"ID": result.TransactionID}) + "\n"
	output += trc("internal_tools_multiedit_preview_summary_line", map[string]any{
		"TotalFiles": result.Summary.TotalFiles,
		"Created":    result.Summary.FilesCreated,
		"Modified":   result.Summary.FilesModified,
		"Deleted":    result.Summary.FilesDeleted,
	}) + "\n"
	output += trc("internal_tools_multiedit_preview_changes_line", map[string]any{
		"Added":   result.Summary.TotalLinesAdded,
		"Deleted": result.Summary.TotalLinesDeleted,
	}) + "\n"

	if result.Summary.HasConflicts {
		output += "\n" + trc("internal_tools_multiedit_preview_conflicts_line",
			map[string]any{"Count": len(result.Conflicts)}) + "\n"
		for _, conflict := range result.Conflicts {
			output += fmt.Sprintf("  - %s: %s\n", conflict.FilePath, conflict.Description)
		}
	}

	output += "\n" + trc("internal_tools_multiedit_preview_files_heading", nil) + "\n"
	for _, file := range result.Files {
		output += fmt.Sprintf("\n%s (%s)\n", file.FilePath, file.Operation)
		if file.Diff != nil {
			output += file.Diff.Unified
		}
	}

	return output, nil
}

// formatMarkdown formats as markdown
func (pf *PreviewFormatter) formatMarkdown(result *PreviewResult) (string, error) {
	var output string

	output += trc("internal_tools_multiedit_preview_md_title",
		map[string]any{"ID": result.TransactionID}) + "\n\n"
	output += trc("internal_tools_multiedit_preview_md_summary_heading", nil) + "\n\n"
	output += trc("internal_tools_multiedit_preview_md_total_files",
		map[string]any{"Count": result.Summary.TotalFiles}) + "\n"
	output += trc("internal_tools_multiedit_preview_md_created",
		map[string]any{"Count": result.Summary.FilesCreated}) + "\n"
	output += trc("internal_tools_multiedit_preview_md_modified",
		map[string]any{"Count": result.Summary.FilesModified}) + "\n"
	output += trc("internal_tools_multiedit_preview_md_deleted",
		map[string]any{"Count": result.Summary.FilesDeleted}) + "\n"
	output += trc("internal_tools_multiedit_preview_md_lines_added",
		map[string]any{"Count": result.Summary.TotalLinesAdded}) + "\n"
	output += trc("internal_tools_multiedit_preview_md_lines_deleted",
		map[string]any{"Count": result.Summary.TotalLinesDeleted}) + "\n\n"

	if result.Summary.HasConflicts {
		output += trc("internal_tools_multiedit_preview_md_conflicts_heading", nil) + "\n\n"
		for _, conflict := range result.Conflicts {
			output += fmt.Sprintf("- **%s**: %s\n", conflict.FilePath, conflict.Description)
		}
		output += "\n"
	}

	output += trc("internal_tools_multiedit_preview_md_files_heading", nil) + "\n\n"
	for _, file := range result.Files {
		output += fmt.Sprintf("### %s (%s)\n\n", file.FilePath, file.Operation)
		if file.Diff != nil {
			output += "```diff\n"
			output += file.Diff.Unified
			output += "```\n\n"
		}
	}

	return output, nil
}

// formatJSON formats as JSON.
//
// Forensic anchor (round-34 §11.4 audit, 2026-05-18): the previous
// implementation returned a hand-rolled JSON snippet that included only
// {transaction_id, summary.total_files} and silently dropped every other
// field that the plain / markdown / html formatters render — Files,
// Conflicts, Summary.{FilesCreated, FilesModified, FilesDeleted,
// FilesRenamed, TotalLinesAdded, TotalLinesDeleted, HasConflicts}. A
// caller asking for FormatJSON expecting machine-readable parity with
// the human-readable formats received a stub that hid every diff and
// every conflict. That is a §11.4 HIGH PASS-bluff: the surface
// (FormatJSON) advertised JSON serialization of a PreviewResult, the
// body produced a two-field placeholder labelled "this would use
// encoding/json in a real implementation".
//
// The new implementation uses encoding/json (now wired) with
// MarshalIndent for human-debuggable output and surfaces marshalling
// failures as honest errors instead of swallowing them.
func (pf *PreviewFormatter) formatJSON(result *PreviewResult) (string, error) {
	if result == nil {
		return "", fmt.Errorf("multiedit: nil PreviewResult cannot be JSON-formatted")
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", fmt.Errorf("multiedit: failed to marshal PreviewResult to JSON: %w", err)
	}
	return string(data), nil
}

// formatHTML formats as HTML
func (pf *PreviewFormatter) formatHTML(result *PreviewResult) (string, error) {
	var output string

	output += "<html><body>\n"
	output += fmt.Sprintf("<h1>%s</h1>\n", trc("internal_tools_multiedit_preview_html_title",
		map[string]any{"ID": result.TransactionID}))
	output += fmt.Sprintf("<h2>%s</h2>\n", trc("internal_tools_multiedit_preview_html_summary_heading", nil))
	output += "<ul>\n"
	output += fmt.Sprintf("<li>%s</li>\n", trc("internal_tools_multiedit_preview_html_total_files",
		map[string]any{"Count": result.Summary.TotalFiles}))
	output += fmt.Sprintf("<li>%s</li>\n", trc("internal_tools_multiedit_preview_html_created",
		map[string]any{"Count": result.Summary.FilesCreated}))
	output += fmt.Sprintf("<li>%s</li>\n", trc("internal_tools_multiedit_preview_html_modified",
		map[string]any{"Count": result.Summary.FilesModified}))
	output += fmt.Sprintf("<li>%s</li>\n", trc("internal_tools_multiedit_preview_html_deleted",
		map[string]any{"Count": result.Summary.FilesDeleted}))
	output += "</ul>\n"

	if result.Summary.HasConflicts {
		output += fmt.Sprintf("<h2>%s</h2>\n", trc("internal_tools_multiedit_preview_html_conflicts_heading", nil))
		output += "<ul>\n"
		for _, conflict := range result.Conflicts {
			output += fmt.Sprintf("<li>%s: %s</li>\n", conflict.FilePath, conflict.Description)
		}
		output += "</ul>\n"
	}

	output += fmt.Sprintf("<h2>%s</h2>\n", trc("internal_tools_multiedit_preview_html_files_heading", nil))
	for _, file := range result.Files {
		output += fmt.Sprintf("<h3>%s (%s)</h3>\n", file.FilePath, file.Operation)
		if file.Diff != nil {
			output += "<pre><code>\n"
			output += file.Diff.Unified
			output += "</code></pre>\n"
		}
	}

	output += "</body></html>\n"

	return output, nil
}
