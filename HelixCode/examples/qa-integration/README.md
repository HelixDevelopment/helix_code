# QA Integration Example

This example demonstrates how to integrate with HelixCode's Quality Assurance API from an external application.

## Prerequisites

- HelixCode server running (default: http://localhost:8080)
- Valid JWT authentication token

## Usage

```bash
export HELIXCODE_URL=http://localhost:8080
export HELIXCODE_TOKEN=your-jwt-token

go run ./examples/qa-integration
```

## What it demonstrates

1. **Start Session** — Creates a new QA session with specified platforms and test banks
2. **Poll Status** — Periodically checks session status and progress
3. **Retrieve Report** — Fetches the final report in markdown format
4. **List Sessions** — Retrieves all QA sessions for overview

## API Client

The example uses `dev.helix.code/internal/server.Client` which provides:

- `StartQASession(req StartSessionRequest) (*SessionState, error)`
- `GetQASession(id string) (*SessionState, error)`
- `GetReport(sessionID, format string) ([]byte, error)`
- `ListQASessions() ([]*SessionState, error)`
- `CancelQASession(sessionID string) error`
- `CaptureScreenshot(sessionID, platform string, base64 bool) ([]byte, map[string]string, error)`
