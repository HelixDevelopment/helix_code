package testbank

import (
	"encoding/json"
	"fmt"
	"os"

	"dev.helix.code/tests/e2e/orchestrator/pkg"
	"dev.helix.code/tests/e2e/test-bank/core"
	"dev.helix.code/tests/e2e/test-bank/distributed"
	"dev.helix.code/tests/e2e/test-bank/integration"
	"dev.helix.code/tests/e2e/test-bank/platform"
)

// TestMetadata represents the metadata for a test case
type TestMetadata struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Category          string   `json:"category"`
	Priority          string   `json:"priority"`
	Tags              []string `json:"tags"`
	EstimatedDuration string   `json:"estimated_duration"`
	Dependencies      []string `json:"dependencies"`
	Timeout           string   `json:"timeout"`
	RetryCount        int      `json:"retry_count"`
	Platforms         []string `json:"platforms"`
	Description       string   `json:"description"`
	Preconditions     []string `json:"preconditions"`
	Steps             []string `json:"steps"`
	ExpectedResults   []string `json:"expected_results"`
}

// TestBank manages all test cases and their metadata
type TestBank struct {
	metadata map[string]*TestMetadata
	tests    map[string]*pkg.TestCase
}

// NewTestBank creates a new test bank instance
func NewTestBank() *TestBank {
	return &TestBank{
		metadata: make(map[string]*TestMetadata),
		tests:    make(map[string]*pkg.TestCase),
	}
}

// LoadMetadata loads test metadata from JSON files
func (tb *TestBank) LoadMetadata(metadataFile string) error {
	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadataList []*TestMetadata
	if err := json.Unmarshal(data, &metadataList); err != nil {
		return fmt.Errorf("failed to parse metadata JSON: %w", err)
	}

	for _, meta := range metadataList {
		tb.metadata[meta.ID] = meta
	}

	return nil
}

// LoadTests loads all test cases from different categories
func (tb *TestBank) LoadTests() error {
	// Load core tests (TC-001 to TC-010)
	coreTests := core.GetCoreTests()
	for _, test := range coreTests {
		tb.tests[test.ID] = test
	}

	// Load integration tests (IT-001 to IT-010)
	integrationTests := integration.GetIntegrationTests()
	for _, test := range integrationTests {
		tb.tests[test.ID] = test
	}

	// Load distributed tests (DT-001 to DT-010)
	distributedTests := distributed.GetDistributedTests()
	for _, test := range distributedTests {
		tb.tests[test.ID] = test
	}

	// Load platform tests (PT-001 to PT-012)
	platformTests := platform.GetPlatformTests()
	for _, test := range platformTests {
		tb.tests[test.ID] = test
	}

	return nil
}

// LoadCoreTests loads only core test cases
func (tb *TestBank) LoadCoreTests() error {
	coreTests := core.GetCoreTests()
	for _, test := range coreTests {
		tb.tests[test.ID] = test
	}
	return nil
}

// LoadIntegrationTests loads only integration test cases
func (tb *TestBank) LoadIntegrationTests() error {
	integrationTests := integration.GetIntegrationTests()
	for _, test := range integrationTests {
		tb.tests[test.ID] = test
	}
	return nil
}

// LoadDistributedTests loads only distributed test cases
func (tb *TestBank) LoadDistributedTests() error {
	distributedTests := distributed.GetDistributedTests()
	for _, test := range distributedTests {
		tb.tests[test.ID] = test
	}
	return nil
}

// LoadPlatformTests loads only platform test cases
func (tb *TestBank) LoadPlatformTests() error {
	platformTests := platform.GetPlatformTests()
	for _, test := range platformTests {
		tb.tests[test.ID] = test
	}
	return nil
}

// GetAllTests returns all loaded test cases
func (tb *TestBank) GetAllTests() []*pkg.TestCase {
	tests := make([]*pkg.TestCase, 0, len(tb.tests))
	for _, test := range tb.tests {
		tests = append(tests, test)
	}
	return tests
}

// GetTestsByCategory returns tests filtered by category
func (tb *TestBank) GetTestsByCategory(category string) []*pkg.TestCase {
	tests := make([]*pkg.TestCase, 0)
	for _, test := range tb.tests {
		// Check if test has the category tag
		for _, tag := range test.Tags {
			if tag == category {
				tests = append(tests, test)
				break
			}
		}
	}
	return tests
}

// GetTestsByTags returns tests that match all given tags
func (tb *TestBank) GetTestsByTags(tags []string) []*pkg.TestCase {
	tests := make([]*pkg.TestCase, 0)
	for _, test := range tb.tests {
		if tb.hasAllTags(test, tags) {
			tests = append(tests, test)
		}
	}
	return tests
}

// GetTestByID returns a specific test by ID
func (tb *TestBank) GetTestByID(id string) (*pkg.TestCase, bool) {
	test, found := tb.tests[id]
	return test, found
}

// GetMetadata returns metadata for a specific test
func (tb *TestBank) GetMetadata(id string) (*TestMetadata, bool) {
	meta, found := tb.metadata[id]
	return meta, found
}

// hasAllTags checks if a test has all the specified tags
func (tb *TestBank) hasAllTags(test *pkg.TestCase, tags []string) bool {
	testTagSet := make(map[string]bool)
	for _, tag := range test.Tags {
		testTagSet[tag] = true
	}

	for _, tag := range tags {
		if !testTagSet[tag] {
			return false
		}
	}

	return true
}

// GetTestSuite creates a test suite with all tests
func (tb *TestBank) GetTestSuite() *pkg.TestSuite {
	return &pkg.TestSuite{
		Name:        "HelixCode E2E Test Suite",
		Description: "Comprehensive end-to-end test suite for the HelixCode platform",
		Tests:       tb.GetAllTests(),
	}
}

// GetTestSuiteByCategory creates a test suite for a specific category
func (tb *TestBank) GetTestSuiteByCategory(category string) *pkg.TestSuite {
	return &pkg.TestSuite{
		Name:        fmt.Sprintf("HelixCode %s Tests", category),
		Description: fmt.Sprintf("%s test suite for the HelixCode platform", category),
		Tests:       tb.GetTestsByCategory(category),
	}
}

// GetCoreTestSuite returns a test suite with only core tests
func (tb *TestBank) GetCoreTestSuite() *pkg.TestSuite {
	tb.LoadCoreTests()
	return &pkg.TestSuite{
		Name:        "HelixCode Core Tests",
		Description: "Core functionality test suite for the HelixCode platform",
		Tests:       tb.GetTestsByCategory("core"),
	}
}

// GetIntegrationTestSuite returns a test suite with only integration tests
func (tb *TestBank) GetIntegrationTestSuite() *pkg.TestSuite {
	tb.LoadIntegrationTests()
	return &pkg.TestSuite{
		Name:        "HelixCode Integration Tests",
		Description: "Integration test suite for the HelixCode platform",
		Tests:       tb.GetTestsByCategory("integration"),
	}
}

// GetDistributedTestSuite returns a test suite with only distributed tests
func (tb *TestBank) GetDistributedTestSuite() *pkg.TestSuite {
	tb.LoadDistributedTests()
	return &pkg.TestSuite{
		Name:        "HelixCode Distributed Tests",
		Description: "Distributed computing test suite for the HelixCode platform",
		Tests:       tb.GetTestsByCategory("distributed"),
	}
}

// GetPlatformTestSuite returns a test suite with only platform tests
func (tb *TestBank) GetPlatformTestSuite() *pkg.TestSuite {
	tb.LoadPlatformTests()
	return &pkg.TestSuite{
		Name:        "HelixCode Platform Tests",
		Description: "Platform compatibility test suite for the HelixCode platform",
		Tests:       tb.GetTestsByCategory("platform"),
	}
}

// GetTestCount returns the total number of loaded tests
func (tb *TestBank) GetTestCount() int {
	return len(tb.tests)
}

// GetTestCountByCategory returns the number of tests in a category
func (tb *TestBank) GetTestCountByCategory(category string) int {
	return len(tb.GetTestsByCategory(category))
}

// GetTestSummary returns a summary of loaded tests
func (tb *TestBank) GetTestSummary() map[string]int {
	summary := make(map[string]int)
	summary["total"] = len(tb.tests)
	summary["core"] = tb.GetTestCountByCategory("core")
	summary["integration"] = tb.GetTestCountByCategory("integration")
	summary["distributed"] = tb.GetTestCountByCategory("distributed")
	summary["platform"] = tb.GetTestCountByCategory("platform")
	return summary
}
