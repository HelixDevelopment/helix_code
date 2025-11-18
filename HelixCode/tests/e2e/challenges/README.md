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

Each challenge execution is validated through **6 comprehensive layers**:

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

### 6. Runtime Validation with Diverse Data
- ✅ **Binary executes successfully**
- ✅ **Diverse data testing** - Multiple inputs tested
- ✅ **Expected output format** verified
- ✅ **Edge cases** handled correctly
- ✅ **All features** functional

## Diverse Data Testing

**Critical Requirement**: All generated applications must be tested with a **variety of data sets** to ensure real-world usability, not just basic examples.

### Why Diverse Data Testing?

During development, we discovered that an ASCII art generator passed all validations but only worked for "HELLO" - any other input produced empty or garbage output. This highlighted the need for testing with diverse inputs across all challenges.

### ASCII Art Generator - Diverse Testing

Tests with **5 different inputs** across **4 different styles**:

```go
testCases := []struct {
    input string
    style string
    name  string
}{
    {"HELLO", "banner", "basic text"},
    {"DIGITAL", "standard", "different word"},
    {"VASIC", "block", "another word"},
    {"TEST123", "standard", "alphanumeric"},
    {"ABC", "shadow", "short text"},
}
```

**Validation**:
- Each input must produce non-empty output
- Output must be formatted correctly
- All styles must work for all characters
- No crashes or errors for any input

### Tic-Tac-Toe - Game Logic Testing

Tests **5 game scenarios** to verify actual gameplay:

```go
gameTests := []struct {
    name   string
    moves  string
    desc   string
}{
    {"single_move", "5\nq\n", "Place move at center (5) then quit"},
    {"corner_move", "1\nq\n", "Place move at corner (1) then quit"},
    {"multiple_moves", "1\n5\n9\nq\n", "Place moves at corners and center"},
    {"full_row", "1\n2\n3\nq\n", "Attempt to fill top row"},
    {"diagonal", "1\n5\n9\nq\n", "Test diagonal placement"},
}
```

**Validation**:
- Game accepts input and displays board
- No crashes on various move patterns
- Proper input validation
- Game state updates correctly

### Notes API - Endpoint Coverage

Tests **12 diverse API scenarios**:

1. POST /notes with valid data (title, content, tags)
2. POST /notes with minimal data (title only)
3. POST /notes with maximum length content
4. POST /notes with special characters in title
5. GET /notes with empty database
6. GET /notes with multiple notes
7. GET /notes/:id with valid ID
8. GET /notes/:id with invalid ID
9. PUT /notes/:id with updates
10. DELETE /notes/:id with valid ID
11. GET /notes with search query
12. GET /notes with tag filter

**Validation**:
- All endpoints accessible
- Proper HTTP status codes
- JSON response formatting
- Error handling for invalid inputs
- CRUD operations work correctly

### Implementing Diverse Testing

When adding new challenges, ensure runtime validation includes:

1. **Multiple inputs**: Test at least 5 different input variations
2. **Edge cases**: Empty, maximum length, special characters
3. **Different features**: Test all major features, not just one
4. **Error cases**: Test with invalid inputs
5. **Real-world scenarios**: Use realistic data, not just test data

Example runtime validation implementation:

```go
func (v *RuntimeValidator) ValidateASCIIArtGenerator(ctx context.Context, resultDir string) ValidationResult {
    // Test 1: Basic execution
    exePath := filepath.Join(resultDir, "ascii-art")

    // Test 2: Diverse data testing
    testCases := []struct {
        input string
        style string
        name  string
    }{
        {"HELLO", "banner", "basic text"},
        {"DIGITAL", "standard", "different word"},
        {"VASIC", "block", "another word"},
        {"TEST123", "standard", "alphanumeric"},
        {"ABC", "shadow", "short text"},
    }

    allPassed := true
    var failureDetails []string

    for _, tc := range testCases {
        args := []string{tc.input}
        if tc.style != "" {
            args = append([]string{"-s", tc.style}, args...)
        }

        result := v.runCommand(ctx, exePath, args, "", 5*time.Second)
        if result.Error != "" {
            allPassed = false
            failureDetails = append(failureDetails,
                fmt.Sprintf("%s (%s style): failed - %s",
                    tc.name, tc.style, result.Error))
        } else if len(result.Stdout) < 10 {
            allPassed = false
            failureDetails = append(failureDetails,
                fmt.Sprintf("%s (%s style): no/insufficient output",
                    tc.name, tc.style))
        }
    }

    if !allPassed {
        return ValidationResult{
            CheckName: "diverse_data_testing",
            Passed:    false,
            Message:   "Failed diverse data tests",
            Details:   strings.Join(failureDetails, "\n"),
        }
    }

    return ValidationResult{
        CheckName: "diverse_data_testing",
        Passed:    true,
        Message:   fmt.Sprintf("All %d diverse data tests passed", len(testCases)),
    }
}
```

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

### Supported Providers

#### Local Providers (No API Key Required)
- **Ollama**: Local inference server with models like llama2, codellama, mistral
- **Llama.cpp**: Direct llama.cpp integration
- **vLLM**: High-performance inference server
- **LocalAI**: OpenAI-compatible local API

#### Cloud Providers (Requires API Keys)
- **xAI (Grok)**: Advanced reasoning with Grok models
  - Models: `grok-beta`, `grok-vision-beta`
  - Endpoint: `https://api.x.ai/v1`
  - Rate limits: 60 req/min, 100K tokens/min

- **OpenAI**: Industry-leading GPT models
  - Models: `gpt-4-turbo-preview`, `gpt-4`, `gpt-3.5-turbo`, `gpt-4-32k`
  - Rate limits: 3500 req/min, 90K tokens/min

- **Anthropic**: Claude family models
  - Models: `claude-3-opus-20240229`, `claude-3-sonnet-20240229`, `claude-3-haiku-20240307`, `claude-2.1`
  - Rate limits: 1000 req/min, 100K tokens/min

- **Groq**: Ultra-fast inference with open models
  - Models: `llama-3.1-405b-reasoning`, `llama-3.1-70b-versatile`, `llama-3.1-8b-instant`, `mixtral-8x7b-32768`
  - Rate limits: 30 req/min, 14.4K tokens/min

- **Google Gemini**: Multimodal AI models
  - Models: `gemini-pro`, `gemini-pro-vision`, `gemini-ultra`

- **Mistral AI**: European AI models
  - Models: `mistral-large-latest`, `mistral-medium-latest`, `mistral-small-latest`

- **Cohere**: Enterprise-grade language models
  - Models: `command-r-plus`, `command-r`, `command`

- **DeepSeek**: Chinese AI models specialized for coding and reasoning
  - Models: `deepseek-chat`, `deepseek-coder`, `deepseek-reasoner`
  - Rate Limits: 60 requests/min, 100K tokens/min
  - Endpoint: `https://api.deepseek.com/v1`

- **Azure OpenAI**: Enterprise OpenAI deployment
  - Uses deployment names configured in Azure

- **AWS Bedrock**: AWS-hosted foundation models
- **Vertex AI**: Google Cloud AI platform

### API Key Configuration

For cloud providers, you need to configure API keys:

1. **Copy the example configuration**:
   ```bash
   cd tests/e2e/challenges
   cp api-keys.yaml.example api-keys.yaml
   ```

2. **Add your API keys** to `api-keys.yaml`:
   ```yaml
   # xAI (Grok) API
   xai:
     api_key: "your-xai-api-key-here"

   # OpenAI API
   openai:
     api_key: "sk-..."
     organization: "org-..."  # Optional

   # Anthropic Claude API
   anthropic:
     api_key: "sk-ant-..."

   # Groq API
   groq:
     api_key: "gsk_..."

   # Google Gemini API
   gemini:
     api_key: "..."

   # Mistral AI API
   mistral:
     api_key: "..."

   # Cohere API
   cohere:
     api_key: "..."

   # DeepSeek API
   deepseek:
     api_key: "sk-..."
   ```

3. **Security**: The `api-keys.yaml` file is automatically gitignored and will never be committed.

### API Key Security Features

- **Automatic Gitignore**: API keys file is protected from version control
- **Key Masking**: API keys are masked in logs (shows only first 4 and last 4 characters)
  ```go
  maskedKey := MaskAPIKey("a977c8417a45457a83a897de82e4215b.lnHprFLE4TikOOjX")
  // Returns: "a977...OjX"
  ```
- **Sanitization**: All log output is sanitized to remove any leaked keys
- **Graceful Fallback**: If API keys fail to load, system continues with local providers only

### Using API Keys Programmatically

```go
// Load API keys from file
apiKeys, err := LoadAPIKeys("")  // Uses default "api-keys.yaml"
if err != nil {
    log.Printf("Warning: Failed to load API keys: %v", err)
    apiKeys = &APIKeys{}  // Continue with empty config
}

// Create LLM client
client := NewLLMClient(ProviderXAI, "grok-beta", apiKeys)

// Make completion request
resp, err := client.Complete(ctx, &CompletionRequest{
    Prompt: "Write a hello world program in Go",
    SystemPrompt: "You are an expert Go programmer",
    MaxTokens: 1000,
    Temperature: 0.7,
})

if err != nil {
    log.Fatal(err)
}

fmt.Println(resp.Content)
fmt.Printf("Tokens used: %d\n", resp.TokensUsed)
fmt.Printf("Finish reason: %s\n", resp.FinishReason)
```

### Provider Helper Functions

```go
// Get all supported models for a provider
models := GetSupportedModels(ProviderXAI)
// Returns: ["grok-beta", "grok-vision-beta"]

// Get default model
defaultModel := GetDefaultModel(ProviderOpenAI)
// Returns: "gpt-4-turbo-preview"

// Get API endpoint
endpoint := GetProviderAPIEndpoint(ProviderAnthropic)
// Returns: "https://api.anthropic.com/v1"

// Get rate limits
limits := GetProviderRateLimits(ProviderGroq)
// Returns: map[string]int{"requests_per_minute": 30, "tokens_per_minute": 14400}

// Check if provider requires API key
requiresKey := IsCloudProvider(ProviderXAI)
// Returns: true

requiresKey = IsCloudProvider(ProviderOllama)
// Returns: false
```

### Dynamic Model Discovery

The framework supports dynamic model discovery from provider APIs, automatically fetching available models instead of relying on hardcoded lists:

```go
// Load API keys
apiKeys, err := LoadAPIKeys("")
if err != nil {
    log.Fatal(err)
}

// Get models with dynamic discovery (falls back to static list if discovery fails)
models := GetSupportedModelsWithDiscovery(ProviderOpenAI, apiKeys)
// Dynamically fetches from OpenAI API, returns latest models

// Get detailed model information
modelDetails, err := GetModelDetails(ProviderDeepSeek, apiKeys)
if err != nil {
    log.Fatal(err)
}

for _, model := range modelDetails {
    fmt.Printf("Model: %s\n", model.ID)
    fmt.Printf("  Owned by: %s\n", model.OwnedBy)
    fmt.Printf("  Created: %d\n", model.Created)
}
```

**Supported Providers for Dynamic Discovery**:
- ✅ OpenAI - `/v1/models` endpoint
- ✅ xAI (Grok) - `/v1/models` endpoint
- ✅ DeepSeek - `/v1/models` endpoint
- ✅ Groq - `/openai/v1/models` endpoint
- ✅ Ollama - `/api/tags` endpoint (local)
- ⚠️ Anthropic - Uses hardcoded list (no models API)
- ❌ Others - Fall back to static lists

**Features**:
- **Automatic Caching**: Models cached for 24 hours to reduce API calls
- **Graceful Fallback**: Falls back to static lists if API discovery fails
- **No API Key Required for Fallback**: Works without API keys by using static lists
- **Thread-Safe**: Cache operations are protected with mutex locks

**Example: Clear Cache**:
```go
discovery := NewModelDiscovery(apiKeys)

// Clear cache for specific provider
provider := ProviderOpenAI
discovery.ClearCache(&provider)

// Clear all caches
discovery.ClearCache(nil)
```

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
