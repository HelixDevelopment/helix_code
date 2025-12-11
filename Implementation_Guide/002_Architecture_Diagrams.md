# HelixCode - Architecture Diagrams

## System Architecture Overview

### Core System Architecture

```mermaid
graph TB
    subgraph "Clients"
        TUI[Terminal UI]
        CLI[CLI Interface]
        REST[REST API]
        MOB[Mobile Apps]
        WEB[Web Interface]
    end
    
    subgraph "HelixCode Server"
        API[API Gateway]
        AUTH[Authentication]
        WM[Worker Manager]
        TM[Task Manager]
        MCP[MCP Integration]
        LLM[LLM Provider]
        NOTIF[Notification]
    end
    
    subgraph "Database Layer"
        PG[(PostgreSQL)]
        REDIS[(Redis Cache)]
    end
    
    subgraph "Worker Network"
        W1[Worker 1]
        W2[Worker 2]
        W3[Worker 3]
        WN[Worker N]
    end
    
    subgraph "External Services"
        HF[HuggingFace]
        OLL[Ollama]
        OPENAI[OpenAI]
        ANTH[Anthropic]
    end
    
    TUI --> API
    CLI --> API
    REST --> API
    MOB --> API
    WEB --> API
    
    API --> AUTH
    API --> WM
    API --> TM
    API --> MCP
    API --> LLM
    API --> NOTIF
    
    AUTH --> PG
    WM --> PG
    TM --> PG
    MCP --> PG
    LLM --> PG
    NOTIF --> PG
    
    WM --> REDIS
    TM --> REDIS
    
    WM --> W1
    WM --> W2
    WM --> W3
    WM --> WN
    
    LLM --> HF
    LLM --> OLL
    LLM --> OPENAI
    LLM --> ANTH
    
    MCP --> W1
    MCP --> W2
    MCP --> W3
    MCP --> WN
```

### Distributed Task Processing Architecture

```mermaid
sequenceDiagram
    participant C as Client
    participant TM as Task Manager
    participant WM as Worker Manager
    participant W1 as Worker 1
    participant W2 as Worker 2
    participant W3 as Worker 3
    participant DB as Database
    
    C->>TM: Submit Large Task
    TM->>DB: Store Task Metadata
    TM->>TM: Analyze Task Dependencies
    TM->>TM: Divide into Subtasks
    
    loop For Each Subtask
        TM->>WM: Request Worker
        WM->>DB: Check Worker Availability
        WM->>TM: Assign Worker
        TM->>Worker: Send Subtask
        Worker->>TM: Acknowledge Receipt
        Worker->>Worker: Process Subtask
        Worker->>DB: Store Checkpoint
        Worker->>TM: Send Progress Update
    end
    
    TM->>TM: Monitor Subtask Completion
    
    alt All Subtasks Successful
        TM->>TM: Aggregate Results
        TM->>DB: Mark Task Complete
        TM->>C: Send Final Result
    else Some Subtasks Failed
        TM->>TM: Identify Failed Subtasks
        TM->>WM: Request Worker Replacement
        WM->>TM: Assign New Worker
        TM->>New Worker: Retry Failed Subtask
        New Worker->>TM: Send Result
        TM->>TM: Continue Aggregation
    end
```

### Work Preservation & Rollback Architecture

```mermaid
stateDiagram-v2
    [*] --> TaskCreated
    TaskCreated --> TaskAnalyzed
    TaskAnalyzed --> TaskDivided
    TaskDivided --> SubtasksAssigned
    
    SubtasksAssigned --> SubtaskProcessing
    SubtaskProcessing --> CheckpointSaved
    CheckpointSaved --> SubtaskProcessing
    
    SubtaskProcessing --> WorkerDisconnected
    WorkerDisconnected --> CriticalityCheck
    
    CriticalityCheck --> PauseAllTasks : Critical Task
    CriticalityCheck --> ContinueOtherTasks : Non-Critical Task
    
    PauseAllTasks --> WaitForWorker
    WaitForWorker --> WorkerReconnected
    WorkerReconnected --> ResumeTasks
    ResumeTasks --> SubtaskProcessing
    
    ContinueOtherTasks --> MonitorWorker
    MonitorWorker --> WorkerReconnected
    WorkerReconnected --> ResumeTask
    ResumeTask --> SubtaskProcessing
    
    SubtaskProcessing --> AllSubtasksComplete
    AllSubtasksComplete --> ResultsAggregated
    ResultsAggregated --> TaskCompleted
    TaskCompleted --> [*]
    
    SubtaskProcessing --> RollbackRequired
    RollbackRequired --> LoadCheckpoint
    LoadCheckpoint --> SubtaskProcessing
```

### Database Schema Relationships

```mermaid
erDiagram
    users ||--o{ user_sessions : "has"
    users ||--o{ projects : "owns"
    users ||--o{ notifications : "receives"
    users ||--o{ audit_logs : "generates"
    
    projects ||--o{ sessions : "contains"
    
    workers ||--o{ worker_metrics : "records"
    workers ||--o{ worker_connectivity_events : "generates"
    workers ||--o{ distributed_tasks : "assigned"
    workers ||--o{ task_checkpoints : "creates"
    
    distributed_tasks ||--o{ task_checkpoints : "has"
    distributed_tasks }o--|| workers : "assigned_to"
    distributed_tasks }o--|| workers : "original_worker"
    
    sessions ||--|| distributed_tasks : "current_task"
    
    llm_providers ||--o{ llm_models : "provides"
    
    mcp_servers ||--o{ tools : "exposes"
    
    distributed_tasks {
        uuid id PK
        varchar task_type
        jsonb task_data
        varchar status
        integer priority
        varchar criticality
        uuid assigned_worker_id FK
        uuid[] dependencies
        integer retry_count
        jsonb checkpoint_data
        timestamptz created_at
        timestamptz updated_at
    }
    
    workers {
        uuid id PK
        varchar hostname
        jsonb ssh_config
        text[] capabilities
        varchar status
        varchar health_status
        timestamptz last_heartbeat
        integer current_tasks_count
        timestamptz created_at
        timestamptz updated_at
    }
    
    task_checkpoints {
        uuid id PK
        uuid task_id FK
        varchar checkpoint_name
        jsonb checkpoint_data
        uuid worker_id FK
        timestamptz created_at
    }
    
    worker_connectivity_events {
        uuid id PK
        uuid worker_id FK
        varchar event_type
        jsonb event_data
        timestamptz created_at
    }
```

### Cross-Platform Client Architecture

```mermaid
graph TB
    subgraph "Shared Business Logic (Kotlin Multiplatform)"
        CORE[Core Module]
        AI[AI/ML Services]
        MCP[MCP Integration]
        AUTH[Authentication]
        DB[Database Layer]
    end
    
    subgraph "Mobile Platforms"
        subgraph "iOS"
            IOS_UI[iOS SwiftUI]
            IOS_NAT[iOS Native APIs]
        end
        
        subgraph "Android"
            AND_UI[Android Compose]
            AND_NAT[Android Native APIs]
        end
    end
    
    subgraph "Desktop/Web Platforms (Flutter)"
        FLUTTER[Flutter Framework]
        WEB[Web Support]
        DESKTOP[Desktop Support]
        TERMINAL[Terminal Integration]
    end
    
    subgraph "Terminal Interface (Go)"
        GO_CLI[Go CLI]
        TUI[Terminal UI]
        API_INT[API Integration]
    end
    
    CORE --> IOS_UI
    CORE --> AND_UI
    CORE --> FLUTTER
    CORE --> GO_CLI
    
    IOS_UI --> IOS_NAT
    AND_UI --> AND_NAT
    FLUTTER --> WEB
    FLUTTER --> DESKTOP
    FLUTTER --> TERMINAL
    GO_CLI --> TUI
    GO_CLI --> API_INT
    
    AI --> CORE
    MCP --> CORE
    AUTH --> CORE
    DB --> CORE
```

### Real Model Testing Architecture

```mermaid
graph TB
    subgraph "Testing Infrastructure"
        TEST_MGR[Test Manager]
        MODEL_REG[Model Registry]
        TEST_RUNNER[Test Runner]
        RESULT_AGG[Result Aggregator]
    end
    
    subgraph "Model Providers"
        LOCAL[Local Models]
        OLLAMA[Ollama]
        LLAMA_CPP[LLama.cpp]
        HF[HuggingFace]
    end
    
    subgraph "Test Types"
        UNIT[Unit Tests]
        INTEG[Integration Tests]
        E2E[End-to-End Tests]
        AUTO[Automation Tests]
        AI_QA[AI QA Tests]
    end
    
    subgraph "Real Devices"
        MOB_DEV[Mobile Devices]
        DESK_DEV[Desktop Machines]
        WORKER_DEV[Worker Nodes]
    end
    
    subgraph "Test Results"
        COVERAGE[Coverage Reports]
        PERF[Performance Metrics]
        SECURITY[Security Scans]
        QUALITY[Quality Gates]
    end
    
    TEST_MGR --> MODEL_REG
    TEST_MGR --> TEST_RUNNER
    TEST_RUNNER --> LOCAL
    TEST_RUNNER --> OLLAMA
    TEST_RUNNER --> LLAMA_CPP
    TEST_RUNNER --> HF
    
    TEST_RUNNER --> UNIT
    TEST_RUNNER --> INTEG
    TEST_RUNNER --> E2E
    TEST_RUNNER --> AUTO
    TEST_RUNNER --> AI_QA
    
    UNIT --> MOB_DEV
    INTEG --> DESK_DEV
    E2E --> WORKER_DEV
    AUTO --> MOB_DEV
    AI_QA --> DESK_DEV
    
    TEST_RUNNER --> RESULT_AGG
    RESULT_AGG --> COVERAGE
    RESULT_AGG --> PERF
    RESULT_AGG --> SECURITY
    RESULT_AGG --> QUALITY
```

## Key Architectural Features

### 1. Distributed Task Division
- **Intelligent Task Splitting**: Automatically divides large tasks into optimal subtasks
- **Dependency Management**: Handles complex task dependencies across workers
- **Load Balancing**: Distributes work based on worker capabilities and current load
- **Progress Tracking**: Real-time monitoring of all subtask progress

### 2. Work Preservation Mechanisms
- **Automatic Checkpointing**: Regular save points for all tasks
- **Worker Health Monitoring**: Continuous monitoring of worker connectivity
- **Criticality-Based Pausing**: Pauses entire workflow for critical task failures
- **Graceful Degradation**: Continues non-critical tasks during worker issues

### 3. Cross-Platform Architecture
- **Shared Business Logic**: Kotlin Multiplatform for mobile platforms
- **Unified Desktop/Web**: Flutter for consistent desktop and web experience
- **Native Terminal**: Go-based CLI for maximum terminal performance
- **Platform-Specific Optimization**: Leverages native capabilities where needed

### 4. Real Model Testing
- **Multi-Provider Support**: Tests with local and remote model providers
- **Hardware Alignment**: Uses models that match local machine capabilities
- **Comprehensive Coverage**: 100% test coverage across all test types
- **Real Device Validation**: Testing on actual hardware devices