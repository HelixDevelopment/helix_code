// Package main demonstrates how to integrate with HelixCode's QA API
// from an external application. This example shows starting a QA session,
// polling for status, and retrieving the final report.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"dev.helix.code/examples/i18n"
	"dev.helix.code/internal/server"
)

func main() {
	ctx := context.Background()
	baseURL := os.Getenv("HELIXCODE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	token := os.Getenv("HELIXCODE_TOKEN")
	if token == "" {
		log.Fatal(i18n.Tr(ctx, "examples_qa_integration_token_required", nil))
	}

	client := server.NewClient(baseURL)
	client.SetAuthToken(token)

	// 1. Start a QA session
	fmt.Println(i18n.Tr(ctx, "examples_qa_integration_starting_session", nil))
	req := server.StartSessionRequest{
		Platforms:        []string{"web"},
		Banks:            []string{"./banks/api", "./banks/web"},
		Autonomous:       false,
		CoverageTarget:   0.9,
		CuriosityEnabled: true,
	}

	session, err := client.StartQASession(req)
	if err != nil {
		log.Fatalf("Failed to start session: %v", err)
	}
	fmt.Printf("Session started: %s (status: %s)\n", session.ID, session.Status)

	// 2. Poll for completion
	fmt.Println(i18n.Tr(ctx, "examples_qa_integration_waiting", nil))
	for {
		s, err := client.GetQASession(session.ID)
		if err != nil {
			log.Fatalf("Failed to get status: %v", err)
		}
		fmt.Printf("  Status: %s | Phase: %s | Progress: %.0f%%\n",
			s.Status, s.Phase, s.PhaseProgress*100)

		if s.Status == "completed" || s.Status == "failed" || s.Status == "cancelled" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	// 3. Retrieve report
	fmt.Println(i18n.Tr(ctx, "examples_qa_integration_fetching_report", nil))
	report, err := client.GetReport(session.ID, "markdown")
	if err != nil {
		log.Printf("Report not available: %v", err)
	} else {
		fmt.Printf("Report size: %d bytes\n", len(report))
	}

	// 4. List all sessions
	fmt.Println(i18n.Tr(ctx, "examples_qa_integration_listing_sessions", nil))
	sessions, err := client.ListQASessions()
	if err != nil {
		log.Fatalf("Failed to list sessions: %v", err)
	}
	fmt.Printf("Total sessions: %d\n", len(sessions))
	for _, s := range sessions {
		fmt.Printf("  - %s: %s (%s)\n", s.ID[:8], s.Status, s.Phase)
	}
}
