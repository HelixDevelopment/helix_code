// Package testutil provides mock servers for testing notification delivery.
//
// Testing notification integrations requires simulating external services.
// This package provides mock HTTP servers that simulate Slack, Telegram,
// Discord, and other notification backends.
//
// # Mock Servers
//
// MockSlackServer simulates Slack webhook endpoints:
//
//	server := testutil.NewMockSlackServer()
//	defer server.Close()
//
//	// Configure notifier to use mock URL
//	notifier := slack.New(server.URL)
//	notifier.Send(message)
//
//	// Verify requests
//	requests := server.GetRequests()
//	assert.Equal(t, 1, len(requests))
//	assert.Equal(t, "test message", requests[0].Text)
//
// MockTelegramServer simulates the Telegram Bot API:
//
//	server := testutil.NewMockTelegramServer()
//	defer server.Close()
//	// Returns proper Telegram API response format
//
// MockDiscordServer simulates Discord webhook endpoints:
//
//	server := testutil.NewMockDiscordServer()
//	defer server.Close()
//
// # Thread Safety
//
// All mock servers are thread-safe and can handle concurrent requests.
// They track all received requests for later inspection.
//
// # Common Operations
//
// All mock servers support:
//   - GetRequests(): Get all received requests
//   - GetRequestCount(): Get number of requests
//   - Reset(): Clear recorded requests
//   - Close(): Shut down the server
package testutil
