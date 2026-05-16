// Package core contains end-to-end tests for core HelixCode functionality
package core

import ()

// Hello returns a greeting message
func Hello() string {
	return "Hello from HelixCode E2E Core package"
}

// GetTestCount returns the number of available E2E tests
func GetTestCount() int {
	return 15 // Total number of comprehensive E2E tests
}

// GetTestNames returns the names of all available E2E tests
func GetTestNames() []string {
	return []string{
		"TestUserRegistration",
		"TestUserLoginLogout",
		"TestRoleBasedAccess",
		"TestProjectCreation",
		"TestProjectFileOperations",
		"TestProjectCollaboration",
		"TestTaskCreationExecution",
		"TestWorkflowAutomation",
		"TestTaskCheckpointingRecovery",
		"TestLLMProviderIntegration",
		"TestLLMModelManagement",
		"TestLLMContextMemory",
		"TestMultiProviderLLMIntegration",
		"TestMemorySystemIntegration",
		"TestNotificationSystemIntegration",
	}
}