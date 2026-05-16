// Package builder provides a fluent API for constructing LLM context.
//
// The builder aggregates context from multiple sources (sessions, focus chains,
// files, git history) and assembles them into a coherent context string for
// LLM calls. It handles priority-based selection when context exceeds size limits.
//
// # Context Sources
//
// The builder supports various source types:
//   - SourceSession: Current session information
//   - SourceFocus: Recent user focus history
//   - SourceFile: File contents
//   - SourceGit: Git changes and history
//   - SourceProject: Project metadata
//   - SourceError: Error messages and stack traces
//   - SourceLog: Relevant log entries
//   - SourceCustom: User-defined context
//
// # Priority System
//
// Each context item has a priority level:
//   - PriorityCritical (20): Must be included
//   - PriorityHigh (10): Important context
//   - PriorityNormal (5): Standard context
//   - PriorityLow (1): Optional background
//
// When context exceeds the configured maximum size, lower-priority items are
// dropped first.
//
// # Usage
//
//	b := builder.NewBuilder()
//	b.SetMaxTokens(4000)
//	b.AddText("Task", "Fix login bug", builder.PriorityHigh)
//	b.AddSession(currentSession)
//	b.AddFocusChain(focusChain, 5)
//	context, err := b.Build()
//
// # Templates
//
// For consistent context structures, register and use templates:
//
//	b.RegisterTemplate(&builder.Template{
//	    Name: "code-review",
//	    Sections: []*builder.TemplateSection{
//	        {Title: "Code", Types: []builder.SourceType{builder.SourceFile}},
//	        {Title: "Context", Types: []builder.SourceType{builder.SourceGit}},
//	    },
//	})
//	context, err := b.BuildWithTemplate("code-review")
//
// # Caching
//
// The builder caches built context with a configurable TTL to avoid rebuilding
// for repeated calls. Cache is automatically invalidated when items change.
package builder
