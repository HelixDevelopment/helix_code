// Copyright 2024 HelixCode. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

/*
Package event provides event-driven architecture support for the HelixCode platform.

# Overview

The event package implements a publish-subscribe event bus that enables loose coupling
between components. It supports both synchronous and asynchronous event handling,
event type categorization, severity levels, and comprehensive event metadata.

# Event Bus

EventBus manages event subscriptions and publishing:

	// Create async event bus
	bus := event.NewEventBus(true)

	// Create sync event bus
	bus := event.NewEventBus(false)

In async mode, handlers are invoked in separate goroutines. In sync mode,
handlers are invoked sequentially and errors are collected.

# Subscribing to Events

Subscribe handlers to specific event types:

	bus.Subscribe(event.EventTaskCompleted, func(ctx context.Context, e event.Event) error {
	    log.Printf("Task completed: %s", e.TaskID)
	    return nil
	})

	// Subscribe to multiple events
	bus.SubscribeMultiple([]event.EventType{
	    event.EventTaskCreated,
	    event.EventTaskStarted,
	    event.EventTaskCompleted,
	}, taskHandler)

	// Unsubscribe from an event type
	bus.Unsubscribe(event.EventTaskCreated)

# Publishing Events

Publish events to registered handlers:

	e := event.Event{
	    Type:      event.EventTaskCompleted,
	    Source:    "task-manager",
	    Severity:  event.SeverityInfo,
	    TaskID:    "task-123",
	    Data: map[string]interface{}{
	        "result": "success",
	        "duration": "5s",
	    },
	}

	// Fire and forget (async mode)
	err := bus.Publish(ctx, e)

	// Wait for all handlers (async mode)
	err := bus.PublishAndWait(ctx, e)

# Event Types

The package defines standard event types for the platform:

Task Events:
  - EventTaskCreated, EventTaskAssigned, EventTaskStarted
  - EventTaskCompleted, EventTaskFailed
  - EventTaskPaused, EventTaskResumed, EventTaskCancelled

Workflow Events:
  - EventWorkflowStarted, EventWorkflowCompleted, EventWorkflowFailed
  - EventStepCompleted, EventStepFailed

Worker Events:
  - EventWorkerConnected, EventWorkerDisconnected
  - EventWorkerHealthDegraded, EventWorkerHeartbeatMissed
  - EventWorkerTaskAssigned, EventWorkerTaskCompleted

API Events:
  - EventUserRegistered, EventUserLogin, EventUserLogout
  - EventProjectCreated, EventProjectDeleted
  - EventAuthFailure

System Events:
  - EventSystemStartup, EventSystemShutdown, EventSystemError

# Event Severity

Events have severity levels for filtering and alerting:

  - SeverityInfo: Informational events
  - SeverityWarning: Warning conditions
  - SeverityError: Error conditions
  - SeverityCritical: Critical issues requiring immediate attention

# Event Structure

Events contain comprehensive metadata:

	type Event struct {
	    ID        string                 // Unique identifier (auto-generated if empty)
	    Type      EventType              // Event type
	    Timestamp time.Time              // Event timestamp (auto-set if zero)
	    Source    string                 // Event source identifier
	    Severity  EventSeverity          // Severity level
	    Data      map[string]interface{} // Custom event data
	    UserID    string                 // Associated user ID
	    ProjectID string                 // Associated project ID
	    TaskID    string                 // Associated task ID
	    WorkerID  string                 // Associated worker ID
	}

# Error Handling

The bus maintains an error log for debugging:

	// Get recent errors
	errors := bus.GetErrors()

	// Clear error log
	bus.ClearErrors()

Errors are automatically trimmed to keep only the last 100 entries.

# Introspection

Query event bus state:

	// Get subscriber count for an event type
	count := bus.GetSubscriberCount(event.EventTaskCompleted)

	// Get total subscriptions
	total := bus.GetTotalSubscribers()

	// Get all event types with subscribers
	types := bus.GetSubscribedEvents()

	// Check async mode
	isAsync := bus.IsAsync()

# Global Instance

The package provides a global event bus instance:

	// Initialize global bus
	event.InitGlobal(true) // async mode

	// Get global bus
	bus := event.Global()

	// Publish to global bus
	event.Publish(ctx, e)

# Configuration

Event bus settings are configured via config.yaml:

	event:
	  queue_size: 1000
	  workers: 4
	  persistence: true
	  storage: "redis"

# Thread Safety

EventBus is fully thread-safe. All operations use read-write mutex protection
for safe concurrent access from multiple goroutines.

# Usage Example

Complete usage example:

	// Create event bus
	bus := event.NewEventBus(true) // async

	// Subscribe to task events
	bus.Subscribe(event.EventTaskCompleted, func(ctx context.Context, e event.Event) error {
	    log.Printf("Task %s completed", e.TaskID)
	    // Process completion...
	    return nil
	})

	// Subscribe to error events
	bus.Subscribe(event.EventSystemError, func(ctx context.Context, e event.Event) error {
	    log.Printf("System error: %v", e.Data["error"])
	    // Alert team...
	    return nil
	})

	// Publish events
	bus.Publish(ctx, event.Event{
	    Type:     event.EventTaskCompleted,
	    Source:   "worker-1",
	    Severity: event.SeverityInfo,
	    TaskID:   "task-456",
	})
*/
package event
