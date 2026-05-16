// Package mentions provides @mention parsing and resolution for context references.
//
// Mentions allow users to reference specific resources in their prompts using
// @-prefixed syntax. The package parses these mentions and resolves them to
// their full content for inclusion in LLM context.
//
// # Mention Types
//
// The package supports several mention types:
//
// @file:path/to/file.go - References a specific file
// @folder:src/components - References all files in a folder
// @url:https://example.com - Fetches and includes web content
// @git-changes - Current uncommitted changes
// @commit:abc123 - Specific commit contents
// @terminal - Recent terminal output
// @problems - Current IDE problems/errors
//
// # Usage
//
// Parse and resolve mentions in user input:
//
//	parser := mentions.NewParser(handlers)
//	result, err := parser.Parse(ctx, userInput)
//	// result.ProcessedText has mentions replaced with content
//	// result.Contexts contains the resolved mention data
//
// # Custom Handlers
//
// Implement MentionHandler to add custom mention types:
//
//	type MyHandler struct{}
//	func (h *MyHandler) Type() MentionType { return "custom" }
//	func (h *MyHandler) CanHandle(mention string) bool { ... }
//	func (h *MyHandler) Resolve(ctx, mention, options) (*MentionContext, error) { ... }
//
// # Fuzzy Search
//
// The package includes fuzzy search capabilities for matching file names when
// exact paths are not provided. This allows users to reference files by partial
// names that will be matched against the project file tree.
//
// # Token Counting
//
// Each resolved mention tracks its token count, allowing the caller to manage
// total context size when combining multiple mentions.
package mentions
