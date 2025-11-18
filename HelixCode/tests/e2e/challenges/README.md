# HelixCode Challenge Testing Framework

A comprehensive testing framework for validating HelixCode's ability to generate complete, working software projects from prompts.

## Overview

The Challenge Testing Framework tests HelixCode by having it implement real-world projects ("challenges") and then rigorously validating the generated code for:

- **Completeness**: No placeholders, TODOs, or empty implementations
- **Compilation**: Generated code compiles successfully
- **Functionality**: Tests pass and applications run
- **Quality**: Proper structure, error handling, and best practices

## Architecture

```
challenges/
├── types.go              # Core type definitions
├── validator.go          # Code validation logic
├── executor.go           # Challenge execution engine
├── manager.go            # Challenge and batch management
├── definitions/          # Challenge specifications (JSON)
│   ├── notes-project.json
│   ├── url-shortener.json
│   └── cli-task-manager.json
├── cmd/
│   └── runner/
│       └── main.go       # CLI test runner
├── docker-compose-workers.yml  # Distributed worker setup
└── README.md            # This file
```

## Quick Start

### 1. List Available Challenges

```bash
cd tests/e2e/challenges
go run cmd/runner/main.go -list
```

### 2. Run a Single Challenge

```bash
# Run the Notes Project challenge with CLI interface and Ollama
go run cmd/runner/main.go \
  -challenge notes-project-001 \
  -interfaces cli \
  -providers ollama \
  -models llama2
```

### 3. Run Full Test Suite

```bash
# Run all challenges across all interfaces and providers
go run cmd/runner/main.go \
  -interfaces cli,tui,rest \
  -distributions single,worker_2 \
  -providers ollama,openai \
  -models llama2,gpt-4 \
  -batch-name "full-suite" \
  -export-report ./results/full-suite-report.json
```

## Challenge Definitions

Challenges are defined in JSON format in the `definitions/` directory:

```json
{
  "id": "notes-project-001",
  "name": "Simple Notes Application",
  "description": "Build a complete notes application with CRUD operations",
  "type": "crud",
  "language": "go",
  "tags": ["crud", "web", "api", "database"],
  "timeout": "30m",
  "priority": 1,
  "prompt": "Create a simple notes application...",
  "requirements": {
    "compilation_check": true,
    "tests_pass": true,
    "run_check": true,
    "has_tests": true,
    "has_readme": true,
    "has_dockerfile": true
  }
}
```

## Validation Checks

Each challenge execution is validated for:

### 1. Directory Structure
- ✅ Result directory exists and has content
- ✅ Required files and directories present
- ✅ Minimum file count met

### 2. Code Quality
- ✅ No TODO, FIXME, or placeholder comments
- ✅ No empty function implementations
- ✅ Proper package structure
- ✅ Required files (README, Dockerfile, etc.)

### 3. Compilation
- ✅ Code compiles without errors
- ✅ Dependencies resolve correctly
- ✅ Build configurations valid

### 4. Testing
- ✅ Tests exist and are comprehensive
- ✅ All tests pass
- ✅ Coverage meets minimums

### 5. Functionality
- ✅ Application starts without errors
- ✅ Basic functionality works
- ✅ Proper error handling

## Testing Modes

### Single Instance
```bash
go run cmd/runner/main.go -distributions single
```
Tests with a standalone HelixCode instance.

### Distributed Workers (2 Workers)
```bash
# Start workers
docker-compose -f docker-compose-workers.yml up -d --scale helixcode-worker=2

# Run tests
go run cmd/runner/main.go -distributions worker_2
```

### Distributed Workers (5 Workers)
```bash
docker-compose -f docker-compose-workers.yml up -d --scale helixcode-worker=5
go run cmd/runner/main.go -distributions worker_5
```

### Distributed Workers (10 Workers)
```bash
docker-compose -f docker-compose-workers.yml up -d --scale helixcode-worker=10
go run cmd/runner/main.go -distributions worker_10
```

## Interface Testing

### CLI Interface
```bash
go run cmd/runner/main.go -interfaces cli
```
Uses the `helix` bash script to send commands.

### TUI Interface
```bash
go run cmd/runner/main.go -interfaces tui
```
Tests terminal UI interactions.

### REST API
```bash
go run cmd/runner/main.go -interfaces rest
```
Sends requests to HelixCode REST API.

### WebSocket
```bash
go run cmd/runner/main.go -interfaces websocket
```
Uses WebSocket connections for real-time interaction.

## LLM Provider Testing

Test with any combination of providers:

```bash
go run cmd/runner/main.go \
  -providers "ollama,openai,anthropic,gemini" \
  -models "llama2,gpt-4,claude-3-sonnet,gemini-pro"
```

Supported providers:
- **Local**: Ollama, Llama.cpp, vLLM, LocalAI
- **Cloud**: OpenAI, Anthropic, Gemini, Mistral, Qwen, Groq
- **Enterprise**: Azure OpenAI, AWS Bedrock, Vertex AI
- **Aggregators**: OpenRouter

## Results and Logging

### Directory Structure
```
test-results/
├── challenges/
│   └── notes-project-001/
│       └── cli_ollama_llama2_20250118_150405_abc123/
│           ├── main.go
│           ├── go.mod
│           ├── ...
│           └── execution-metadata.json
├── logs/
│   └── abc123/
│       ├── execution.log         # Main execution log
│       ├── requests.log           # LLM requests/responses
│       └── validation.log         # Validation results
└── state/
    ├── challenges.json
    ├── executions.json
    └── batches.json
```

### Execution Logs
Every request, response, and action is logged:

```
[2025-01-18 15:04:05.123] Starting challenge execution
[2025-01-18 15:04:05.124] Challenge: Simple Notes Application (notes-project-001)
[2025-01-18 15:04:05.125] Interface: cli
[2025-01-18 15:04:05.126] Provider: ollama
[2025-01-18 15:04:05.127] Model: llama2
...
```

### Request Logs
Full LLM request/response logging:

```
=== REQUEST [2025-01-18 15:04:10.000] CLI ===
{
  "prompt": "Create a simple notes application...",
  "provider": "ollama",
  "model": "llama2",
  "output": "/path/to/results"
}
=== END REQUEST ===
```

### Validation Logs
Detailed validation results:

```
✓ PASS: directory_exists
  Message: Directory exists with 23 entries

✓ PASS: no_placeholders
  Message: No placeholders found

✓ PASS: compilation
  Message: Compilation successful (took 2.3s)

✓ PASS: tests_pass
  Message: Tests passed (took 5.1s)
```

## Batch Reports

Export comprehensive JSON reports:

```bash
go run cmd/runner/main.go \
  -export-report ./results/batch-report.json
```

Report includes:
- Batch configuration and summary
- All execution details
- Validation results
- Metrics (tokens, LOC, files, etc.)
- Timing information

## Command Line Options

```
Usage of runner:
  -batch-desc string
        Description for the batch (default "Challenge test batch")
  -batch-name string
        Name for the batch (default "challenge-test-run")
  -challenge string
        Specific challenge ID to run
  -challenges-dir string
        Directory containing challenge definitions (default "./definitions")
  -distributions string
        Comma-separated list of distributions (default "single")
  -export-report string
        Export batch report to file
  -interfaces string
        Comma-separated list of interfaces (default "cli")
  -list
        List all available challenges
  -logs-dir string
        Directory to store logs (default "./test-results/logs")
  -max-concurrent int
        Maximum concurrent executions (default 3)
  -models string
        Comma-separated list of models (default "llama2")
  -providers string
        Comma-separated list of LLM providers (default "ollama")
  -results-dir string
        Directory to store results (default "./test-results/challenges")
  -save-state
        Save execution state to disk (default true)
  -state-dir string
        Directory to save state (default "./test-results/state")
  -timeout duration
        Default timeout for challenges (default 45m0s)
  -verbose
        Enable verbose logging
```

## Creating New Challenges

1. Create a JSON file in `definitions/`:

```json
{
  "id": "my-challenge-001",
  "name": "My Challenge",
  "description": "Build something cool",
  "type": "api",
  "language": "go",
  "prompt": "Detailed prompt here...",
  "requirements": {
    "compilation_check": true,
    "tests_pass": true,
    "has_tests": true
  }
}
```

2. Run the challenge:

```bash
go run cmd/runner/main.go -challenge my-challenge-001
```

## Integration with CI/CD

```yaml
# .github/workflows/challenge-tests.yml
name: Challenge Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2

      - name: Setup Go
        uses: actions/setup-go@v2
        with:
          go-version: '1.24'

      - name: Run Challenge Tests
        run: |
          cd tests/e2e/challenges
          go run cmd/runner/main.go \
            -interfaces cli,rest \
            -providers ollama \
            -models llama2 \
            -export-report ./results/ci-report.json

      - name: Upload Results
        uses: actions/upload-artifact@v2
        with:
          name: challenge-results
          path: tests/e2e/challenges/test-results/
```

## Best Practices

1. **Start Small**: Begin with single challenge, single interface
2. **Use Verbose Logging**: Enable `-verbose` during development
3. **Check Logs**: Always review logs for failed challenges
4. **Save State**: Keep `-save-state=true` for debugging
5. **Export Reports**: Generate reports for analysis
6. **Distributed Testing**: Test with workers to find concurrency issues

## Troubleshooting

### Challenge Fails with "No placeholders found" but code has TODOs
- Check validation log for exact matches
- Review `validator.go` placeholder patterns

### Compilation fails
- Check `compilation` validation in logs
- Review error messages in validation.log
- Verify language toolchain is available

### Tests don't run
- Ensure test files are generated
- Check test command in `validator.go`
- Review test execution logs

### Docker workers don't connect
- Check docker-compose logs: `docker-compose -f docker-compose-workers.yml logs`
- Verify network connectivity
- Check worker registration in HelixCode server

## Contributing

To add new validation checks:

1. Add check function to `validator.go`
2. Call from `ValidateAll()`
3. Update documentation

To add new challenge types:

1. Add type to `ChallengeType` in `types.go`
2. Create challenge definition JSON
3. Test with runner

## License

Part of the HelixCode project.
