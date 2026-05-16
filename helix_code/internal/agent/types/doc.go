// Package types provides specialized agent implementations for different task categories.
//
// Each agent type is optimized for a specific class of work, implementing the Agent
// interface from the parent agent package. These agents work together through the
// coordinator to handle complex multi-step tasks.
//
// # Agent Types
//
// The package provides five specialized agent implementations:
//
// CodingAgent handles code generation and modification tasks. It uses LLM providers
// to generate code based on requirements and applies changes through the tool registry.
// CodingAgent can collaborate with ReviewAgent to validate generated code.
//
// PlanningAgent performs strategic planning and task decomposition. It breaks down
// complex requests into smaller, manageable tasks and creates execution plans.
//
// ReviewAgent performs code review and quality analysis. It can analyze code for
// issues, suggest improvements, and validate changes made by other agents.
//
// TestingAgent generates and executes tests. It creates test cases based on
// requirements and validates that code behaves correctly.
//
// DebuggingAgent finds and fixes bugs. It analyzes error reports, traces issues
// through code paths, and implements fixes.
//
// # Usage
//
// Agents are typically created through factory functions and registered with
// the coordinator:
//
//	codingAgent, err := types.NewCodingAgent(cfg, llmProvider, toolRegistry)
//	if err != nil {
//	    return err
//	}
//	coordinator.RegisterAgent(codingAgent)
//
// # Collaboration
//
// Agents can collaborate on tasks through the Collaborate method. For example,
// CodingAgent automatically requests reviews from ReviewAgent when available:
//
//	result, err := codingAgent.Collaborate(ctx, []agent.Agent{reviewAgent}, task)
package types
