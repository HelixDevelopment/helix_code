# 2. Core System Architecture - Implementation Details

## 2.1 Component Architecture Implementation

### 2.1.1 LLM Integration Layer - Exact Implementation

#### Interface Definition Requirements:
```go
// Core LLM Provider Interface - Must implement exactly
type LLMProvider interface {
    // Generate must handle context cancellation and streaming
    Generate(ctx context.Context, req GenerationRequest) (*GenerationResponse, error)
    
    // Stream must implement proper backpressure and chunk handling
    Stream(ctx context.Context, req GenerationRequest) (<-chan StreamChunk, error)
    
    // GetCapabilities must return detailed provider-specific capabilities
    GetCapabilities() ProviderCapabilities
    
    // ValidateConfig must perform comprehensive configuration validation
    ValidateConfig() error
    
    // Close must ensure proper resource cleanup
    Close() error
}
```

#### Provider Implementation Patterns:
- **BaseProvider**: Common functionality for all providers
- **LlamaCPPProvider**: Local Llama.cpp integration with hardware optimization
- **OllamaProvider**: Ollama API client with model management
- **OpenRouterProvider**: OpenRouter API integration
- **DeepSeekProvider**: DeepSeek API client
- **QwenProvider**: Qwen API client
- **ClaudeProvider**: Anthropic Claude API
- **GeminiProvider**: Google Gemini API
- **GrokProvider**: xAI Grok API
- **MistralProvider**: Mistral AI API
- **HuggingFaceProvider**: HuggingFace endpoints
- **NvidiaProvider**: NVIDIA NIM API

### 2.1.2 User Interface Layer - Implementation Specifications

#### Terminal UI Component Structure:
```go
type TerminalUI struct {
    app         *tview.Application // Must use tview for terminal UI
    layout      *LayoutManager    // Must implement fixed layout regions
    components  map[string]UIComponent
    eventBus    *EventBus         // Must use event-driven architecture
    state       *UIState          // Must maintain consistent UI state
}
```

#### Required UI Components:
- **HeaderComponent**: ASCII art + project info + system status
- **NavigationComponent**: Mode and project tree with keyboard navigation
- **InputComponent**: Command input with history and auto-completion
- **OutputComponent**: Results with syntax highlighting and pagination
- **StatusComponent**: System status and real-time notifications
- **HelpComponent**: Contextual help system with search

#### Layout Manager Specifications:
- **Header**: Top 3 lines - project name, mode, status indicators
- **Navigation**: Left 20% - project hierarchy and available modes
- **Main**: Center 60% - input/output with proper scrolling
- **Sidebar**: Right 20% - context and active tools
- **Status**: Bottom 2 lines - system metrics and notifications

### 2.1.3 Processing Engine Layer - Implementation Requirements

#### Base Engine Interface:
```go
type ProcessingEngine interface {
    // Initialize must validate all dependencies and setup state
    Initialize(ctx *ProjectContext) error
    
    // Execute must handle cancellation and progress reporting
    Execute(req *ExecutionRequest) (*ExecutionResult, error)
    
    // Validate must perform comprehensive input validation
    Validate() []ValidationError
    
    // Cleanup must ensure proper resource release
    Cleanup() error
    
    // GetProgress must return detailed execution progress
    GetProgress() *ExecutionProgress
}
```

#### Engine Implementations:
- **PlannerEngine**: Project analysis and planning
- **BuilderEngine**: Code generation and modification
- **RefactorerEngine**: Code optimization and restructuring
- **TesterEngine**: Comprehensive testing execution
- **DebuggerEngine**: Issue detection and resolution
- **DesignerEngine**: Design system integration
- **DiagramEngine**: Diagram generation
- **DeploymentEngine**: Deployment automation
- **PortingEngine**: Cross-platform code conversion

## 2.2 Data Architecture - Implementation Specifications

### 2.2.1 Database Schema - Exact Implementation

#### Core Tables Structure:
```sql
-- Projects table must store exactly these fields
CREATE TABLE projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    path TEXT NOT NULL UNIQUE,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Indexes must be created exactly as specified
    CONSTRAINT projects_name_check CHECK (name ~ '^[a-zA-Z0-9_-]+$'),
    CONSTRAINT projects_path_check CHECK (path ~ '^/[a-zA-Z0-9_/-]+$')
);

-- Sessions table must implement exactly this structure
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(255),
    state JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    active_until TIMESTAMPTZ,
    
    -- Session state management
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed', 'failed')),
    
    -- Indexes for performance
    CONSTRAINT sessions_name_check CHECK (name IS NULL OR name ~ '^[a-zA-Z0-9_-]+$')
);

-- Additional required tables:
- requests: Command execution history and results
- models: Model registry with capabilities
- users: User management and preferences
- permissions: Access control and roles
- logs: System and audit logging
- configurations: Hierarchical configuration storage
```

### 2.2.2 Configuration Management - Implementation Requirements

#### Configuration Structure:
```go
type Config struct {
    Global    GlobalConfig    `json:"global" validate:"required"`
    Providers ProviderConfigs `json:"providers" validate:"required,dive"`
    UI        UIConfig        `json:"ui" validate:"required"`
    Security  SecurityConfig  `json:"security" validate:"required"`
    Database  DatabaseConfig  `json:"database" validate:"required"`
}
```

#### Global Configuration Specifications:
```go
type GlobalConfig struct {
    DefaultProvider string            `json:"default_provider" validate:"required,oneof=llama-cpp ollama openrouter deepseek qwen claude gemini grok mistral huggingface nvidia"`
    Theme           string            `json:"theme" validate:"required,oneof=default warm-red blue yellow gold grey white darcula dark-blue violet warm-orange"`
    AutoConfirm     bool              `json:"auto_confirm"`
    MaxWorkers      int               `json:"max_workers" validate:"required,min=1,max=64"`
    LogLevel        string            `json:"log_level" validate:"required,oneof=debug info warn error"`
    DataDir         string            `json:"data_dir" validate:"required,dirpath"`
    CacheDir        string            `json:"cache_dir" validate:"required,dirpath"`
}
```

#### Configuration Loading Process:
1. **Primary Source**: Load from `~/.config/Helix/helix.json`
2. **Environment Overrides**: Apply `HELIX_*` environment variables
3. **Validation**: Validate all configuration values against JSON schema
4. **Defaults**: Set sensible defaults for missing values
5. **Directory Creation**: Create required directories with proper permissions
6. **Backup**: Create backup of existing configuration before modification

#### Hierarchical Configuration Levels:
- **Global**: System-wide settings in `~/.config/Helix/helix.json`
- **Project**: Project-specific settings in `Helix.md` files
- **Module**: Module-specific configurations
- **Session**: Temporary session-specific settings
- **User**: User preferences and customization