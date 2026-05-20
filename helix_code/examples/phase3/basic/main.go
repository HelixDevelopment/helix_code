// Basic Phase 3 Example
// Demonstrates fundamental usage of Session, Memory, Persistence, and Templates

package main

import (
	"context"
	"fmt"
	"log"

	"dev.helix.code/examples/i18n"
	"dev.helix.code/internal/memory"
	"dev.helix.code/internal/persistence"
	"dev.helix.code/internal/session"
	"dev.helix.code/internal/template"
)

func main() {
	ctx := context.Background()
	fmt.Println(i18n.Tr(ctx, "examples_basic_header", nil))

	// Initialize all managers
	sessionMgr := session.NewManager()
	memoryMgr := memory.NewManager()
	templateMgr := template.NewManager()

	// Set up persistence
	store, err := persistence.NewStore("./data")
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	store.SetSessionManager(sessionMgr)
	store.SetMemoryManager(memoryMgr)
	store.SetTemplateManager(templateMgr)

	// Enable auto-save every 5 minutes
	store.EnableAutoSave(300)

	// Load built-in templates
	if err := templateMgr.RegisterBuiltinTemplates(); err != nil {
		log.Fatalf("Failed to load built-in templates: %v", err)
	}

	// Try to restore previous state
	if err := store.Load(); err != nil {
		fmt.Println(i18n.Tr(ctx, "examples_basic_no_previous_state", nil))
	} else {
		fmt.Println(i18n.Tr(ctx, "examples_basic_state_restored", nil))
	}

	// Create a development session
	sess, err := sessionMgr.Create(
		"examples",
		"basic-example",
		"Basic HelixCode example session",
		session.ModeBuilding,
	)
	if err != nil {
		log.Fatalf("Failed to create session: %v", err)
	}
	sess.AddTag("tutorial")
	sess.AddTag("phase3")

	fmt.Printf("Created session: %s\n", sess.Name)

	// Start the session
	if err := sessionMgr.Start(sess.ID); err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}

	fmt.Printf("Started session in %s mode\n\n", sess.Mode)

	// Create a conversation
	conv, err := memoryMgr.CreateConversation("Basic Example Discussion")
	if err != nil {
		log.Fatalf("Failed to create conversation: %v", err)
	}
	conv.SessionID = sess.ID

	fmt.Printf("Created conversation: %s\n", conv.Title)

	// Add some messages
	memoryMgr.AddMessage(conv.ID, memory.NewSystemMessage(
		"You are a helpful AI assistant for HelixCode examples.",
	))

	memoryMgr.AddMessage(conv.ID, memory.NewUserMessage(
		"Help me understand Phase 3 features",
	))

	memoryMgr.AddMessage(conv.ID, memory.NewAssistantMessage(
		"Phase 3 adds sessions, memory, persistence, and templates for better workflow management.",
	))

	fmt.Printf("Added %d messages to conversation\n\n", len(conv.GetMessages()))

	// Use a template to generate code
	code, err := templateMgr.RenderByName("Function", map[string]interface{}{
		"function_name": "HelloWorld",
		"parameters":    "",
		"return_type":   "",
		"body":          `fmt.Println("Hello from Phase 3!")`,
	})

	if err != nil {
		log.Fatalf("Failed to render template: %v", err)
	}

	fmt.Println("Generated code from template:")
	fmt.Println(code)
	fmt.Println()

	// Save the generated code as a message
	memoryMgr.AddMessage(conv.ID, memory.NewAssistantMessage(
		fmt.Sprintf("Here's the code:\n```go\n%s\n```", code),
	))

	// Show statistics
	fmt.Println(i18n.Tr(ctx, "examples_basic_statistics_header", nil))
	fmt.Printf("Sessions: %d\n", sessionMgr.Count())
	fmt.Printf("Conversations: %d\n", len(memoryMgr.GetAll()))
	fmt.Printf("Templates: %d\n", templateMgr.Count())
	fmt.Printf("Total messages: %d\n", memoryMgr.TotalMessages())
	fmt.Println()

	// Save state
	if err := store.Save(); err != nil {
		log.Fatalf("Failed to save state: %v", err)
	}

	fmt.Println(i18n.Tr(ctx, "examples_basic_state_saved", nil))

	// Complete the session
	if err := sessionMgr.Complete(sess.ID); err != nil {
		log.Fatalf("Failed to complete session: %v", err)
	}

	fmt.Printf("\nSession '%s' completed!\n", sess.Name)
	fmt.Printf("Duration: %v\n", sess.EndedAt.Sub(sess.StartedAt))
}
