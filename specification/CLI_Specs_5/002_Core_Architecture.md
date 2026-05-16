# 2. Core System Architecture - Implementation Details

## 2.1 Component Architecture Implementation

### 2.1.1 Distributed Worker Architecture - Exact Implementation

#### Worker Pool Management Structure:
```go
type WorkerPoolManager struct {
    coordinator *WorkerCoordinator
    workers     map[string]*WorkerNode
    scheduler   *TaskScheduler
    discovery   *WorkerDiscovery
    
    // Implementation requirements:
    // - SSH-based worker connection and management
    // - Automatic Helix CLI installation on worker nodes
    // - Dynamic resource allocation and load balancing
    // - Health monitoring and automatic recovery
    // - Cross-platform worker compatibility
}

type WorkerNode struct {
    ID          string            `json:"id" validate:"required"`
    Hostname    string            `json:"hostname" validate:"required"`
    SSHConfig   SSHConfig         `json:"ssh_config" validate:"required"`
    Capabilities NodeCapabilities `json:"capabilities" validate:"required"`
    Status      NodeStatus        `json:"status" validate:"required"`
    Resources   NodeResources     `json:"resources" validate:"required"`
    Load        NodeLoad          `json:"load" validate:"required"`
}

type SSHConfig struct {
    Host        string `json:"host" validate:"required,hostname|ip"`
    Port        int    `json:"port" validate:"required,min=1,max=65535"`
    Username    string `json:"username" validate:"required"`
    KeyPath     string `json:"key_path" validate:"omitempty,filepath"`
    Password    string `json:"password" validate:"omitempty"`
    KnownHosts  string `json:"known_hosts" validate:"omitempty,filepath"`
}
```

#### Worker Configuration in helix.json:
```json
{
  "workers": {
    "enabled": true,
    "pool": {
      "worker-node-1": {
        "host": "192.168.1.100",
        "port": 22,
        "username": "helix",
        "key_path": "~/.ssh/id_rsa",
        "capabilities": ["llm-inference", "code-generation", "testing"]
      },
      "worker-node-2": {
        "host": "192.168.1.101",
        "port": 22,
        "username": "helix",
        "key_path": "~/.ssh/id_rsa",
        "capabilities": ["model-training", "data-processing"]
      }
    },
    "auto_install": true,
    "health_check_interval": 30,
    "max_concurrent_tasks": 10
  }
}
```

#### Worker Discovery and Connection Process:
```go
func (w *WorkerPoolManager) ConnectWorker(config SSHConfig) error {
    // Step 1: SSH connection establishment
    client, err := w.establishSSHConnection(config)
    if err != nil {
        return fmt.Errorf("SSH connection failed: %w", err)
    }
    
    // Step 2: Helix CLI installation check
    installed, err := w.checkHelixInstallation(client)
    if err != nil {
        return fmt.Errorf("installation check failed: %w", err)
    }
    
    // Step 3: Auto-install if needed
    if !installed && w.config.AutoInstall {
        if err := w.installHelixCLI(client); err != nil {
            return fmt.Errorf("installation failed: %w", err)
        }
    }
    
    // Step 4: Capability discovery
    capabilities, err := w.discoverCapabilities(client)
    if err != nil {
        return fmt.Errorf("capability discovery failed: %w", err)
    }
    
    // Step 5: Worker registration
    worker := &WorkerNode{
        ID:          generateWorkerID(config.Host),
        Hostname:    config.Host,
        SSHConfig:   config,
        Capabilities: capabilities,
        Status:      NodeStatusActive,
        Resources:   w.discoverResources(client),
        Load:        NodeLoad{CurrentTasks: 0, CPUUsage: 0, MemoryUsage: 0},
    }
    
    w.workers[worker.ID] = worker
    return nil
}
```

### 2.1.2 LLM Integration Layer - Enhanced Implementation

#### Advanced Provider Interface:
```go
// Enhanced LLM Provider Interface with Tool Calling
type LLMProvider interface {
    // Core generation capabilities
    Generate(ctx context.Context, req GenerationRequest) (*GenerationResponse, error)
    Stream(ctx context.Context, req GenerationRequest) (<-chan StreamChunk, error)
    
    // Tool calling and reasoning support
    GenerateWithTools(ctx context.Context, req ToolGenerationRequest) (*ToolGenerationResponse, error)
    StreamWithTools(ctx context.Context, req ToolGenerationRequest) (<-chan ToolStreamChunk, error)
    
    // Thinking and reasoning capabilities
    GenerateWithReasoning(ctx context.Context, req ReasoningRequest) (*ReasoningResponse, error)
    
    // Provider capabilities
    GetCapabilities() ProviderCapabilities
    ValidateConfig() error
    Close() error
}

type ToolGenerationRequest struct {
    Messages    []Message           `json:"messages" validate:"required,dive"`
    Tools       []ToolDefinition    `json:"tools" validate:"required,dive"`
    ToolChoice  ToolChoice          `json:"tool_choice"`
    MaxTokens   int                 `json:"max_tokens" validate:"required,min=1,max=32000"`
    Temperature float64             `json:"temperature" validate:"min=0,max=2"`
    Reasoning   *ReasoningConfig    `json:"reasoning"`
}

type ReasoningRequest struct {
    Prompt      string              `json:"prompt" validate:"required"`
    Reasoning   ReasoningConfig     `json:"reasoning" validate:"required"`
    Tools       []ToolDefinition    `json:"tools"`
    MaxTokens   int                 `json:"max_tokens" validate:"required,min=1,max=32000"`
}
```

#### Provider Implementation Patterns:
- **BaseProvider**: Common functionality for all providers
- **LlamaCPPProvider**: Local Llama.cpp integration with hardware optimization
- **OllamaProvider**: Ollama API client with model management
- **OpenRouterProvider**: OpenRouter API integration
- **DeepSeekProvider**: DeepSeek API client with reasoning support
- **QwenProvider**: Qwen API client with tool calling
- **ClaudeProvider**: Anthropic Claude API with thinking capabilities
- **GeminiProvider**: Google Gemini API
- **GrokProvider**: xAI Grok API
- **MistralProvider**: Mistral AI API
- **HuggingFaceProvider**: HuggingFace endpoints
- **NvidiaProvider**: NVIDIA NIM API

### 2.1.3 User Interface Layer - Multi-Client Implementation

#### Multi-Client Interface Structure:
```go
type ClientInterface interface {
    // Core interface methods
    Start() error
    Stop() error
    HandleRequest(req ClientRequest) (*ClientResponse, error)
    
    // Client-specific capabilities
    GetClientType() ClientType
    SupportsStreaming() bool
    SupportsInteractive() bool
}

// Client implementations
type TerminalUIClient struct {
    app         *tview.Application
    layout      *LayoutManager
    components  map[string]UIComponent
    eventBus    *EventBus
}

type RESTAPIClient struct {
    server      *http.Server
    router      *mux.Router
    auth        *AuthManager
    rateLimit   *RateLimiter
}

type MobileClient struct {
    platform    MobilePlatform
    bridge      *MobileBridge
    push        *PushNotificationManager
}

type CLIClient struct {
    parser      *cobra.Command
    executor    *CommandExecutor
    output      *OutputFormatter
}
```

#### Client Configuration:
```json
{
  "clients": {
    "terminal_ui": {
      "enabled": true,
      "theme": "default",
      "layout": "standard"
    },
    "rest_api": {
      "enabled": true,
      "port": 8080,
      "cors": {
        "allowed_origins": ["*"],
        "allowed_methods": ["GET", "POST", "PUT", "DELETE"]
      }
    },
    "mobile": {
      "enabled": true,
      "platforms": ["ios", "android"],
      "push_notifications": true
    },
    "cli": {
      "enabled": true,
      "output_format": "text",
      "auto_complete": true
    }
  }
}
```

## 2.2 Data Architecture - Distributed Implementation

### 2.2.1 Database Schema - Enhanced for Distribution

#### Core Tables Structure:
```sql
-- Workers table for distributed computing
CREATE TABLE workers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    hostname VARCHAR(255) NOT NULL,
    ssh_config JSONB NOT NULL,
    capabilities TEXT[] NOT NULL DEFAULT '{}',
    resources JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'maintenance', 'failed')),
    last_heartbeat TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Indexes for performance
    CONSTRAINT workers_hostname_unique UNIQUE (hostname)
);

-- Tasks table for distributed task management
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    worker_id UUID REFERENCES workers(id) ON DELETE SET NULL,
    type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    input JSONB NOT NULL DEFAULT '{}',
    output JSONB,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    -- Performance indexes
    INDEX tasks_status_idx (status),
    INDEX tasks_project_idx (project_id),
    INDEX tasks_worker_idx (worker_id)
);

-- Tool definitions for LLM tool calling
CREATE TABLE tools (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    schema JSONB NOT NULL,
    implementation TEXT NOT NULL,
    category VARCHAR(100) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    CONSTRAINT tools_name_unique UNIQUE (name)
);

-- Additional required tables:
- projects: Project management with distributed capabilities
- sessions: Enhanced for multi-client access
- requests: Command execution history across workers
- models: Model registry with distributed deployment
- users: User management with multi-client authentication
- permissions: Enhanced access control for distributed resources
- logs: Distributed logging and audit trails
- configurations: Hierarchical configuration with worker overrides
- notifications: Notification management for all clients
```

### 2.2.2 Configuration Management - Distributed Implementation

#### Enhanced Configuration Structure:
```go
type Config struct {
    Global      GlobalConfig       `json:"global" validate:"required"`
    Providers   ProviderConfigs    `json:"providers" validate:"required,dive"`
    UI          UIConfig           `json:"ui" validate:"required"`
    Security    SecurityConfig     `json:"security" validate:"required"`
    Database    DatabaseConfig     `json:"database" validate:"required"`
    Workers     WorkerConfig       `json:"workers" validate:"required"`
    Clients     ClientConfigs      `json:"clients" validate:"required"`
    Tools       ToolConfig         `json:"tools" validate:"required"`
    Notifications NotificationConfig `json:"notifications" validate:"required"`
}

type WorkerConfig struct {
    Enabled             bool              `json:"enabled"`
    Pool                map[string]SSHConfig `json:"pool" validate:"dive"`
    AutoInstall         bool              `json:"auto_install"`
    HealthCheckInterval int               `json:"health_check_interval" validate:"min=10,max=300"`
    MaxConcurrentTasks  int               `json:"max_concurrent_tasks" validate:"min=1,max=100"`
}