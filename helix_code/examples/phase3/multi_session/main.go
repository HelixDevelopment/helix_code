// Multi-Session Workflow Example

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev.helix.code/examples/i18n"
	"dev.helix.code/internal/session"
)

func main() {
	ctx := context.Background()
	fmt.Println(i18n.Tr(ctx, "examples_multi_session_header", nil))

	mgr := session.NewManager()

	// Working on multiple features
	authSess, err := mgr.Create("project1", "implement-auth", "api", session.ModeBuilding)
	if err != nil {
		log.Fatal(err)
	}
	authSess.AddTag("auth")

	paymentSess, err := mgr.Create("project1", "implement-payments", "api", session.ModeBuilding)
	if err != nil {
		log.Fatal(err)
	}
	paymentSess.AddTag("payments")

	// Start auth work
	fmt.Println(i18n.Tr(ctx, "examples_multi_session_starting_auth", nil))
	mgr.Start(authSess.ID)
	time.Sleep(100 * time.Millisecond)

	// Switch to payments (urgent)
	fmt.Println(i18n.Tr(ctx, "examples_multi_session_switch_payments", nil))
	mgr.Pause(authSess.ID)
	mgr.Start(paymentSess.ID)
	time.Sleep(100 * time.Millisecond)

	// Complete payments
	fmt.Println(i18n.Tr(ctx, "examples_multi_session_payments_done", nil))
	mgr.Complete(paymentSess.ID)

	// Resume auth
	fmt.Println(i18n.Tr(ctx, "examples_multi_session_resuming_auth", nil))
	mgr.Resume(authSess.ID)
	time.Sleep(100 * time.Millisecond)

	// Complete auth
	mgr.Complete(authSess.ID)

	// Show all sessions
	fmt.Println("\n" + i18n.Tr(ctx, "examples_multi_session_summary_header", nil))
	for _, s := range mgr.GetAll() {
		fmt.Printf("%s (%s): %s\n", s.Name, s.Mode, s.Status)
	}
}
