package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Duration is a time.Duration that can be unmarshaled from JSON strings like "30m"
type Duration time.Duration

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	parsed, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}

	*d = Duration(parsed)
	return nil
}

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// ToDuration converts to time.Duration
func (d Duration) ToDuration() time.Duration {
	return time.Duration(d)
}

// ChallengeType defines the type of challenge
type ChallengeType string

const (
	ChallengeTypeCLI          ChallengeType = "cli"          // Command-line application
	ChallengeTypeWebApp       ChallengeType = "webapp"       // Web application
	ChallengeTypeAPI          ChallengeType = "api"          // REST API
	ChallengeTypeMicroservice ChallengeType = "microservice" // Microservice
	ChallengeTypeCRUD         ChallengeType = "crud"         // CRUD application
	ChallengeTypeLibrary      ChallengeType = "library"      // Library/package
	ChallengeTypeFullStack    ChallengeType = "fullstack"    // Full-stack application
	ChallengeTypeBot          ChallengeType = "bot"          // Bot/automation
	ChallengeTypeGame         ChallengeType = "game"         // Game
	ChallengeTypeML           ChallengeType = "ml"           // Machine learning
)

// ChallengeInterface defines which HelixCode interface to use
type ChallengeInterface string

const (
	InterfaceCLI       ChallengeInterface = "cli"
	InterfaceTUI       ChallengeInterface = "tui"
	InterfaceREST      ChallengeInterface = "rest"
	InterfaceWebSocket ChallengeInterface = "websocket"
	InterfaceDesktop   ChallengeInterface = "desktop"
)

// ChallengeDistribution defines the distribution mode
type ChallengeDistribution string

const (
	DistributionSingle   ChallengeDistribution = "single"    // Single instance
	DistributionWorker2  ChallengeDistribution = "worker_2"  // 2 workers
	DistributionWorker5  ChallengeDistribution = "worker_5"  // 5 workers
	DistributionWorker10 ChallengeDistribution = "worker_10" // 10 workers
)

// LLMProviderType defines the LLM provider to use
type LLMProviderType string

const (
	ProviderOllama      LLMProviderType = "ollama"
	ProviderLlamaCpp    LLMProviderType = "llamacpp"
	ProviderVLLM        LLMProviderType = "vllm"
	ProviderLocalAI     LLMProviderType = "localai"
	ProviderOpenAI      LLMProviderType = "openai"
	ProviderAnthropic   LLMProviderType = "anthropic"
	ProviderGemini      LLMProviderType = "gemini"
	ProviderMistral     LLMProviderType = "mistral"
	ProviderQwen        LLMProviderType = "qwen"
	ProviderXAI         LLMProviderType = "xai"
	ProviderGroq        LLMProviderType = "groq"
	ProviderAzure       LLMProviderType = "azure"
	ProviderBedrock     LLMProviderType = "bedrock"
	ProviderVertexAI    LLMProviderType = "vertexai"
	ProviderOpenRouter  LLMProviderType = "openrouter"
	ProviderCohere      LLMProviderType = "cohere"
	ProviderDeepSeek    LLMProviderType = "deepseek"
	ProviderHuggingFace LLMProviderType = "huggingface"
	ProviderOpenCode    LLMProviderType = "opencode"
)

// ValidationCheck defines what to check in the generated code
type ValidationCheck struct {
	Name        string
	Description string
	Check       func(ctx context.Context, resultDir string) error
}

// ChallengeSpec defines a challenge specification
type ChallengeSpec struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        ChallengeType `json:"type"`
	Prompt      string        `json:"prompt"`      // The prompt to send to HelixCode
	PromptFile  string        `json:"prompt_file"` // Path to file containing the prompt
	Language    string        `json:"language"`    // Target language (go, python, javascript, etc.)
	Tags        []string      `json:"tags"`
	Timeout     Duration      `json:"timeout"`
	Priority    int           `json:"priority"`

	// Requirements define what should be generated
	Requirements struct {
		Files            []string `json:"files"`             // Expected files
		Directories      []string `json:"directories"`       // Expected directories
		MinFiles         int      `json:"min_files"`         // Minimum number of files
		HasTests         bool     `json:"has_tests"`         // Should have test files
		HasReadme        bool     `json:"has_readme"`        // Should have README
		HasDockerfile    bool     `json:"has_dockerfile"`    // Should have Dockerfile
		HasCI            bool     `json:"has_ci"`            // Should have CI configuration
		CompilationCheck bool     `json:"compilation_check"` // Should compile successfully
		TestsPass        bool     `json:"tests_pass"`        // Tests should pass
		RunCheck         bool     `json:"run_check"`         // Should run successfully
	} `json:"requirements"`

	// Validation checks
	Validations []ValidationCheck `json:"-"`

	// Metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ChallengeExecution represents a single execution of a challenge
type ChallengeExecution struct {
	ID            string                 `json:"id"`
	ChallengeID   string                 `json:"challenge_id"`
	Interface     ChallengeInterface     `json:"interface"`
	Distribution  ChallengeDistribution  `json:"distribution"`
	Provider      LLMProviderType        `json:"provider"`
	Model         string                 `json:"model"`
	StartTime     time.Time              `json:"start_time"`
	EndTime       time.Time              `json:"end_time"`
	Duration      time.Duration          `json:"duration"`
	Status        ExecutionStatus        `json:"status"`
	ResultDir     string                 `json:"result_dir"`     // Where generated code is stored
	LogFile       string                 `json:"log_file"`       // Execution log file
	RequestLog    string                 `json:"request_log"`    // Request/response log
	ValidationLog string                 `json:"validation_log"` // Validation results log
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`

	// Validation results
	ValidationResults []ValidationResult `json:"validation_results"`

	// Metrics
	Metrics ExecutionMetrics `json:"metrics"`
}

// ExecutionStatus defines the status of a challenge execution
type ExecutionStatus string

const (
	StatusQueued           ExecutionStatus = "queued"
	StatusRunning          ExecutionStatus = "running"
	StatusCompleted        ExecutionStatus = "completed"
	StatusFailed           ExecutionStatus = "failed"
	StatusTimeout          ExecutionStatus = "timeout"
	StatusValidationFailed ExecutionStatus = "validation_failed"
)

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	CheckName string    `json:"check_name"`
	Passed    bool      `json:"passed"`
	Message   string    `json:"message,omitempty"`
	Error     string    `json:"error,omitempty"`
	Details   string    `json:"details,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ExecutionMetrics contains metrics about the execution
type ExecutionMetrics struct {
	FilesGenerated    int           `json:"files_generated"`
	LinesOfCode       int           `json:"lines_of_code"`
	Requests          int           `json:"requests"`            // Number of LLM requests
	TokensUsed        int           `json:"tokens_used"`         // Total tokens used
	CompilationTime   time.Duration `json:"compilation_time"`    // Time to compile
	TestExecutionTime time.Duration `json:"test_execution_time"` // Time to run tests
	PlaceholdersFound int           `json:"placeholders_found"`  // Number of TODOs/placeholders
	EmptyFunctions    int           `json:"empty_functions"`     // Number of empty function implementations
	CoveragePercent   float64       `json:"coverage_percent"`    // Test coverage percentage
}

// ChallengeBatch represents a batch of challenge executions
type ChallengeBatch struct {
	ID            string                  `json:"id"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	Challenges    []string                `json:"challenges"` // Challenge IDs
	Interfaces    []ChallengeInterface    `json:"interfaces"`
	Distributions []ChallengeDistribution `json:"distributions"`
	Providers     []LLMProviderType       `json:"providers"`
	Models        []string                `json:"models"`
	StartTime     time.Time               `json:"start_time"`
	EndTime       time.Time               `json:"end_time"`
	Duration      time.Duration           `json:"duration"`
	Status        ExecutionStatus         `json:"status"`
	Executions    []string                `json:"executions"` // Execution IDs
	Summary       BatchSummary            `json:"summary"`
}

// BatchSummary provides summary statistics for a batch
type BatchSummary struct {
	TotalExecutions     int           `json:"total_executions"`
	Completed           int           `json:"completed"`
	Failed              int           `json:"failed"`
	Timeout             int           `json:"timeout"`
	ValidationFailed    int           `json:"validation_failed"`
	SuccessRate         float64       `json:"success_rate"`
	AvgDuration         time.Duration `json:"avg_duration"`
	TotalTokens         int           `json:"total_tokens"`
	TotalFilesGenerated int           `json:"total_files_generated"`
	TotalLOC            int           `json:"total_loc"`
}

// ChallengeConfig defines configuration for challenge execution
type ChallengeConfig struct {
	// HelixCode connection
	HelixCodeHost string `json:"helixcode_host"`
	HelixCodePort int    `json:"helixcode_port"`
	HelixCodeAuth string `json:"helixcode_auth"`

	// Storage
	ResultsBaseDir string `json:"results_base_dir"`
	LogsBaseDir    string `json:"logs_base_dir"`

	// Execution settings
	MaxConcurrent  int           `json:"max_concurrent"`
	DefaultTimeout time.Duration `json:"default_timeout"`
	RetryCount     int           `json:"retry_count"`

	// Validation settings
	ValidateCompilation bool `json:"validate_compilation"`
	ValidateTests       bool `json:"validate_tests"`
	ValidateRun         bool `json:"validate_run"`
	StrictValidation    bool `json:"strict_validation"`

	// Logging
	VerboseLogging         bool `json:"verbose_logging"`
	SaveAllRequests        bool `json:"save_all_requests"`
	SaveAllResponses       bool `json:"save_all_responses"`
	SaveIntermediateStates bool `json:"save_intermediate_states"`
}

// DefaultChallengeConfig returns a default configuration
func DefaultChallengeConfig() *ChallengeConfig {
	return &ChallengeConfig{
		HelixCodeHost:          "localhost",
		HelixCodePort:          8080,
		ResultsBaseDir:         "./test-results/challenges",
		LogsBaseDir:            "./test-results/logs",
		MaxConcurrent:          5,
		DefaultTimeout:         30 * time.Minute,
		RetryCount:             0,
		ValidateCompilation:    true,
		ValidateTests:          true,
		ValidateRun:            false,
		StrictValidation:       false,
		VerboseLogging:         true,
		SaveAllRequests:        true,
		SaveAllResponses:       true,
		SaveIntermediateStates: true,
	}
}
