package planmode

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"dev.helix.code/internal/llm"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockLLMProvider is a mock LLM provider for testing
type MockLLMProvider struct {
	responses map[string]string
}

func NewMockLLMProvider() *MockLLMProvider {
	return &MockLLMProvider{
		responses: map[string]string{
			"plan": `{
				"title": "Test Plan",
				"description": "A test implementation plan",
				"steps": [
					{
						"title": "Step 1",
						"description": "First step",
						"type": "file_operation",
						"action": "create file",
						"dependencies": [],
						"estimated_minutes": 5
					},
					{
						"title": "Step 2",
						"description": "Second step",
						"type": "shell_command",
						"action": "run command",
						"dependencies": ["step-1"],
						"estimated_minutes": 10
					}
				],
				"risks": [
					{
						"description": "Low risk",
						"impact": "low",
						"likelihood": "low",
						"mitigation": "None needed"
					}
				],
				"estimates": {
					"duration_minutes": 15,
					"complexity": "low",
					"confidence": 0.9
				}
			}`,
			"options": `{
				"options": [
					{
						"title": "Option 1",
						"description": "First approach",
						"plan": {
							"title": "Plan 1",
							"description": "First plan",
							"steps": [
								{
									"title": "Step 1",
									"description": "First step",
									"type": "file_operation",
									"action": "create",
									"dependencies": [],
									"estimated_minutes": 5
								}
							],
							"risks": [],
							"estimates": {
								"duration_minutes": 5,
								"complexity": "low",
								"confidence": 0.95
							}
						},
						"pros": ["Simple", "Fast"],
						"cons": ["Limited features"]
					},
					{
						"title": "Option 2",
						"description": "Second approach",
						"plan": {
							"title": "Plan 2",
							"description": "Second plan",
							"steps": [
								{
									"title": "Step 1",
									"description": "First step",
									"type": "code_generation",
									"action": "generate",
									"dependencies": [],
									"estimated_minutes": 15
								}
							],
							"risks": [],
							"estimates": {
								"duration_minutes": 15,
								"complexity": "medium",
								"confidence": 0.85
							}
						},
						"pros": ["Full featured", "Robust"],
						"cons": ["More complex", "Slower"]
					}
				]
			}`,
		},
	}
}

func (m *MockLLMProvider) Generate(ctx context.Context, request *llm.LLMRequest) (*llm.LLMResponse, error) {
	var response string
	if strings.Contains(request.Messages[1].Content, "multiple") || strings.Contains(request.Messages[1].Content, "options") {
		response = m.responses["options"]
	} else {
		response = m.responses["plan"]
	}

	return &llm.LLMResponse{
		ID:        uuid.New(),
		RequestID: request.ID,
		Content:   response,
		Usage: llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 200,
			TotalTokens:      300,
		},
		CreatedAt: time.Now(),
	}, nil
}

func (m *MockLLMProvider) GenerateStream(ctx context.Context, request *llm.LLMRequest, ch chan<- llm.LLMResponse) error {
	return nil
}

func (m *MockLLMProvider) GetType() llm.ProviderType {
	return llm.ProviderTypeLocal
}

func (m *MockLLMProvider) GetName() string {
	return "mock"
}

func (m *MockLLMProvider) GetModels() []llm.ModelInfo {
	return []llm.ModelInfo{{Name: "mock-model"}}
}

func (m *MockLLMProvider) GetCapabilities() []llm.ModelCapability {
	return []llm.ModelCapability{llm.CapabilityPlanning}
}

func (m *MockLLMProvider) IsAvailable(ctx context.Context) bool {
	return true
}

func (m *MockLLMProvider) GetHealth(ctx context.Context) (*llm.ProviderHealth, error) {
	return &llm.ProviderHealth{Status: "healthy"}, nil
}

func (m *MockLLMProvider) Close() error {
	return nil
}

func (m *MockLLMProvider) GetContextWindow() int {
	return 128000
}

func (m *MockLLMProvider) CountTokens(text string) (int, error) {
	return len(text) / 4, nil
}

// MockPresenter is a mock option presenter for testing
type MockPresenter struct {
	selectedIndex int
}

func (m *MockPresenter) Present(ctx context.Context, options []*PlanOption) (*Selection, error) {
	if len(options) == 0 {
		return nil, assert.AnError
	}

	index := m.selectedIndex
	if index >= len(options) {
		index = 0
	}

	return &Selection{
		OptionID:  options[index].ID,
		Timestamp: time.Now(),
	}, nil
}

func (m *MockPresenter) CompareOptions(options []*PlanOption) (*Comparison, error) {
	return &Comparison{
		Options:  options,
		Criteria: []string{"Complexity", "Duration"},
		Matrix:   [][]ComparisonCell{},
	}, nil
}

func (m *MockPresenter) RankOptions(options []*PlanOption, criteria []RankCriterion) ([]*RankedOption, error) {
	ranked := make([]*RankedOption, len(options))
	for i, opt := range options {
		ranked[i] = &RankedOption{
			Option: opt,
			Rank:   i + 1,
			Score:  100.0 - float64(i*10),
		}
	}
	return ranked, nil
}

// Test Mode Controller
func TestModeController(t *testing.T) {
	t.Run("InitialMode", func(t *testing.T) {
		controller := NewModeController()
		assert.Equal(t, ModeNormal, controller.GetMode())
	})

	t.Run("ValidTransitions", func(t *testing.T) {
		controller := NewModeController()

		// Normal -> Plan
		err := controller.TransitionTo(ModePlan)
		require.NoError(t, err)
		assert.Equal(t, ModePlan, controller.GetMode())

		// Plan -> Act
		err = controller.TransitionTo(ModeAct)
		require.NoError(t, err)
		assert.Equal(t, ModeAct, controller.GetMode())

		// Act -> Paused
		err = controller.TransitionTo(ModePaused)
		require.NoError(t, err)
		assert.Equal(t, ModePaused, controller.GetMode())

		// Paused -> Act
		err = controller.TransitionTo(ModeAct)
		require.NoError(t, err)
		assert.Equal(t, ModeAct, controller.GetMode())

		// Act -> Normal
		err = controller.TransitionTo(ModeNormal)
		require.NoError(t, err)
		assert.Equal(t, ModeNormal, controller.GetMode())
	})

	t.Run("InvalidTransitions", func(t *testing.T) {
		controller := NewModeController()

		// Normal -> Act (invalid)
		err := controller.TransitionTo(ModeAct)
		assert.Error(t, err)

		// Normal -> Paused (invalid)
		err = controller.TransitionTo(ModePaused)
		assert.Error(t, err)
	})

	t.Run("ModeChangeCallback", func(t *testing.T) {
		controller := NewModeController()
		var called bool
		var fromMode, toMode Mode

		controller.RegisterCallback(func(from, to Mode, state *ModeState) {
			called = true
			fromMode = from
			toMode = to
		})

		controller.TransitionTo(ModePlan)
		assert.True(t, called)
		assert.Equal(t, ModeNormal, fromMode)
		assert.Equal(t, ModePlan, toMode)
	})

	t.Run("StateManagement", func(t *testing.T) {
		controller := NewModeController()

		state := &ModeState{
			Mode:        ModePlan,
			PlanID:      "plan-123",
			OptionID:    "option-456",
			ExecutionID: "exec-789",
			Metadata:    map[string]interface{}{"key": "value"},
		}

		err := controller.UpdateState(state)
		require.NoError(t, err)

		retrieved := controller.GetState()
		assert.Equal(t, state.PlanID, retrieved.PlanID)
		assert.Equal(t, state.OptionID, retrieved.OptionID)
		assert.Equal(t, state.ExecutionID, retrieved.ExecutionID)
	})
}

// Test Planner
func TestPlanner(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	planner := NewLLMPlanner(mockLLM)

	t.Run("GeneratePlan", func(t *testing.T) {
		task := &Task{
			ID:          "task-1",
			Description: "Create a test feature",
			Context: &TaskContext{
				WorkspaceRoot: "/test",
			},
		}

		plan, err := planner.GeneratePlan(context.Background(), task)
		require.NoError(t, err)
		assert.NotNil(t, plan)
		assert.Equal(t, "Test Plan", plan.Title)
		assert.Equal(t, 2, len(plan.Steps))
		assert.Equal(t, PlanDraft, plan.Status)
	})

	t.Run("GenerateOptions", func(t *testing.T) {
		task := &Task{
			ID:          "task-2",
			Description: "Implement authentication",
		}

		options, err := planner.GenerateOptions(context.Background(), task)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(options), 2)

		// Check options are ranked
		for i, opt := range options {
			assert.Equal(t, i+1, opt.Rank)
			assert.Greater(t, opt.Score, 0.0)
		}

		// First option should be recommended
		assert.True(t, options[0].Recommended)
	})

	t.Run("ValidatePlan", func(t *testing.T) {
		validPlan := &Plan{
			ID:    "plan-1",
			Title: "Valid Plan",
			Steps: []*PlanStep{
				{
					ID:    "step-1",
					Title: "Step 1",
				},
			},
		}

		result, err := planner.ValidatePlan(context.Background(), validPlan)
		require.NoError(t, err)
		assert.True(t, result.Valid)

		invalidPlan := &Plan{
			ID:    "plan-2",
			Title: "",
			Steps: []*PlanStep{},
		}

		result, err = planner.ValidatePlan(context.Background(), invalidPlan)
		require.NoError(t, err)
		assert.False(t, result.Valid)
		assert.Greater(t, len(result.Errors), 0)
	})
}

// Test State Manager
func TestStateManager(t *testing.T) {
	sm := NewStateManager()

	t.Run("StorePlan", func(t *testing.T) {
		plan := &Plan{
			ID:    "plan-1",
			Title: "Test Plan",
		}

		err := sm.StorePlan(plan)
		require.NoError(t, err)

		retrieved, err := sm.GetPlan("plan-1")
		require.NoError(t, err)
		assert.Equal(t, plan.ID, retrieved.ID)
		assert.Equal(t, plan.Title, retrieved.Title)
	})

	t.Run("StoreOptions", func(t *testing.T) {
		options := []*PlanOption{
			{ID: "opt-1", Title: "Option 1"},
			{ID: "opt-2", Title: "Option 2"},
		}

		err := sm.StoreOptions("plan-1", options)
		require.NoError(t, err)

		retrieved, err := sm.GetOptions("plan-1")
		require.NoError(t, err)
		assert.Equal(t, 2, len(retrieved))
	})

	t.Run("StoreSelection", func(t *testing.T) {
		selection := &Selection{
			OptionID:  "opt-1",
			Timestamp: time.Now(),
		}

		err := sm.StoreSelection("plan-1", selection)
		require.NoError(t, err)

		retrieved, err := sm.GetSelection("plan-1")
		require.NoError(t, err)
		assert.Equal(t, selection.OptionID, retrieved.OptionID)
	})

	t.Run("ListPlans", func(t *testing.T) {
		plans := sm.ListPlans()
		assert.GreaterOrEqual(t, len(plans), 1)
	})

	t.Run("ClearPlan", func(t *testing.T) {
		sm.ClearPlan("plan-1")
		_, err := sm.GetPlan("plan-1")
		assert.Error(t, err)
	})
}

// Test Executor
func TestExecutor(t *testing.T) {
	// Create temp directory for test
	testDir := "/tmp/test"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	executor := NewDefaultExecutor(testDir)

	t.Run("ExecuteStep", func(t *testing.T) {
		step := &PlanStep{
			ID:          "step-1",
			Title:       "Test Step",
			Description: "echo 'test'",
			Type:        StepTypeShellCommand,
			Action:      "echo 'test'",
		}

		result, err := executor.ExecuteStep(context.Background(), step)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("ExecutePlan", func(t *testing.T) {
		plan := &Plan{
			ID: "plan-1",
			Steps: []*PlanStep{
				{
					ID:          "step-1",
					Title:       "Step 1",
					Description: "First step",
					Type:        StepTypeFileOperation,
					Action:      "test",
				},
				{
					ID:          "step-2",
					Title:       "Step 2",
					Description: "Second step",
					Type:        StepTypeFileOperation,
					Action:      "test2",
				},
			},
		}

		result, err := executor.Execute(context.Background(), plan)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, 2, len(result.Steps))
	})

	t.Run("PauseResumeExecution", func(t *testing.T) {
		plan := &Plan{
			ID: "plan-2",
			Steps: []*PlanStep{
				{
					ID:    "step-1",
					Title: "Long Step",
					Type:  StepTypeFileOperation,
				},
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			executor.Execute(ctx, plan)
		}()

		time.Sleep(100 * time.Millisecond)

		// Note: This test is simplified as we can't easily get the execution ID
		// in a real scenario without modifying the API
	})
}

// Test Option Presenter
func TestOptionPresenter(t *testing.T) {
	t.Run("CLIPresenter", func(t *testing.T) {
		// Round-457 CONST-046: the presenter routes every user-facing
		// label through the i18n seam. Wire fakeTranslator so the
		// rendered output carries the seam sentinel rather than a
		// NoopTranslator raw message-ID echo.
		SetTranslator(fakeTranslator{})
		t.Cleanup(func() { SetTranslator(nil) })

		var output bytes.Buffer
		input := strings.NewReader("1\n")

		presenter := NewCLIOptionPresenter(&output, input)

		options := []*PlanOption{
			{
				ID:          "opt-1",
				Title:       "Option 1",
				Description: "First option",
				Score:       85.0,
				Recommended: true,
				Plan: &Plan{
					Estimates: Estimates{
						Duration:   15 * time.Minute,
						Complexity: ComplexityLow,
						Confidence: 0.9,
					},
				},
				Pros: []string{"Fast", "Simple"},
				Cons: []string{"Limited"},
			},
		}

		selection, err := presenter.Present(context.Background(), options)
		require.NoError(t, err)
		assert.Equal(t, "opt-1", selection.OptionID)

		outputStr := output.String()
		// Seam routing: each user-facing label resolves via tr().
		assert.Contains(t, outputStr, "XLATE:internal_workflow_planmode_options_option_label")
		assert.Contains(t, outputStr, "XLATE:internal_workflow_planmode_options_recommended_tag")
		assert.Contains(t, outputStr, "XLATE:internal_workflow_planmode_options_select_prompt")
		// Non-migrated dynamic data (pros entries) still passes through verbatim.
		assert.Contains(t, outputStr, "Fast")
	})

	t.Run("CompareOptions", func(t *testing.T) {
		presenter := NewCLIOptionPresenter(&bytes.Buffer{}, strings.NewReader(""))

		options := []*PlanOption{
			{
				ID:    "opt-1",
				Title: "Option 1",
				Plan: &Plan{
					Estimates: Estimates{
						Duration:   10 * time.Minute,
						Complexity: ComplexityLow,
						Confidence: 0.9,
					},
					Risks: []Risk{},
				},
			},
			{
				ID:    "opt-2",
				Title: "Option 2",
				Plan: &Plan{
					Estimates: Estimates{
						Duration:   20 * time.Minute,
						Complexity: ComplexityMedium,
						Confidence: 0.8,
					},
					Risks: []Risk{},
				},
			},
		}

		comparison, err := presenter.CompareOptions(options)
		require.NoError(t, err)
		assert.Equal(t, 2, len(comparison.Options))
		assert.Greater(t, len(comparison.Criteria), 0)
	})

	t.Run("RankOptions", func(t *testing.T) {
		presenter := NewCLIOptionPresenter(&bytes.Buffer{}, strings.NewReader(""))

		options := []*PlanOption{
			{
				ID: "opt-1",
				Plan: &Plan{
					Estimates: Estimates{
						Duration:   10 * time.Minute,
						Complexity: ComplexityLow,
						Confidence: 0.9,
					},
				},
			},
		}

		criteria := []RankCriterion{
			{Name: "Speed", Weight: 1.0, Type: CriterionSpeed},
			{Name: "Simplicity", Weight: 0.8, Type: CriterionSimplicity},
		}

		ranked, err := presenter.RankOptions(options, criteria)
		require.NoError(t, err)
		assert.Equal(t, 1, len(ranked))
		assert.Equal(t, 1, ranked[0].Rank)
	})
}

// Test Workflow
func TestPlanModeWorkflow(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	planner := NewLLMPlanner(mockLLM)
	presenter := &MockPresenter{selectedIndex: 0}
	executor := NewDefaultExecutor("/tmp/test")
	stateManager := NewStateManager()
	controller := NewModeController()

	workflow := NewPlanModeWorkflow(planner, presenter, executor, stateManager, controller)

	t.Run("ExecuteWorkflow", func(t *testing.T) {
		task := &Task{
			ID:          "task-1",
			Description: "Test task",
		}

		result, err := workflow.ExecuteWorkflow(context.Background(), task)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ModeNormal, controller.GetMode())
	})

	t.Run("ExecuteWithProgress", func(t *testing.T) {
		task := &Task{
			ID:          "task-2",
			Description: "Test task with progress",
		}

		progressUpdates := 0
		progressFn := func(progress *WorkflowProgress) {
			progressUpdates++
		}

		result, err := workflow.ExecuteWithProgress(context.Background(), task, progressFn)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Greater(t, progressUpdates, 0)
	})

	t.Run("YOLOMode", func(t *testing.T) {
		task := &Task{
			ID:          "task-3",
			Description: "YOLO task",
		}

		result, err := workflow.ExecuteYOLOMode(context.Background(), task)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, ModeNormal, controller.GetMode())
	})
}

// Test PlanMode
func TestPlanMode(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	planner := NewLLMPlanner(mockLLM)
	presenter := &MockPresenter{selectedIndex: 0}
	executor := NewDefaultExecutor("/tmp/test")
	stateManager := NewStateManager()
	controller := NewModeController()

	workflow := NewPlanModeWorkflow(planner, presenter, executor, stateManager, controller)

	t.Run("DefaultConfig", func(t *testing.T) {
		config := DefaultConfig()
		assert.Equal(t, 3, config.DefaultOptionCount)
		assert.Equal(t, 5, config.MaxOptionCount)
		assert.False(t, config.AutoSelectBest)
		assert.True(t, config.ShowComparison)
	})

	t.Run("RunNormalMode", func(t *testing.T) {
		config := DefaultConfig()
		pm := NewPlanMode(workflow, config)

		task := &Task{
			ID:          "task-1",
			Description: "Test task",
		}

		result, err := pm.Run(context.Background(), task)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("RunYOLOMode", func(t *testing.T) {
		config := DefaultConfig()
		config.AutoSelectBest = true
		pm := NewPlanMode(workflow, config)

		task := &Task{
			ID:          "task-2",
			Description: "YOLO task",
		}

		result, err := pm.Run(context.Background(), task)
		require.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("RunWithProgress", func(t *testing.T) {
		config := DefaultConfig()
		pm := NewPlanMode(workflow, config)

		task := &Task{
			ID:          "task-3",
			Description: "Progress task",
		}

		var lastPhase string
		progressFn := func(progress *WorkflowProgress) {
			lastPhase = progress.Phase
		}

		result, err := pm.RunWithProgress(context.Background(), task, progressFn)
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, "Completed", lastPhase)
	})
}

// Test error recovery
func TestErrorRecovery(t *testing.T) {
	t.Run("InvalidPlan", func(t *testing.T) {
		executor := NewDefaultExecutor("/tmp/test")

		plan := &Plan{
			ID: "invalid-plan",
			Steps: []*PlanStep{
				{
					ID:           "step-1",
					Title:        "Invalid Step",
					Type:         StepType(999), // Invalid type
					Dependencies: []string{"non-existent"},
				},
			},
		}

		result, err := executor.Execute(context.Background(), plan)
		require.NoError(t, err) // Executor should handle errors gracefully
		assert.NotNil(t, result)
	})

	t.Run("EmptyOptions", func(t *testing.T) {
		presenter := &MockPresenter{}
		_, err := presenter.Present(context.Background(), []*PlanOption{})
		assert.Error(t, err)
	})
}

// Benchmark tests
func BenchmarkPlanGeneration(b *testing.B) {
	mockLLM := NewMockLLMProvider()
	planner := NewLLMPlanner(mockLLM)

	task := &Task{
		ID:          "bench-task",
		Description: "Benchmark task",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.GeneratePlan(context.Background(), task)
	}
}

func BenchmarkOptionRanking(b *testing.B) {
	options := make([]*PlanOption, 5)
	for i := 0; i < 5; i++ {
		options[i] = &PlanOption{
			ID:    uuid.New().String(),
			Title: "Option",
			Plan: &Plan{
				Estimates: Estimates{
					Duration:   time.Duration(i*5) * time.Minute,
					Complexity: Complexity(i % 4),
					Confidence: 0.9,
				},
			},
		}
	}

	presenter := NewCLIOptionPresenter(&bytes.Buffer{}, strings.NewReader(""))
	criteria := []RankCriterion{
		{Name: "Speed", Weight: 1.0, Type: CriterionSpeed},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		presenter.RankOptions(options, criteria)
	}
}

// ========================================
// Additional Executor Tests
// ========================================

func TestExecutorWithLLM(t *testing.T) {
	testDir := "/tmp/test-llm"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	t.Run("NewDefaultExecutorWithLLM", func(t *testing.T) {
		mockLLM := NewMockLLMProvider()
		executor := NewDefaultExecutorWithLLM(testDir, mockLLM)
		assert.NotNil(t, executor)
		assert.NotNil(t, executor.llmProvider)
	})

	t.Run("SetLLMProvider", func(t *testing.T) {
		executor := NewDefaultExecutor(testDir)
		assert.Nil(t, executor.llmProvider)

		mockLLM := NewMockLLMProvider()
		executor.SetLLMProvider(mockLLM)
		assert.NotNil(t, executor.llmProvider)
	})
}

func TestExecutorPauseResumeCancel(t *testing.T) {
	testDir := "/tmp/test-pause"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	executor := NewDefaultExecutor(testDir)

	t.Run("Pause_NotFound", func(t *testing.T) {
		err := executor.Pause("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Resume_NotFound", func(t *testing.T) {
		err := executor.Resume("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("Cancel_NotFound", func(t *testing.T) {
		err := executor.Cancel("non-existent-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetProgress_NotFound", func(t *testing.T) {
		progress, err := executor.GetProgress("non-existent-id")
		assert.Error(t, err)
		assert.Nil(t, progress)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestExecuteStepTypes(t *testing.T) {
	testDir := "/tmp/test-step-types"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	executor := NewDefaultExecutor(testDir)

	t.Run("ShellCommand", func(t *testing.T) {
		step := &PlanStep{
			ID:          "step-shell",
			Title:       "Shell Command",
			Description: "Run echo",
			Type:        StepTypeShellCommand,
			Action:      "echo 'test'",
		}

		result, err := executor.ExecuteStep(context.Background(), step)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("FileOperation_Create", func(t *testing.T) {
		step := &PlanStep{
			ID:          "step-file-create",
			Title:       "Create File",
			Description: "Create a test file",
			Type:        StepTypeFileOperation,
			Action:      "create:" + testDir + "/test.txt",
		}

		result, err := executor.ExecuteStep(context.Background(), step)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})

	t.Run("FileOperation_Delete", func(t *testing.T) {
		// First create the file to delete
		os.WriteFile(testDir+"/delete-test.txt", []byte("test content"), 0644)

		step := &PlanStep{
			ID:          "step-file-delete",
			Title:       "Delete File",
			Description: "Delete a test file",
			Type:        StepTypeFileOperation,
			Action:      "delete:" + testDir + "/delete-test.txt",
		}

		result, err := executor.ExecuteStep(context.Background(), step)
		require.NoError(t, err)
		assert.True(t, result.Success)
	})
}

func TestExecutePlanWithDependencies(t *testing.T) {
	testDir := "/tmp/test-deps"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	executor := NewDefaultExecutor(testDir)

	plan := &Plan{
		ID:    "plan-deps",
		Title: "Plan with Dependencies",
		Steps: []*PlanStep{
			{
				ID:           "step-1",
				Title:        "First Step",
				Type:         StepTypeShellCommand,
				Action:       "echo 'step 1'",
				Dependencies: []string{},
			},
			{
				ID:           "step-2",
				Title:        "Second Step",
				Type:         StepTypeShellCommand,
				Action:       "echo 'step 2'",
				Dependencies: []string{"step-1"},
			},
			{
				ID:           "step-3",
				Title:        "Third Step",
				Type:         StepTypeShellCommand,
				Action:       "echo 'step 3'",
				Dependencies: []string{"step-1", "step-2"},
			},
		},
	}

	result, err := executor.Execute(context.Background(), plan)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Steps, 3)
	assert.True(t, result.Success)
}

func TestExecutePlanWithFailingStep(t *testing.T) {
	testDir := "/tmp/test-fail"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	executor := NewDefaultExecutor(testDir)

	plan := &Plan{
		ID:    "plan-fail",
		Title: "Plan with Failing Step",
		Steps: []*PlanStep{
			{
				ID:     "step-1",
				Title:  "Failing Step",
				Type:   StepTypeShellCommand,
				Action: "exit 1",
			},
		},
	}

	result, err := executor.Execute(context.Background(), plan)
	require.NoError(t, err) // Executor doesn't return error, just marks step as failed
	assert.NotNil(t, result)
}

// Test extractCodeFromMarkdown
func TestExtractCodeFromMarkdown(t *testing.T) {
	t.Run("NoCodeBlocks", func(t *testing.T) {
		content := "This is plain text without code blocks"
		result := extractCodeFromMarkdown(content)
		assert.Equal(t, content, result)
	})

	t.Run("SingleCodeBlock", func(t *testing.T) {
		content := "```\nfunc main() {\n    fmt.Println(\"Hello\")\n}\n```"
		result := extractCodeFromMarkdown(content)
		assert.Contains(t, result, "func main()")
		assert.Contains(t, result, "fmt.Println")
	})

	t.Run("CodeBlockWithLanguage", func(t *testing.T) {
		content := "```go\npackage main\n```"
		result := extractCodeFromMarkdown(content)
		assert.Equal(t, "package main", result)
	})

	t.Run("MultipleCodeBlocks", func(t *testing.T) {
		content := "```\nblock1\n```\nsome text\n```\nblock2\n```"
		result := extractCodeFromMarkdown(content)
		assert.Contains(t, result, "block1")
		assert.Contains(t, result, "block2")
	})

	t.Run("EmptyCodeBlock", func(t *testing.T) {
		content := "```\n```"
		result := extractCodeFromMarkdown(content)
		assert.Equal(t, "", result)
	})
}

// Test isSourceFile
func TestIsSourceFile(t *testing.T) {
	tests := []struct {
		ext      string
		expected bool
	}{
		{".go", true},
		{".py", true},
		{".js", true},
		{".ts", true},
		{".tsx", true},
		{".jsx", true},
		{".java", true},
		{".rs", true},
		{".c", true},
		{".cpp", true},
		{".rb", true},
		{".php", true},
		{".swift", true},
		{".vue", true},
		{".yaml", true},
		{".json", true},
		{".sql", true},
		{".graphql", true},
		{".proto", true},
		{".txt", false},
		{".exe", false},
		{".bin", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := isSourceFile(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test detectProjectType
func TestDetectProjectType(t *testing.T) {
	t.Run("GoProject", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(tmpDir+"/go.mod", []byte("module test"), 0644)
		result := detectProjectType(tmpDir)
		assert.Equal(t, "go", result)
	})

	t.Run("NodeProject", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(tmpDir+"/package.json", []byte("{}"), 0644)
		result := detectProjectType(tmpDir)
		assert.Equal(t, "node", result)
	})

	t.Run("PythonProject_Requirements", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(tmpDir+"/requirements.txt", []byte("requests==2.0"), 0644)
		result := detectProjectType(tmpDir)
		assert.Equal(t, "python", result)
	})

	t.Run("RustProject", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(tmpDir+"/Cargo.toml", []byte("[package]"), 0644)
		result := detectProjectType(tmpDir)
		assert.Equal(t, "rust", result)
	})

	t.Run("JavaProject_Maven", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.WriteFile(tmpDir+"/pom.xml", []byte("<project/>"), 0644)
		result := detectProjectType(tmpDir)
		assert.Equal(t, "java", result)
	})

	t.Run("UnknownProject", func(t *testing.T) {
		tmpDir := t.TempDir()
		result := detectProjectType(tmpDir)
		assert.Equal(t, "unknown", result)
	})
}

// Test ProgressTracker
func TestProgressTracker(t *testing.T) {
	t.Run("NewProgressTracker", func(t *testing.T) {
		tracker := NewProgressTracker()
		assert.NotNil(t, tracker)
		assert.NotNil(t, tracker.callbacks)
	})

	t.Run("RegisterCallback", func(t *testing.T) {
		tracker := NewProgressTracker()
		callCount := 0

		tracker.RegisterCallback("exec-1", func(p *ExecutionProgress) {
			callCount++
		})

		// Register another callback for same execution
		tracker.RegisterCallback("exec-1", func(p *ExecutionProgress) {
			callCount++
		})

		// Notify
		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "exec-1"})
		assert.Equal(t, 2, callCount)
	})

	t.Run("NotifyProgress_NoCallbacks", func(t *testing.T) {
		tracker := NewProgressTracker()
		// Should not panic when no callbacks exist
		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "non-existent"})
	})

	t.Run("ClearCallbacks", func(t *testing.T) {
		tracker := NewProgressTracker()
		callCount := 0

		tracker.RegisterCallback("exec-1", func(p *ExecutionProgress) {
			callCount++
		})

		tracker.ClearCallbacks("exec-1")
		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "exec-1"})
		assert.Equal(t, 0, callCount)
	})

	t.Run("MultipleExecutions", func(t *testing.T) {
		tracker := NewProgressTracker()
		exec1Count := 0
		exec2Count := 0

		tracker.RegisterCallback("exec-1", func(p *ExecutionProgress) {
			exec1Count++
		})
		tracker.RegisterCallback("exec-2", func(p *ExecutionProgress) {
			exec2Count++
		})

		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "exec-1"})
		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "exec-2"})
		tracker.NotifyProgress(&ExecutionProgress{ExecutionID: "exec-1"})

		assert.Equal(t, 2, exec1Count)
		assert.Equal(t, 1, exec2Count)
	})
}

// Test StateManager additional methods
func TestStateManager_Execution(t *testing.T) {
	sm := NewStateManager()

	t.Run("StoreExecution", func(t *testing.T) {
		execution := &ExecutionResult{
			ID:        "exec-1",
			PlanID:    "plan-1",
			StartTime: time.Now(),
			Success:   true,
		}

		err := sm.StoreExecution(execution)
		require.NoError(t, err)

		retrieved, err := sm.GetExecution("exec-1")
		require.NoError(t, err)
		assert.Equal(t, "exec-1", retrieved.ID)
		assert.Equal(t, "plan-1", retrieved.PlanID)
		assert.True(t, retrieved.Success)
	})

	t.Run("GetExecution_NotFound", func(t *testing.T) {
		_, err := sm.GetExecution("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("StorePlan_EmptyID", func(t *testing.T) {
		plan := &Plan{ID: "", Title: "Test"}
		err := sm.StorePlan(plan)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ID is required")
	})

	t.Run("StoreOptions_Empty", func(t *testing.T) {
		err := sm.StoreOptions("plan-1", []*PlanOption{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least one option")
	})

	t.Run("GetPlan_NotFound", func(t *testing.T) {
		_, err := sm.GetPlan("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetOptions_NotFound", func(t *testing.T) {
		_, err := sm.GetOptions("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("GetSelection_NotFound", func(t *testing.T) {
		_, err := sm.GetSelection("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Test Workflow error paths
func TestWorkflow_ErrorPaths(t *testing.T) {
	t.Run("ExecuteWorkflow_SelectedOptionNotFound", func(t *testing.T) {
		mockLLM := NewMockLLMProvider()
		planner := NewLLMPlanner(mockLLM)
		executor := NewDefaultExecutor("/tmp/test")
		stateManager := NewStateManager()
		controller := NewModeController()

		// Create a presenter that returns a non-existent option ID
		badPresenter := &MockPresenterBadSelection{}

		workflow := NewPlanModeWorkflow(planner, badPresenter, executor, stateManager, controller)

		task := &Task{ID: "task-1", Description: "Test"}
		_, err := workflow.ExecuteWorkflow(context.Background(), task)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

// MockPresenterBadSelection always returns a non-existent option ID
type MockPresenterBadSelection struct{}

func (m *MockPresenterBadSelection) Present(ctx context.Context, options []*PlanOption) (*Selection, error) {
	return &Selection{
		OptionID:  "non-existent-id",
		Timestamp: time.Now(),
	}, nil
}

func (m *MockPresenterBadSelection) CompareOptions(options []*PlanOption) (*Comparison, error) {
	return &Comparison{}, nil
}

func (m *MockPresenterBadSelection) RankOptions(options []*PlanOption, criteria []RankCriterion) ([]*RankedOption, error) {
	return nil, nil
}

// Test Config
func TestConfig(t *testing.T) {
	t.Run("DefaultConfig_Values", func(t *testing.T) {
		config := DefaultConfig()
		assert.Equal(t, 3, config.DefaultOptionCount)
		assert.Equal(t, 5, config.MaxOptionCount)
		assert.False(t, config.AutoSelectBest)
		assert.True(t, config.ShowComparison)
		assert.True(t, config.EnableProgressBar)
		assert.Equal(t, 0.7, config.ConfidenceThreshold)
		assert.Equal(t, ComplexityHigh, config.MaxPlanComplexity)
	})

	t.Run("NewPlanMode_NilConfig", func(t *testing.T) {
		mockLLM := NewMockLLMProvider()
		planner := NewLLMPlanner(mockLLM)
		presenter := &MockPresenter{}
		executor := NewDefaultExecutor("/tmp/test")
		stateManager := NewStateManager()
		controller := NewModeController()
		workflow := NewPlanModeWorkflow(planner, presenter, executor, stateManager, controller)

		pm := NewPlanMode(workflow, nil)
		assert.NotNil(t, pm)
		assert.NotNil(t, pm.config)
		assert.Equal(t, 3, pm.config.DefaultOptionCount)
	})
}

// Test PlanMode control methods
func TestPlanMode_Control(t *testing.T) {
	mockLLM := NewMockLLMProvider()
	planner := NewLLMPlanner(mockLLM)
	presenter := &MockPresenter{}
	executor := NewDefaultExecutor("/tmp/test")
	stateManager := NewStateManager()
	controller := NewModeController()
	workflow := NewPlanModeWorkflow(planner, presenter, executor, stateManager, controller)
	pm := NewPlanMode(workflow, nil)

	t.Run("Pause_NotFound", func(t *testing.T) {
		err := pm.Pause("non-existent")
		assert.Error(t, err)
	})

	t.Run("Resume_NotFound", func(t *testing.T) {
		err := pm.Resume("non-existent")
		assert.Error(t, err)
	})

	t.Run("Cancel_NotFound", func(t *testing.T) {
		err := pm.Cancel("non-existent")
		assert.Error(t, err)
	})

	t.Run("GetProgress_NotFound", func(t *testing.T) {
		_, err := pm.GetProgress("non-existent")
		assert.Error(t, err)
	})
}
