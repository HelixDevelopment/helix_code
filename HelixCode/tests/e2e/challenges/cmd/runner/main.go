package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"dev.helix.code/tests/e2e/challenges"
)

func main() {
	// Command line flags
	var (
		challengesDir  = flag.String("challenges-dir", "./definitions", "Directory containing challenge definitions")
		challengeID    = flag.String("challenge", "", "Specific challenge ID to run")
		interfaces     = flag.String("interfaces", "cli", "Comma-separated list of interfaces (cli,tui,rest,websocket)")
		distributions  = flag.String("distributions", "single", "Comma-separated list of distributions (single,worker_2,worker_5,worker_10)")
		providers      = flag.String("providers", "ollama", "Comma-separated list of LLM providers")
		models         = flag.String("models", "llama2", "Comma-separated list of models")
		batchName      = flag.String("batch-name", "challenge-test-run", "Name for the batch")
		batchDesc      = flag.String("batch-desc", "Challenge test batch", "Description for the batch")
		resultsDir     = flag.String("results-dir", "./test-results/challenges", "Directory to store results")
		logsDir        = flag.String("logs-dir", "./test-results/logs", "Directory to store logs")
		maxConcurrent  = flag.Int("max-concurrent", 3, "Maximum concurrent executions")
		timeout        = flag.Duration("timeout", 45*time.Minute, "Default timeout for challenges")
		listChallenges = flag.Bool("list", false, "List all available challenges")
		verbose        = flag.Bool("verbose", false, "Enable verbose logging")
		saveState      = flag.Bool("save-state", true, "Save execution state to disk")
		stateDir       = flag.String("state-dir", "./test-results/state", "Directory to save state")
		exportReport   = flag.String("export-report", "", "Export batch report to file")
	)

	flag.Parse()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("Received interrupt signal, shutting down...")
		cancel()
	}()

	// Create configuration
	config := challenges.DefaultChallengeConfig()
	config.ResultsBaseDir = *resultsDir
	config.LogsBaseDir = *logsDir
	config.MaxConcurrent = *maxConcurrent
	config.DefaultTimeout = *timeout
	config.VerboseLogging = *verbose

	// Create challenge manager
	manager := challenges.NewChallengeManager(config)

	// Load challenges
	if err := manager.LoadChallengesFromDir(*challengesDir); err != nil {
		log.Fatalf("Failed to load challenges: %v", err)
	}

	// List challenges if requested
	if *listChallenges {
		listAllChallenges(manager)
		return
	}

	// Determine which challenges to run
	var challengeIDs []string
	if *challengeID != "" {
		challengeIDs = []string{*challengeID}
	} else {
		// Run all challenges
		allChallenges := manager.ListChallenges()
		for _, c := range allChallenges {
			challengeIDs = append(challengeIDs, c.ID)
		}
	}

	if len(challengeIDs) == 0 {
		log.Fatal("No challenges to run")
	}

	// Parse interfaces
	interfaceList := parseInterfaces(*interfaces)
	if len(interfaceList) == 0 {
		log.Fatal("No valid interfaces specified")
	}

	// Parse distributions
	distributionList := parseDistributions(*distributions)
	if len(distributionList) == 0 {
		log.Fatal("No valid distributions specified")
	}

	// Parse providers
	providerList := parseProviders(*providers)
	if len(providerList) == 0 {
		log.Fatal("No valid providers specified")
	}

	// Parse models
	modelList := strings.Split(*models, ",")
	for i := range modelList {
		modelList[i] = strings.TrimSpace(modelList[i])
	}

	// Create batch
	log.Printf("Creating batch: %s", *batchName)
	log.Printf("  Challenges: %v", challengeIDs)
	log.Printf("  Interfaces: %v", interfaceList)
	log.Printf("  Distributions: %v", distributionList)
	log.Printf("  Providers: %v", providerList)
	log.Printf("  Models: %v", modelList)

	batch, err := manager.CreateBatch(
		*batchName,
		*batchDesc,
		challengeIDs,
		interfaceList,
		distributionList,
		providerList,
		modelList,
	)
	if err != nil {
		log.Fatalf("Failed to create batch: %v", err)
	}

	log.Printf("Batch created with ID: %s", batch.ID)

	// Execute batch
	log.Println("Starting batch execution...")
	startTime := time.Now()

	if err := manager.ExecuteBatch(ctx, batch.ID); err != nil {
		log.Printf("Batch execution completed with errors: %v", err)
	} else {
		log.Println("Batch execution completed successfully")
	}

	duration := time.Since(startTime)

	// Print summary
	printBatchSummary(batch, duration)

	// Save state if requested
	if *saveState {
		log.Printf("Saving state to %s...", *stateDir)
		if err := manager.SaveState(*stateDir); err != nil {
			log.Printf("Failed to save state: %v", err)
		} else {
			log.Println("State saved successfully")
		}
	}

	// Export report if requested
	if *exportReport != "" {
		log.Printf("Exporting report to %s...", *exportReport)
		if err := manager.ExportBatchReport(batch.ID, *exportReport); err != nil {
			log.Printf("Failed to export report: %v", err)
		} else {
			log.Println("Report exported successfully")
		}
	}

	// Exit with appropriate code
	if batch.Summary.Failed > 0 || batch.Summary.ValidationFailed > 0 {
		os.Exit(1)
	}
}

func listAllChallenges(manager *challenges.ChallengeManager) {
	allChallenges := manager.ListChallenges()

	if len(allChallenges) == 0 {
		fmt.Println("No challenges found")
		return
	}

	fmt.Printf("Available Challenges (%d):\n\n", len(allChallenges))

	for _, c := range allChallenges {
		fmt.Printf("ID:          %s\n", c.ID)
		fmt.Printf("Name:        %s\n", c.Name)
		fmt.Printf("Type:        %s\n", c.Type)
		fmt.Printf("Language:    %s\n", c.Language)
		fmt.Printf("Priority:    %d\n", c.Priority)
		fmt.Printf("Timeout:     %v\n", c.Timeout)
		fmt.Printf("Tags:        %s\n", strings.Join(c.Tags, ", "))
		fmt.Printf("Description: %s\n", c.Description)
		fmt.Println(strings.Repeat("-", 80))
	}
}

func printBatchSummary(batch *challenges.ChallengeBatch, duration time.Duration) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("BATCH EXECUTION SUMMARY")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("Batch ID:         %s\n", batch.ID)
	fmt.Printf("Batch Name:       %s\n", batch.Name)
	fmt.Printf("Status:           %s\n", batch.Status)
	fmt.Printf("Duration:         %v\n", duration)
	fmt.Println()
	fmt.Printf("Total Executions: %d\n", batch.Summary.TotalExecutions)
	fmt.Printf("  Completed:      %d\n", batch.Summary.Completed)
	fmt.Printf("  Failed:         %d\n", batch.Summary.Failed)
	fmt.Printf("  Timeout:        %d\n", batch.Summary.Timeout)
	fmt.Printf("  Val. Failed:    %d\n", batch.Summary.ValidationFailed)
	fmt.Println()
	fmt.Printf("Success Rate:     %.2f%%\n", batch.Summary.SuccessRate)
	fmt.Printf("Avg Duration:     %v\n", batch.Summary.AvgDuration)
	fmt.Printf("Total Tokens:     %d\n", batch.Summary.TotalTokens)
	fmt.Printf("Files Generated:  %d\n", batch.Summary.TotalFilesGenerated)
	fmt.Printf("Total LOC:        %d\n", batch.Summary.TotalLOC)
	fmt.Println(strings.Repeat("=", 80))
}

func parseInterfaces(s string) []challenges.ChallengeInterface {
	parts := strings.Split(s, ",")
	var result []challenges.ChallengeInterface

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		switch part {
		case "cli":
			result = append(result, challenges.InterfaceCLI)
		case "tui":
			result = append(result, challenges.InterfaceTUI)
		case "rest":
			result = append(result, challenges.InterfaceREST)
		case "websocket", "ws":
			result = append(result, challenges.InterfaceWebSocket)
		case "desktop":
			result = append(result, challenges.InterfaceDesktop)
		}
	}

	return result
}

func parseDistributions(s string) []challenges.ChallengeDistribution {
	parts := strings.Split(s, ",")
	var result []challenges.ChallengeDistribution

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		switch part {
		case "single":
			result = append(result, challenges.DistributionSingle)
		case "worker_2", "2":
			result = append(result, challenges.DistributionWorker2)
		case "worker_5", "5":
			result = append(result, challenges.DistributionWorker5)
		case "worker_10", "10":
			result = append(result, challenges.DistributionWorker10)
		}
	}

	return result
}

func parseProviders(s string) []challenges.LLMProviderType {
	parts := strings.Split(s, ",")
	var result []challenges.LLMProviderType

	for _, part := range parts {
		part = strings.TrimSpace(strings.ToLower(part))
		switch part {
		case "ollama":
			result = append(result, challenges.ProviderOllama)
		case "llamacpp", "llama.cpp":
			result = append(result, challenges.ProviderLlamaCpp)
		case "vllm":
			result = append(result, challenges.ProviderVLLM)
		case "localai":
			result = append(result, challenges.ProviderLocalAI)
		case "openai":
			result = append(result, challenges.ProviderOpenAI)
		case "anthropic", "claude":
			result = append(result, challenges.ProviderAnthropic)
		case "gemini":
			result = append(result, challenges.ProviderGemini)
		case "mistral":
			result = append(result, challenges.ProviderMistral)
		case "qwen":
			result = append(result, challenges.ProviderQwen)
		case "groq":
			result = append(result, challenges.ProviderGroq)
		case "azure":
			result = append(result, challenges.ProviderAzure)
		case "bedrock":
			result = append(result, challenges.ProviderBedrock)
		case "vertexai":
			result = append(result, challenges.ProviderVertexAI)
		case "openrouter":
			result = append(result, challenges.ProviderOpenRouter)
		}
	}

	return result
}
