// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package focus provides a hierarchical attention management system for tracking
development focus points during coding sessions in the HelixCode platform.

# Overview

The focus package helps the AI agent maintain context awareness by tracking what
files, tasks, errors, tests, or code elements are currently being worked on.
It supports hierarchical focus organization, priority management, expiration,
tagging, and chain-based focus tracking.

# Focus Types

FocusType defines what kind of entity is being focused on:

  - FocusTypeFile: Single file
  - FocusTypeDirectory: Directory
  - FocusTypeTask: Task or feature
  - FocusTypeError: Error or bug investigation
  - FocusTypeTest: Test case
  - FocusTypeFunction: Specific function
  - FocusTypeClass: Class or struct
  - FocusTypePackage: Package or module
  - FocusTypeProject: Entire project
  - FocusTypeCustom: Custom focus type

# Creating Focuses

Create new focus points:

	// Simple focus
	f := focus.NewFocus(focus.FocusTypeFile, "/path/to/file.go")

	// With priority
	f := focus.NewFocusWithPriority(
	    focus.FocusTypeError,
	    "nil pointer in handler",
	    focus.PriorityCritical,
	)

# Priority Levels

FocusPriority defines importance levels:

  - PriorityLow (1): Background tasks
  - PriorityNormal (5): Standard work
  - PriorityHigh (10): Important items
  - PriorityCritical (20): Urgent issues

# Hierarchical Organization

Focuses can be organized in parent-child relationships:

	// Create project focus
	project := focus.NewFocus(focus.FocusTypeProject, "my-app")

	// Add package focus
	pkg := focus.NewFocus(focus.FocusTypePackage, "internal/auth")
	project.AddChild(pkg)

	// Add file focus
	file := focus.NewFocus(focus.FocusTypeFile, "handler.go")
	pkg.AddChild(file)

	// Navigate hierarchy
	depth := file.GetDepth()    // Returns 2
	root := file.GetRoot()      // Returns project
	path := file.GetPath()      // Returns [project, pkg, file]

# Context and Tags

Attach metadata to focuses:

	f := focus.NewFocus(focus.FocusTypeTask, "implement-auth")

	// Add context
	f.SetContext("related_files", []string{"auth.go", "token.go"})
	f.SetContext("assignee", "ai-agent")

	// Add tags
	f.AddTag("security")
	f.AddTag("high-priority")

	// Check tags
	if f.HasTag("security") {
	    // Handle security-related focus
	}

	// Access metadata
	f.SetMetadata("key", "value")
	value, ok := f.GetMetadata("key")

# Expiration

Focuses can expire automatically:

	f.SetExpiration(2 * time.Hour)

	if f.IsExpired() {
	    // Clean up focus
	}

	// Update timestamp to keep focus active
	f.Touch()

# Focus Chains

Chain manages a stack of focuses for tracking focus history:

	chain := focus.NewChain("auth-feature")

	// Push focuses
	chain.Push(focus.NewFocus(focus.FocusTypeFile, "auth.go"))
	chain.Push(focus.NewFocus(focus.FocusTypeFunction, "Authenticate"))

	// Get current focus
	current, _ := chain.Current()

	// Pop to return to previous focus
	chain.Pop()

	// Navigate history
	chain.Back()
	chain.Forward()

# Focus Manager

Manager handles multiple focus chains with thread-safe operations:

	manager := focus.NewManager()

	// Create chains
	chain, _ := manager.CreateChain("feature-1", true) // set as active
	manager.CreateChain("bugfix-1", false)

	// Work with active chain
	manager.PushToActive(newFocus)
	current, _ := manager.GetCurrentFocus()

	// Switch active chain
	manager.SetActiveChain(otherChainID)

	// Get statistics
	stats := manager.GetStatistics()
	// stats.TotalChains, stats.TotalFocuses, stats.AverageFocusesPerChain

# Chain Operations

Manager supports advanced chain operations:

	// Find chains by name
	chains := manager.FindChainsByName("feature")

	// Get recent chains
	recent := manager.GetRecentChains(5)

	// Merge chains
	manager.MergeChains(targetID, sourceID)

	// Export/Import chains
	snapshot, _ := manager.ExportChain(chainID)
	manager.ImportChain(snapshot, true)

	// Clean expired focuses across all chains
	removed := manager.CleanExpiredFocuses()

# Callbacks

Manager supports callbacks for chain events:

	manager.OnCreate(func(chain *Chain) {
	    log.Printf("Chain created: %s", chain.Name)
	})

	manager.OnDelete(func(chain *Chain) {
	    log.Printf("Chain deleted: %s", chain.Name)
	})

	manager.OnActivate(func(chain *Chain) {
	    log.Printf("Chain activated: %s", chain.Name)
	})

# Validation

Focuses support validation:

	if err := f.Validate(); err != nil {
	    log.Printf("Invalid focus: %v", err)
	}

Validation checks:
  - ID is not empty
  - Type is not empty
  - Target is not empty
  - Priority is within valid range
  - Expiration time is after creation time

# Cloning

Create deep copies of focuses and chains:

	// Clone a focus (includes all children)
	clone := f.Clone()

	// Clone a chain
	chainClone := chain.Clone()

# Thread Safety

Manager is fully thread-safe for concurrent access. Individual Focus and Chain
operations should be synchronized externally if accessed from multiple goroutines.

# Integration

The focus system integrates with:

  - Task Manager: Track which tasks are currently active
  - Context Builder: Include relevant focuses in AI prompts
  - Agent Orchestration: Direct agent attention to priority items

# Usage Example

Complete usage example:

	manager := focus.NewManager()

	// Create a chain for the current session
	chain, _ := manager.CreateChain("bug-investigation", true)

	// Track focus progression
	manager.PushToActive(focus.NewFocusWithPriority(
	    focus.FocusTypeError,
	    "NullPointerException in UserService",
	    focus.PriorityCritical,
	))

	manager.PushToActive(focus.NewFocus(
	    focus.FocusTypeFile,
	    "/src/services/UserService.java",
	))

	manager.PushToActive(focus.NewFocus(
	    focus.FocusTypeFunction,
	    "getUserById",
	))

	// Get current focus for context
	current, _ := manager.GetCurrentFocus()
	log.Printf("Currently focused on: %s", current.String())
*/
package focus
