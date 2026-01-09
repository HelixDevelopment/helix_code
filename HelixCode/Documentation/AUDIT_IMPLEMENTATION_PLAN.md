# HelixCode Audit Implementation Plan

**Created**: 2026-01-09
**Status**: IN PROGRESS
**Last Updated**: 2026-01-09

This document tracks the implementation of fixes identified in the Comprehensive Audit Report.

---

## Progress Tracking

### Overall Status

| Phase | Status | Progress | Blockers |
|-------|--------|----------|----------|
| P0 Critical - Mock Data | NOT STARTED | 0% | None |
| P0 Critical - Dependencies | NOT STARTED | 0% | None |
| P0 Critical - Security | NOT STARTED | 0% | None |
| P1 High - Stubs | NOT STARTED | 0% | Depends on P0 |
| P1 High - Coverage | NOT STARTED | 0% | Depends on P0 |
| P2 Medium - Apps | NOT STARTED | 0% | Depends on P1 |
| P2 Medium - Docs | NOT STARTED | 0% | None |
| P3 Low - Remaining | NOT STARTED | 0% | Depends on P2 |

---

## Phase 0: CRITICAL Fixes

### Task P0-1: Remove Mock Data from handlers.go

**File**: `internal/server/handlers.go`
**Priority**: P0 - CRITICAL
**Estimated Effort**: 8 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P0-1.1** Fix `createTask()` (lines 392-405)
  - Replace placeholder ID with proper error response
  - Return HTTP 503 when database unavailable
  - Add test for this behavior

- [ ] **P0-1.2** Fix `getTask()` (lines 429-442)
  - Remove "Sample Task" hardcoded data
  - Return HTTP 503 or 404 appropriately
  - Add test for database unavailable scenario

- [ ] **P0-1.3** Fix `updateTask()` (lines 507-521)
  - Remove placeholder data return
  - Return proper error when database unavailable
  - Add test coverage

- [ ] **P0-1.4** Fix `updateProject()` (lines 287-302)
  - Remove hardcoded "/path/to/project"
  - Implement actual project update logic
  - Add test coverage

- [ ] **P0-1.5** Fix `deleteProject()` (lines 305-311)
  - Implement actual deletion logic
  - Return error if delete fails
  - Add test coverage

- [ ] **P0-1.6** Fix `getWorker()` (lines 593-605)
  - Remove hardcoded "localhost"
  - Return proper error when worker not found
  - Add test coverage

**Verification**:
- [ ] All tests pass
- [ ] No placeholder data in responses
- [ ] Coverage increased

---

### Task P0-2: Fix Authentication Bypass

**Files**:
- `internal/server/handlers.go`
- `internal/project/manager_db.go`

**Priority**: P0 - CRITICAL
**Estimated Effort**: 4 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P0-2.1** Remove `"default-user"` fallback in `listProjects()` (handlers.go:18-22)
  - Return HTTP 401 Unauthorized when user_id missing
  - Add proper authentication middleware check
  - Add test for unauthorized access

- [ ] **P0-2.2** Fix `CreateProject()` (manager_db.go:134-135)
  - Remove hardcoded "default-user"
  - Require user ID parameter
  - Update all callers
  - Add test coverage

**Verification**:
- [ ] No hardcoded user IDs
- [ ] Proper 401 responses for unauthenticated requests
- [ ] Tests cover auth scenarios

---

### Task P0-3: Fix Confirmation/Permission Bypass

**Files**:
- `internal/tools/confirmation/prompter.go`
- `internal/workflow/autonomy/permission.go`

**Priority**: P0 - CRITICAL
**Estimated Effort**: 6 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P0-3.1** Fix auto-allow in prompter (prompter.go:179-185)
  - Remove automatic `ChoiceAllow` return
  - Require actual input source
  - Return error when no input available
  - Add test for this behavior

- [ ] **P0-3.2** Fix mock confirmation (permission.go:232-242)
  - Implement actual confirmation flow
  - Block until user responds or timeout
  - Add test coverage

**Verification**:
- [ ] No automatic permission grants
- [ ] Confirmation required for sensitive actions
- [ ] Tests verify blocking behavior

---

### Task P0-4: Security Dependency Updates

**File**: `go.mod`
**Priority**: P0 - CRITICAL
**Estimated Effort**: 4 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P0-4.1** Remove deprecated Redis v8
  ```bash
  go mod edit -droprequire github.com/go-redis/redis/v8
  ```
  - Update all imports to use v9
  - Verify no v8 references remain

- [ ] **P0-4.2** Update Redis v9
  ```bash
  go get github.com/redis/go-redis/v9@v9.17.2
  ```
  - Test Redis functionality

- [ ] **P0-4.3** Update golang.org/x/crypto
  ```bash
  go get golang.org/x/crypto@v0.46.0
  ```
  - Run security tests

- [ ] **P0-4.4** Replace nfnt/resize
  - Find modern image resize library
  - Update all usages
  - Add deprecation tests

- [ ] **P0-4.5** Replace golang.org/x/freetype
  - Find modern font library
  - Update all usages

- [ ] **P0-4.6** Run `go mod tidy`
  - Verify build passes
  - Run full test suite

**Verification**:
- [ ] `go mod graph` shows no deprecated deps
- [ ] All tests pass
- [ ] Security scan clean

---

### Task P0-5: Implement Network Isolation

**File**: `internal/tools/shell/sandbox.go`
**Priority**: P0 - CRITICAL
**Estimated Effort**: 8 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P0-5.1** Design network isolation approach
  - Research Linux network namespaces
  - Document implementation strategy

- [ ] **P0-5.2** Implement for Linux (lines 190-196)
  - Create network namespace
  - Apply firewall rules
  - Test isolation

- [ ] **P0-5.3** Add fallback for other platforms
  - Return error on unsupported platforms
  - Document limitations

- [ ] **P0-5.4** Add comprehensive tests
  - Test network is isolated
  - Test fallback behavior

**Verification**:
- [ ] Network isolated in sandbox
- [ ] Tests verify isolation
- [ ] Documentation updated

---

## Phase 1: HIGH Priority Fixes

### Task P1-1: Implement AI Provider Wrappers

**File**: `internal/providers/ai_integration.go`
**Priority**: P1 - HIGH
**Estimated Effort**: 16 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P1-1.1** Implement OpenAI wrapper (line 1326)
- [ ] **P1-1.2** Implement Anthropic wrapper (line 1329)
- [ ] **P1-1.3** Implement Cohere wrapper (line 1332)
- [ ] **P1-1.4** Implement HuggingFace wrapper (line 1335)
- [ ] **P1-1.5** Implement Mistral wrapper (line 1338)
- [ ] **P1-1.6** Implement Gemini wrapper (line 1341)
- [ ] **P1-1.7** Implement Gemma wrapper (line 1344)
- [ ] **P1-1.8** Implement LlamaIndex wrapper (line 1347)
- [ ] **P1-1.9** Implement MemGPT wrapper (line 1350)
- [ ] **P1-1.10** Implement CrewAI wrapper (line 1353)
- [ ] **P1-1.11** Implement CharacterAI wrapper (line 1356)
- [ ] **P1-1.12** Implement Repika wrapper (line 1359)
- [ ] **P1-1.13** Implement Anima wrapper (line 1362)
- [ ] **P1-1.14** Add tests for all wrappers

**Verification**:
- [ ] No `NotImplementedProvider` returns
- [ ] All wrappers functional
- [ ] Tests pass

---

### Task P1-2: Complete Zep Provider

**File**: `internal/memory/providers/zep_provider.go`
**Priority**: P1 - HIGH
**Estimated Effort**: 6 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P1-2.1** Implement `Retrieve()` (line 207)
  - Research Zep API for ID retrieval
  - Implement proper retrieval
  - Add tests

- [ ] **P1-2.2** Implement `Update()` (line 213)
  - Research Zep API for updates
  - Implement proper update
  - Add tests

- [ ] **P1-2.3** Implement `Delete()` (line 221)
  - Research Zep API for deletion
  - Implement proper deletion
  - Add tests

**Verification**:
- [ ] CRUD operations work
- [ ] No stub warnings logged
- [ ] Tests pass

---

### Task P1-3: Implement Tree-Sitter Parser

**File**: `internal/tools/mapping/treesitter.go`
**Priority**: P1 - HIGH
**Estimated Effort**: 8 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P1-3.1** Add tree-sitter dependency
  ```bash
  go get github.com/smacker/go-tree-sitter
  ```

- [ ] **P1-3.2** Implement Go parser
- [ ] **P1-3.3** Implement Python parser
- [ ] **P1-3.4** Implement JavaScript/TypeScript parser
- [ ] **P1-3.5** Implement fallback for unsupported languages
- [ ] **P1-3.6** Add comprehensive tests

**Verification**:
- [ ] AST parsing works for supported languages
- [ ] Tests pass
- [ ] Documentation updated

---

### Task P1-4: Increase Test Coverage (Critical Packages)

**Priority**: P1 - HIGH
**Estimated Effort**: 60 hours
**Assigned To**: TBD (multiple developers)

#### Coverage Targets

| Package | Current | Target | Tests Needed |
|---------|---------|--------|--------------|
| internal/tools/browser | 20.5% | 80% | ~50 |
| internal/memory/providers | 28.2% | 80% | ~80 |
| internal/workflow/planmode | 31.0% | 80% | ~30 |
| internal/llm | 42.0% | 80% | ~100 |
| internal/server | 56.4% | 80% | ~30 |

#### Subtasks

- [ ] **P1-4.1** Browser package tests
- [ ] **P1-4.2** Memory providers tests
- [ ] **P1-4.3** Workflow planmode tests
- [ ] **P1-4.4** LLM package tests
- [ ] **P1-4.5** Server package tests

**Verification**:
- [ ] Coverage reports show 80%+
- [ ] All tests pass
- [ ] No flaky tests

---

## Phase 2: MEDIUM Priority

### Task P2-1: Application Test Coverage

**Priority**: P2 - MEDIUM
**Estimated Effort**: 40 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P2-1.1** Desktop application tests (target 60%)
- [ ] **P2-1.2** Terminal UI tests (target 60%)
- [ ] **P2-1.3** Aurora OS tests (target 60%)
- [ ] **P2-1.4** Harmony OS tests (target 60%)
- [ ] **P2-1.5** Mobile binding tests (target 50%)

---

### Task P2-2: Documentation Updates

**Priority**: P2 - MEDIUM
**Estimated Effort**: 16 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P2-2.1** Update CLAUDE.md with accurate info
  - Fix checkpoint interval documentation
  - Document MCP transport status
  - Update fallback documentation

- [ ] **P2-2.2** Add missing READMEs
  - cmd/config-test/README.md
  - cmd/helix-config/README.md
  - cmd/security-fix/README.md
  - applications/android/README.md
  - applications/ios/README.md

- [ ] **P2-2.3** Update API documentation
  - Document error responses
  - Remove references to placeholder data

- [ ] **P2-2.4** Update user manuals
  - Add troubleshooting for database unavailable
  - Document authentication requirements

---

## Phase 3: LOW Priority

### Task P3-1: Complete Remaining Coverage

**Priority**: P3 - LOW
**Estimated Effort**: 40 hours
**Assigned To**: TBD

Target: 100% coverage for all packages

---

### Task P3-2: Enhancement Features

**Priority**: P3 - LOW
**Estimated Effort**: 24 hours
**Assigned To**: TBD

#### Subtasks

- [ ] **P3-2.1** Multi-Edit Git integration
- [ ] **P3-2.2** Multi-Edit rename operation
- [ ] **P3-2.3** FAISS real compression
- [ ] **P3-2.4** MCP stdio transport (if needed)

---

## Verification Checklist Template

Use this checklist for each completed task:

```markdown
### Task [ID] Verification

**Developer**:
**Review Date**:
**Reviewer**:

#### Code Quality
- [ ] Code follows project style guide
- [ ] No new linting warnings
- [ ] No hardcoded values
- [ ] Error handling complete

#### Testing
- [ ] Unit tests written
- [ ] Integration tests written (if applicable)
- [ ] Tests pass locally
- [ ] Tests pass in CI
- [ ] Coverage target met

#### Documentation
- [ ] Code comments added
- [ ] README updated (if applicable)
- [ ] API docs updated (if applicable)

#### Security
- [ ] No security vulnerabilities introduced
- [ ] No mock data in production paths
- [ ] Authentication/authorization correct

#### Review
- [ ] Self-review completed
- [ ] Peer review completed
- [ ] All review comments addressed
```

---

## How to Resume

This plan is designed to be pausable and resumable:

1. **Check Current Status**: Review the checkboxes above to see what's completed
2. **Pick Next Task**: Start with the lowest numbered incomplete task in the current phase
3. **Update Progress**: Check off subtasks as you complete them
4. **Verify**: Use the verification checklist before marking a task complete
5. **Update Status**: Update the overall status table at the top

### Commands for Progress Check

```bash
# Check current coverage
cd /run/media/milosvasic/DATA4TB/Projects/HelixCode/HelixCode
go test -cover ./...

# Run specific package tests
go test -v -cover ./internal/server/...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Metrics to Track

| Metric | Baseline | Current | Target |
|--------|----------|---------|--------|
| Total Coverage | 47.5% | 47.5% | 100% |
| Critical Issues | 15 | 15 | 0 |
| High Issues | 23 | 23 | 0 |
| Mock Data Instances | 12 | 12 | 0 |
| Deprecated Dependencies | 2 | 2 | 0 |

---

**Document Version**: 1.0
**Next Review Date**: TBD
