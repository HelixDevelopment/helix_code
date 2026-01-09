// Package formats provides multi-format code editing support.
//
// Different LLM models work better with different edit representations.
// This package defines the EditFormat interface and provides implementations
// for various edit formats, allowing the system to select the optimal format
// for each model.
//
// # Edit Formats
//
// The package supports these format types:
//
// FormatTypeWhole - Replace entire file content. Simple and reliable but
// verbose for small changes.
//
// FormatTypeDiff - Standard unified diff format. Compact for small changes,
// familiar to developers.
//
// FormatTypeUDiff - Git-style unified diff with extended headers. More
// context than standard diff.
//
// FormatTypeSearchReplace - Regex-based search and replace. Precise for
// pattern-based modifications.
//
// FormatTypeEditor - Line-number based editing. Specify exact lines to
// modify, insert, or delete.
//
// FormatTypeLineNumber - Direct line number references. Simple line-based
// modifications.
//
// FormatTypeArchitect - High-level structural changes. For refactoring and
// architectural modifications.
//
// FormatTypeAsk - Question/confirmation mode. For interactive clarification
// before making changes.
//
// # Format Registry
//
// Formats are registered and accessed through FormatRegistry:
//
//	registry := formats.NewFormatRegistry()
//	formats.RegisterAllFormats(registry)
//
//	// Parse with specific format
//	edits, err := registry.ParseWithFormat(ctx, formats.FormatTypeDiff, content)
//
//	// Auto-detect format
//	edits, format, err := registry.ParseWithAutoDetect(ctx, content)
//
// # FileEdit Structure
//
// All formats parse to a common FileEdit structure containing:
//   - FilePath: Target file
//   - Operation: create, update, delete, rename
//   - OldContent/NewContent: Before/after content
//   - LineNumber/LineCount: For line-based edits
//   - SearchPattern/ReplaceWith: For regex operations
package formats
