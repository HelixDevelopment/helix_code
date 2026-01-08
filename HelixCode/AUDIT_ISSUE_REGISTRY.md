# HelixCode Audit Issue Registry

**Created**: 2026-01-08
**Last Updated**: 2026-01-08
**Audit Phase**: Phase 1 - Critical Path Audit

---

## Issue Summary

| Category | Open | In Progress | Fixed | Verified | Total |
|----------|------|-------------|-------|----------|-------|
| CRITICAL | 0 | 0 | 2 | 0 | 2 |
| HIGH | 2 | 0 | 8 | 0 | 10 |
| MEDIUM | 9 | 0 | 8 | 0 | 17 |
| LOW | 5 | 0 | 4 | 0 | 9 |
| **Total** | **16** | **0** | **22** | **0** | **38** |

---

## Phase 1.1: auth/ Package Issues

### HELIX-001: Broken Argon2 Password Verification
```
ID: HELIX-001
Category: BROKEN
Severity: CRITICAL
Package: internal/auth
File: auth.go:329-398
Status: FIXED
```

**Description**: The `verifyArgon2Password` function is critically broken. It compares the hash with itself (`subtle.ConstantTimeCompare([]byte(hash), []byte(hash)) == 1`), which always returns true. This means any password would be accepted for Argon2-hashed passwords.

**Expected**: Function should properly decode Argon2 hash parameters (salt, time, memory, threads, key length) and verify the password against those parameters.

**Actual**: Function always returns true because it compares `hash` to `hash`.

**Code**:
```go
func (s *AuthService) verifyArgon2Password(password, hash string) bool {
    // This is a simplified implementation
    parts := strings.Split(hash, "$")
    if len(parts) != 6 {
        return false
    }
    // For now, just use a simple comparison
    // In a real implementation, you'd decode the parameters and verify
    return subtle.ConstantTimeCompare([]byte(hash), []byte(hash)) == 1  // BUG: Always true!
}
```

**Fix Required**: Yes - implement proper Argon2 verification or remove the fallback
**Test Required**: Yes - add test to verify Argon2 password verification

---

### HELIX-002: JWT Secret Hardcoded in Config File
```
ID: HELIX-002
Category: MOCKLEAK
Severity: HIGH
Package: config
File: config/config.yaml:27
Status: FIXED
```

**Description**: JWT secret was hardcoded in the config file that's checked into version control.

**Resolution**: Changed config.yaml to use environment variable reference `${HELIX_AUTH_JWT_SECRET}`. Test config updated with clear warning comment. Documentation added to auth README.

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - configuration loading already tested

---

### HELIX-003: VerifyJWT Returns Minimal User Object
```
ID: HELIX-003
Category: INCOMPLETE
Severity: HIGH
Package: internal/auth
File: auth.go:279-285
Status: FIXED
```

**Description**: The `VerifyJWT` function returned a minimal user object. Users needing complete user data had no option.

**Resolution**: Added new `VerifyJWTWithDB(ctx, token)` method that:
- Validates the JWT token
- Fetches complete user from database
- Verifies user is still active
- Returns complete User with all fields

Original `VerifyJWT()` kept for backward compatibility and fast validation without DB lookup. Users can choose:
- `VerifyJWT()` - Fast, returns minimal user from claims
- `VerifyJWTWithDB()` - Complete, returns full user from database

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - COMPLETED (TestAuthService_VerifyJWTWithDB added)

---

### HELIX-004: README AuthService Struct Mismatch
```
ID: HELIX-004
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md:32-40
Status: FIXED
```

**Description**: README shows incorrect AuthService struct definition.

**README Shows**:
```go
type AuthService struct {
    db          *database.Database
    jwtSecret   []byte
    tokenExpiry time.Duration
}
```

**Actual**:
```go
type AuthService struct {
    config AuthConfig
    db     AuthRepository
}
```

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-005: README Claims Struct Mismatch
```
ID: HELIX-005
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md (removed)
Status: FIXED
```

**Description**: README shows a custom Claims struct that doesn't exist. Code uses jwt.MapClaims.

**README Shows**:
```go
type Claims struct {
    UserID   string   `json:"user_id"`
    Username string   `json:"username"`
    Roles    []string `json:"roles"`
    jwt.RegisteredClaims
}
```

**Actual**: Code uses `jwt.MapClaims` with keys: `user_id`, `username`, `email`, `exp`, `iat`. No Roles field.

**Fix Required**: Yes - update README to match actual implementation
**Test Required**: No

---

### HELIX-006: README Session Struct Incomplete
```
ID: HELIX-006
Category: INCONSISTENT
Severity: LOW
Package: internal/auth
File: README.md:80-94
Status: FIXED
```

**Description**: README shows simplified Session struct missing several fields.

**README Shows**: `ID, UserID, Token, ExpiresAt, CreatedAt`

**Actual Has**: `ID, UserID, SessionToken, ClientType, IPAddress, UserAgent, ExpiresAt, CreatedAt`

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-007: README NewAuthService Constructor Mismatch
```
ID: HELIX-007
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/auth
File: README.md:99-113
Status: FIXED
```

**Description**: README shows incorrect constructor signature.

**README Shows**: `auth.NewAuthService(db, jwtSecret, 24*time.Hour)`

**Actual**: `NewAuthService(config AuthConfig, db AuthRepository)`

**Fix Required**: Yes - update README
**Test Required**: No

---

### HELIX-008: RBAC Not Implemented
```
ID: HELIX-008
Category: MISSING
Severity: HIGH
Package: internal/auth
File: README.md:11
Status: FIXED
```

**Description**: README previously claimed "Role-based access control (RBAC)" but no RBAC implementation exists.

**Resolution**: Updated README to remove false RBAC claim. Added "Future Enhancements" section listing unimplemented features:
- Role-based access control (RBAC)
- Token refresh for long-running sessions
- Rate limiting for login attempts
- Multi-factor authentication (MFA)

**Fix Required**: Yes - COMPLETED (documentation corrected)
**Test Required**: No (documentation only)

---

### HELIX-009: Token Refresh Not Implemented
```
ID: HELIX-009
Category: MISSING
Severity: MEDIUM
Package: internal/auth
File: README.md:110
Status: OPEN
```

**Description**: README Security section mentions "Implement token refresh for long-running sessions" but no token refresh functionality exists.

**Fix Required**: Yes - either implement or update README
**Test Required**: Yes - if implementing

---

### HELIX-010: Rate Limiting Not Implemented
```
ID: HELIX-010
Category: MISSING
Severity: MEDIUM
Package: internal/auth
File: README.md:111
Status: OPEN
```

**Description**: README mentions "Implement rate limiting for login attempts" but no rate limiting exists.

**Fix Required**: Yes - either implement or update README
**Test Required**: Yes - if implementing

---

### HELIX-011: MFA Field Unused
```
ID: HELIX-011
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth.go:36
Status: OPEN
```

**Description**: User struct has `MFAEnabled bool` field but no MFA implementation exists. The field is always set to false during registration.

**Fix Required**: Yes - either implement MFA or remove field
**Test Required**: Yes - if implementing MFA

---

### HELIX-012: DisplayName Not Stored in DB
```
ID: HELIX-012
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth_db.go:72,111
Status: OPEN
```

**Description**: In `GetUserByUsername` and `GetUserByEmail`, DisplayName is hardcoded to empty string with comment "Not stored in DB", but `GetUserByID` tries to read it from DB. Schema inconsistency.

**Code**:
```go
user.DisplayName = "" // Not stored in DB
```

But in `GetUserByID`:
```go
&user.DisplayName,  // Tries to read from DB
```

**Fix Required**: Yes - ensure DisplayName is stored and retrieved consistently
**Test Required**: Yes - add test for DisplayName persistence

---

### HELIX-013: Test Coverage Gap - verifyArgon2Password Edge Cases
```
ID: HELIX-013
Category: INCOMPLETE
Severity: LOW
Package: internal/auth
File: auth_test.go:163-243
Status: FIXED
```

**Description**: No test for `verifyArgon2Password` edge cases. Current tests only use bcrypt. The broken Argon2 implementation (HELIX-001) was not caught by tests.

**Resolution**: Added comprehensive `TestAuthService_verifyArgon2Password` test with 9 test cases covering:
- Valid Argon2id hash verification
- Wrong password rejection
- Invalid hash format detection
- Invalid algorithm detection
- Invalid version format detection
- Invalid parameters format detection
- Invalid base64 salt detection
- Invalid base64 hash detection
- bcrypt hash rejection in Argon2 verification

**Fix Required**: Yes - add tests for Argon2 password flow
**Test Required**: Yes - COMPLETED

---

## Phase 1.2: task/ Package Issues

### HELIX-014: Checkpoint Creates Fake WorkerID
```
ID: HELIX-014
Category: STUB
Severity: HIGH
Package: internal/task
File: checkpoint.go:22, manager_methods.go:179-195
Status: FIXED
```

**Description**: The `CreateCheckpoint` function was generating a random worker ID instead of using the actual worker ID from task context.

**Resolution**:
1. Modified `CheckpointManager.CreateCheckpoint` to accept `workerID uuid.UUID` as a parameter
2. Modified `TaskManager.CreateCheckpoint` to retrieve workerID from `task.AssignedWorker` field
3. Updated all tests to pass the workerID parameter

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - COMPLETED (checkpoint_test.go updated)

---

### HELIX-015: RestoreFromCheckpoint Not Implemented
```
ID: HELIX-015
Category: MISSING
Severity: HIGH
Package: internal/task
File: README.md:121
Status: FIXED
```

**Description**: README documented `manager.RestoreFromCheckpoint(ctx, taskID)` but this function does not exist in the codebase.

**Resolution**: Removed non-existent function from README. Updated checkpointing section to show actual API: `GetLatestCheckpoint` and `GetCheckpoints`.

**Fix Required**: Yes - COMPLETED (removed from README)
**Test Required**: No (documentation only)

---

### HELIX-016: Automatic Checkpoint Interval Not Implemented
```
ID: HELIX-016
Category: MISSING
Severity: MEDIUM
Package: internal/task
File: README.md:114-115, 169
Status: FIXED
```

**Description**: README claimed checkpoints are "saved automatically at configured intervals" (default 300 seconds) but no automatic checkpointing is implemented.

**Resolution**: Removed false claim about automatic checkpointing from README. Updated to document only manual checkpoint creation.

**Fix Required**: Yes - COMPLETED (removed from README)
**Test Required**: No (documentation only)

---

### HELIX-017: Priority Values Mismatch
```
ID: HELIX-017
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/task
File: README.md:55-60 vs manager.go:34-39
Status: FIXED
```

**Description**: README showed different priority values than actual implementation.

**Resolution**: Updated README to show correct TaskPriority values (1, 5, 10, 20) matching manager.go.

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

### HELIX-018: Checkpoint Struct Mismatch
```
ID: HELIX-018
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/task
File: README.md:181-187 vs checkpoint.go:158-165
Status: FIXED
```

**Description**: README showed different Checkpoint struct than actual implementation.

**Resolution**: Updated README to show correct Checkpoint struct matching checkpoint.go with ID, CheckpointName, CheckpointData, WorkerID, CreatedAt fields.

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

### HELIX-019: Task Struct Incomplete in README
```
ID: HELIX-019
Category: INCONSISTENT
Severity: LOW
Package: internal/task
File: README.md:22-32 vs manager.go:75-95
Status: FIXED
```

**Description**: README showed simplified Task struct missing many fields.

**Resolution**: Updated README to show complete Task struct with all 18 fields matching manager.go.

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

### HELIX-020: TaskType Naming Convention Different
```
ID: HELIX-020
Category: INCONSISTENT
Severity: LOW
Package: internal/task
File: README.md:38-47 vs manager.go:19-29
Status: FIXED
```

**Description**: README showed `TypePlanning` but actual code uses `TaskTypePlanning`.

**Resolution**: Updated README to use correct naming convention (TaskTypePlanning, etc.) and added missing types: TaskTypeDesign, TaskTypeDiagram, TaskTypePorting.

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

### HELIX-021: Status Values Missing
```
ID: HELIX-021
Category: INCONSISTENT
Severity: LOW
Package: internal/task
File: README.md:66-75 vs manager.go:54-63
Status: FIXED
```

**Description**: README showed `StatusCancelled` but actual code uses `TaskStatusPaused`, `TaskStatusWaitingForWorker`, `TaskStatusWaitingForDeps`.

**Resolution**: Updated README to show correct TaskStatus values including TaskStatusPaused, TaskStatusWaitingForWorker, TaskStatusWaitingForDeps (removed non-existent cancelled).

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

## Phase 1.3: worker/ Package Issues

### HELIX-022: Hardcoded Helix CLI Download URL
```
ID: HELIX-022
Category: STUB
Severity: MEDIUM
Package: internal/worker
File: ssh_pool.go:562
Status: OPEN
```

**Description**: The `installHelixCLI` function uses a hardcoded URL for downloading the Helix CLI binary.

**Code**:
```go
curl -L https://github.com/helixdev/helix-cli/releases/latest/download/helix-linux-amd64 -o /tmp/helix
```

**Expected**: URL should be configurable via config or environment variable.

**Fix Required**: Yes - make URL configurable
**Test Required**: Yes - add test for configurable URL

---

### HELIX-023: installHelixCLI Called Before Worker ID Set
```
ID: HELIX-023
Category: BROKEN
Severity: HIGH
Package: internal/worker
File: ssh_pool.go:268-291
Status: FIXED
```

**Description**: In `AddWorker`, `installHelixCLI` was called before `worker.ID` was assigned. The `installHelixCLI` method internally calls `ExecuteCommand(ctx, worker.ID, ...)` but `worker.ID` was zero UUID.

**Resolution**: Moved worker ID assignment (and other initialization) BEFORE the auto-install and capability detection operations. Also added the worker to the pool map before these operations so `ExecuteCommand` can find it.

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - existing tests pass

---

### HELIX-024: README Documentation Complete Mismatch (FIXED)
```
ID: HELIX-024
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/worker
File: README.md (entire file)
Status: FIXED
```

**Description**: README documentation was completely mismatched with actual implementation. Wrong struct definitions, wrong status values, wrong function signatures, missing features.

**Resolution**: Completely rewrote README to match actual implementation including:
- Correct SSHWorkerPool struct
- Correct SSHWorker and Worker structs
- Correct Resources struct
- Correct SSHWorkerConfig struct
- Correct WorkerStatus values (active, inactive, maintenance, failed, offline)
- Correct WorkerHealth values (healthy, degraded, unhealthy, unknown)
- Documented isolation and consensus features
- Updated usage examples

**Fix Required**: Yes - COMPLETED
**Test Required**: No

---

## Phase 1.4: llm/ Package Issues

### HELIX-025: Missing Factory Support for 10 Local Providers
```
ID: HELIX-025
Category: MISSING
Severity: HIGH
Package: internal/llm
File: factory.go
Status: FIXED
```

**Description**: 10+ local LLM providers had implementations but were NOT registered in factory.go.

**Resolution**: Added 12 new switch cases to factory.go:
- ProviderTypeGroq → NewGroqProvider()
- ProviderTypeVLLM → NewVLLMProvider()
- ProviderTypeLocalAI → NewLocalAIProvider()
- ProviderTypeFastChat → NewFastChatProvider()
- ProviderTypeTextGen → NewTextGenProvider()
- ProviderTypeLMStudio → NewLMStudioProvider()
- ProviderTypeJan → NewJanProvider()
- ProviderTypeGPT4All → NewGPT4AllProvider()
- ProviderTypeTabbyAPI → NewTabbyAPIProvider()
- ProviderTypeMLX → NewMLXProvider()
- ProviderTypeMistralRS → NewMistralRSProvider()
- ProviderTypeKoboldAI → NewKoboldAIProvider() (with config mapping)

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - existing provider tests cover creation

---

### HELIX-026: README Type Names Mismatch
```
ID: HELIX-026
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/llm
File: README.md
Status: OPEN
```

**Description**: README documents types that don't exist or have wrong names.

**Type Mismatches**:
| README Type | Actual Type |
|-------------|-------------|
| GenerateRequest | LLMRequest |
| GenerateResponse | LLMResponse |
| ProviderConfig | ProviderConfigEntry |
| StreamChunk | LLMResponse (sent to channel) |

**Interface Mismatches**:
- README: `Generate(ctx, *GenerateRequest) (*GenerateResponse, error)`
- Actual: `Generate(ctx, *LLMRequest) (*LLMResponse, error)`
- README: Response has `Text` field
- Actual: Response has `Content` field

**Fix Required**: Yes - update README to match actual types
**Test Required**: No

---

### HELIX-027: Cost Tracking Not Implemented
```
ID: HELIX-027
Category: MISSING
Severity: MEDIUM
Package: internal/llm
File: README.md:192-196
Status: OPEN
```

**Description**: README documents cost tracking features that don't exist.

**README Claims**:
```go
cost := resp.Usage.Cost        // DOES NOT EXIST
currency := resp.Usage.Currency // DOES NOT EXIST
rates := provider.GetCostRates() // DOES NOT EXIST
```

**Actual Usage Struct**:
```go
type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    // No Cost, Currency, or rates
}
```

**Fix Required**: Yes - implement or remove from README
**Test Required**: Yes - if implementing

---

### HELIX-028: Provider Manager API Mismatch
```
ID: HELIX-028
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/llm
File: README.md:134-145
Status: OPEN
```

**Description**: README documents `NewProviderManager()` but this function doesn't exist.

**README Claims**:
```go
manager := llm.NewProviderManager(config)
manager.AddProvider("openai", openaiProvider)
manager.GenerateWithProvider(ctx, "anthropic", req)
```

**Actual**: Use `NewModelManager()`, `AutoLLMManager`, or `IntegratedModelManager` instead.

**Fix Required**: Yes - update README to show actual API
**Test Required**: No

---

### HELIX-029: Hardcoded Model Alternatives in Discovery
```
ID: HELIX-029
Category: STUB
Severity: LOW
Package: internal/llm
File: model_discovery.go:1135-1156
Status: OPEN
```

**Description**: `FindAlternativeModels` uses hardcoded map instead of dynamic discovery.

**Code**:
```go
// For now, return some hardcoded alternatives
alternativeMap := map[string][]string{
    "llama-3-8b-instruct":   {"mistral-7b-instruct", "codellama-7b-instruct"},
    "mistral-7b-instruct":   {"llama-3-8b-instruct", "zephyr-7b-beta"},
    "codellama-7b-instruct": {"starcoder-7b", "deepseek-coder-6.7b"},
}
```

**Fix Required**: Yes - implement dynamic discovery or document limitation
**Test Required**: Yes - if implementing

---

### HELIX-030: Unused Provider Type Constants
```
ID: HELIX-030
Category: INCOMPLETE
Severity: LOW
Package: internal/llm
File: missing_types.go:37-77
Status: OPEN
```

**Description**: 41 provider type constants are defined but only ~17 have implementations.

**Unused Types (no implementation)**:
- ProviderTypeMemGPT, ProviderTypeCrewAI, ProviderTypeCharacterAI
- ProviderTypeReplika, ProviderTypeAnima, ProviderTypeGemma
- ProviderTypeLlamaIndex, ProviderTypeCohere, ProviderTypeHuggingFace
- ProviderTypeMistral, ProviderTypeClickHouse, ProviderTypeSupabase
- ProviderTypeDeepLake, ProviderTypeChroma, ProviderTypeAgnostic
- And others (~24 total)

**Fix Required**: Yes - implement or remove unused constants
**Test Required**: Yes - if implementing

---

## Phase 1.5: workflow/ Package Issues

### HELIX-031: Command Injection Vulnerability
```
ID: HELIX-031
Category: BROKEN
Severity: CRITICAL
Package: internal/workflow
File: executor.go:796-879
Status: FIXED
```

**Description**: `executeCommandStep()` passed user input directly to bash without validation.

**Resolution**: Added comprehensive command security validation:
1. Added `isDangerousCommand()` function that checks:
   - 20+ dangerous command prefixes (rm, dd, mkfs, fdisk, shred, wipefs, parted, shutdown, reboot, etc.)
   - 25+ dangerous patterns (rm -rf /, piped shell execution, command substitution, raw disk access, etc.)
   - Case-insensitive matching to prevent bypass attempts
   - Whitespace normalization

2. Modified `executeCommandStep()` to:
   - Validate command is not empty
   - Check command against dangerous patterns before execution
   - Return clear error messages when dangerous commands are blocked

**Security Protection Now Includes**:
- Root/home directory deletion prevention
- Piped shell execution detection (| bash, | sh)
- Command substitution attack detection (\`rm\`, $(rm))
- Chained command detection (; rm, && rm, || rm)
- Raw disk device access prevention
- System control command blocking

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - COMPLETED (65+ test cases in TestIsDangerousCommand, TestExecuteCommandStep_BlocksDangerousCommands, TestExecuteCommandStep_AllowsSafeCommands, TestExecuteCommandStep_EmptyCommand)

---

### HELIX-032: Template Generators Return TODO Placeholders
```
ID: HELIX-032
Category: STUB
Severity: HIGH
Package: internal/workflow
File: executor.go:660-794
Status: OPEN
```

**Description**: Code template generators return TODO comments and "Task implementation pending" messages instead of actual code.

**Generated Code Example**:
```go
// TODO: Implement the following task:
// {step description}

fmt.Println("Task implementation pending")
fmt.Println("Configure an LLM provider in HelixCode for AI-powered code generation")
```

**Affected Templates**: Go, Node.js, Python, Rust

**Fix Required**: Yes - integrate LLM or provide functional templates
**Test Required**: Yes - verify generated code is functional

---

### HELIX-033: Static Analysis Returns Hardcoded Recommendations
```
ID: HELIX-033
Category: STUB
Severity: MEDIUM
Package: internal/workflow
File: executor.go:498-533
Status: OPEN
```

**Description**: `performStaticAnalysis()` returns generic hardcoded recommendations regardless of actual project content.

**Hardcoded Output**:
```
## Recommendations
- Enable LLM analysis for deeper insights
- Review entry points for optimization opportunities
```

**Fix Required**: Yes - implement actual code analysis
**Test Required**: Yes - verify analysis is meaningful

---

### HELIX-034: Missing Documented API Methods
```
ID: HELIX-034
Category: MISSING
Severity: HIGH
Package: internal/workflow
File: executor.go
Status: FIXED
```

**Description**: README documented API methods that don't exist (CreateWorkflow, ExecuteWorkflow, etc.).

**Resolution**: Completely rewrote workflow README to document actual API:
- NewExecutor() / NewExecutorWithLLM() - constructor methods
- ExecutePlanningWorkflow() - planning workflow
- ExecuteBuildingWorkflow() - build workflow
- ExecuteTestingWorkflow() - test workflow
- ExecuteRefactoringWorkflow() - refactoring workflow
- GetWorkflow() - get workflow by ID
- GetActiveWorkflows() - list active workflows
- CancelWorkflow() - cancel running workflow
- GetMetrics() - execution metrics
- SetLLMProvider() - configure LLM

Removed non-existent methods from documentation.

**Fix Required**: Yes - COMPLETED (documentation corrected)
**Test Required**: No (documentation only)

---

### HELIX-035: Missing Step Types and Actions
```
ID: HELIX-035
Category: INCONSISTENT
Severity: MEDIUM
Package: internal/workflow
File: workflow.go
Status: OPEN
```

**Description**: README documents step types and actions that don't exist in code.

**Missing**:
- StepTypeDeployment (constant not defined)
- StepActionDeploy (constant not defined)
- StepActionFormat (constant not defined)

**Fix Required**: Yes - implement or update README
**Test Required**: No

---

### HELIX-036: Planmode Placeholder Implementations
```
ID: HELIX-036
Category: STUB
Severity: MEDIUM
Package: internal/workflow/planmode
File: executor.go:366-409
Status: OPEN
```

**Description**: Multiple planmode step handlers are placeholders that only return formatted strings.

**Placeholder Methods**:
- executeFileOperation() - doesn't modify files
- executeCodeGeneration() - no LLM integration
- executeCodeAnalysis() - no actual analysis
- executeValidation() - no checks performed
- executeTesting() - no tests run

**Fix Required**: Yes - implement actual functionality
**Test Required**: Yes - verify each step type works

---

### HELIX-037: Inadequate Dangerous Command Detection
```
ID: HELIX-037
Category: BROKEN
Severity: HIGH
Package: internal/workflow/autonomy
File: executor.go:224-339
Status: FIXED
```

**Description**: `containsDangerous()` only checked 4 commands with naive prefix matching.

**Resolution**: Completely rewrote `containsDangerous()` function:
- Added normalization (trim whitespace, lowercase conversion)
- Added 20+ dangerous command prefixes (rm, dd, mkfs, fdisk, shred, wipefs, parted, shutdown, reboot, kill, killall, pkill, systemctl, etc.)
- Added 25+ dangerous patterns (rm -rf /, --no-preserve-root, piped shell execution, raw disk access, etc.)
- Added `containsShellExploit()` for command substitution detection
- Added `extractBetween()` helper for parsing nested commands
- Case-insensitive matching to prevent bypass

New features:
- Detects command substitution attacks (`rm`, $(rm))
- Detects chained commands (; rm, && rm, || rm)
- Detects piped shell execution (| bash, | sh)
- Detects raw disk access (/dev/sda, /dev/nvme)

**Fix Required**: Yes - COMPLETED
**Test Required**: Yes - COMPLETED (TestContainsDangerous, TestContainsShellExploit, TestExtractBetween added with 48+ test cases)

---

### HELIX-038: Autonomy Executor Always Succeeds
```
ID: HELIX-038
Category: BROKEN
Severity: MEDIUM
Package: internal/workflow/autonomy
File: executor.go:131-165
Status: OPEN
```

**Description**: `executeAction()` always sets `Success: true` regardless of actual execution.

**Code**:
```go
result := &ActionResult{
    Action:  action,
    Success: true,  // Always true!
    Output:  fmt.Sprintf("Executed: %s", action.Description),
}
```

**Fix Required**: Yes - implement actual execution with proper error handling
**Test Required**: Yes - verify failure cases are handled

---

## Changelog

| Date | Issue | Action | By |
|------|-------|--------|-----|
| 2026-01-08 | HELIX-001 to HELIX-013 | Created | Audit |
| 2026-01-08 | HELIX-001 | FIXED - Implemented proper Argon2 password verification | Audit |
| 2026-01-08 | HELIX-004 | FIXED - Updated README AuthService struct | Audit |
| 2026-01-08 | HELIX-005 | FIXED - Removed incorrect Claims struct from README | Audit |
| 2026-01-08 | HELIX-006 | FIXED - Updated README Session struct | Audit |
| 2026-01-08 | HELIX-007 | FIXED - Updated README constructor example | Audit |
| 2026-01-08 | HELIX-013 | FIXED - Added Argon2 verification tests | Audit |
| 2026-01-08 | HELIX-014 to HELIX-021 | Created (task/ package) | Audit |
| 2026-01-08 | HELIX-014 | FIXED - Added workerID parameter to CreateCheckpoint | Audit |
| 2026-01-08 | HELIX-015 | FIXED - Removed non-existent RestoreFromCheckpoint from README | Audit |
| 2026-01-08 | HELIX-016 | FIXED - Removed false auto-checkpoint claim from README | Audit |
| 2026-01-08 | HELIX-017 | FIXED - Updated README priority values | Audit |
| 2026-01-08 | HELIX-018 | FIXED - Updated README Checkpoint struct | Audit |
| 2026-01-08 | HELIX-019 | FIXED - Updated README Task struct (complete) | Audit |
| 2026-01-08 | HELIX-020 | FIXED - Updated README TaskType naming convention | Audit |
| 2026-01-08 | HELIX-021 | FIXED - Updated README TaskStatus values | Audit |
| 2026-01-08 | HELIX-022 to HELIX-024 | Created (worker/ package) | Audit |
| 2026-01-08 | HELIX-023 | FIXED - Worker ID assigned before installHelixCLI | Audit |
| 2026-01-08 | HELIX-024 | FIXED - Completely rewrote worker README | Audit |
| 2026-01-08 | HELIX-025 to HELIX-030 | Created (llm/ package) | Audit |
| 2026-01-08 | HELIX-031 to HELIX-038 | Created (workflow/ package) | Audit |
| 2026-01-08 | HELIX-002 | FIXED - JWT secret changed to env var reference | Audit |
| 2026-01-08 | HELIX-003 | FIXED - Added VerifyJWTWithDB method | Audit |
| 2026-01-08 | HELIX-008 | FIXED - Documented RBAC as future enhancement | Audit |
| 2026-01-08 | HELIX-025 | FIXED - Added 12 missing providers to factory.go | Audit |
| 2026-01-08 | HELIX-034 | FIXED - Completely rewrote workflow README | Audit |
| 2026-01-08 | HELIX-037 | FIXED - Comprehensive dangerous command detection | Audit |
| 2026-01-08 | HELIX-031 | FIXED - Command injection vulnerability with 65+ tests | Audit |
