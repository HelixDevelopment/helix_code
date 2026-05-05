package main

// wizard_cmd.go (P1-F12-T09): cobra subcommand surface for `helixcode wizard`.
//
// The wizard has two modes:
//
//   1. Interactive (default): launches the tview TUI from internal/llm.RunWizard,
//      writes the resulting WizardResult to $XDG_CONFIG_HOME/helixcode/llm.yaml
//      via WriteWizardConfig (mode 0600 + O_EXCL). If a config already exists,
//      the user is prompted on stdin (y/N) before OverwriteWizardConfig replaces
//      it. We intentionally use a stdin prompt rather than a tview modal here —
//      the cobra entry point must not depend on a TTY for the confirmation
//      step, and a plain reader works for piped input (CI / scripts that
//      explicitly want to overwrite can pre-pipe "y\n").
//
//   2. Non-interactive: when --provider is supplied, the TUI is skipped
//      entirely. We construct a WizardResult from the supplied flags, validate
//      via internal/llm.validateWizardForm (exposed indirectly through
//      buildWizardResult preconditions), and write to disk. This mirrors the
//      WizardConfig.NonInteractiveResult escape hatch documented in T08.
//
// Anti-bluff anchor: this file performs real disk I/O and shells nothing —
// no simulation, no fake "wizard saved" without a successful WriteWizardConfig
// or OverwriteWizardConfig call returning nil.

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"dev.helix.code/internal/llm"
)

// wizardCmdDeps wires test seams for the "wizard" cobra subcommand. All
// fields are optional; nil values fall back to production defaults.
type wizardCmdDeps struct {
	// ConfigPathOverride forces a specific YAML path (tests use this to
	// route writes into t.TempDir()). Empty -> use the default
	// XDG/$HOME-resolved location from the wizard package.
	ConfigPathOverride string

	// EnvLookup overrides os.Getenv (tests use this to inject XDG_CONFIG_HOME
	// without touching the real environment).
	EnvLookup func(string) string

	// Stdin overrides os.Stdin for the y/N overwrite prompt. Tests pass
	// strings.NewReader("y\n").
	Stdin io.Reader

	// Stdout overrides os.Stdout (tests use bytes.Buffer).
	Stdout io.Writer

	// RunWizardFn overrides llm.RunWizard for tests. Production: nil.
	RunWizardFn func(ctx context.Context, cfg llm.WizardConfig) (*llm.WizardResult, error)

	// WriteFn / OverwriteFn override the disk writers for tests. Production: nil.
	WriteFn     func(path string, result *llm.WizardResult) error
	OverwriteFn func(path string, result *llm.WizardResult) error
}

// newWizardCmd builds the `helixcode wizard` cobra subcommand. Mirrors the
// structure of newSessionsCmd / newSkillsCmd / newCommandsCmd: thin command
// surface + dependencies struct so tests can drive it with stubs.
func newWizardCmd(deps wizardCmdDeps) *cobra.Command {
	var (
		providerFlag   string
		apiKeyFlag     string
		regionFlag     string
		endpointFlag   string
		projectFlag    string
		locationFlag   string
		apiVersionFlag string
		forceFlag      bool
	)

	cmd := &cobra.Command{
		Use:   "wizard",
		Short: "First-run setup for cloud LLM providers (Anthropic / Bedrock / Vertex / Azure)",
		Long: "Launch the interactive provider-setup wizard, or use --provider with the\n" +
			"per-provider flags to write a config without TUI input. The resulting\n" +
			"YAML is saved to $XDG_CONFIG_HOME/helixcode/llm.yaml with mode 0600.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWizardCmd(cmd.Context(), deps, wizardCmdFlags{
				Provider:   providerFlag,
				APIKey:     apiKeyFlag,
				Region:     regionFlag,
				Endpoint:   endpointFlag,
				Project:    projectFlag,
				Location:   locationFlag,
				APIVersion: apiVersionFlag,
				Force:      forceFlag,
			})
		},
	}
	cmd.Flags().StringVar(&providerFlag, "provider", "", "non-interactive: provider type (anthropic|bedrock|vertexai|azure)")
	cmd.Flags().StringVar(&apiKeyFlag, "api-key", "", "API key (anthropic, azure)")
	cmd.Flags().StringVar(&regionFlag, "region", "", "AWS region (bedrock)")
	cmd.Flags().StringVar(&endpointFlag, "endpoint", "", "endpoint URL (azure)")
	cmd.Flags().StringVar(&projectFlag, "project", "", "GCP project ID (vertexai)")
	cmd.Flags().StringVar(&locationFlag, "location", "", "GCP location (vertexai)")
	cmd.Flags().StringVar(&apiVersionFlag, "api-version", "", "API version (azure)")
	cmd.Flags().BoolVar(&forceFlag, "force", false, "overwrite existing config without prompting")
	return cmd
}

type wizardCmdFlags struct {
	Provider   string
	APIKey     string
	Region     string
	Endpoint   string
	Project    string
	Location   string
	APIVersion string
	Force      bool
}

func runWizardCmd(ctx context.Context, deps wizardCmdDeps, flags wizardCmdFlags) error {
	stdout := deps.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stdin := deps.Stdin
	if stdin == nil {
		stdin = os.Stdin
	}
	envLookup := deps.EnvLookup
	if envLookup == nil {
		envLookup = os.Getenv
	}

	configPath := deps.ConfigPathOverride
	if configPath == "" {
		configPath = defaultConfigPathFromEnv(envLookup)
	}

	wizardCfg := llm.WizardConfig{
		ConfigPath: configPath,
		EnvLookup:  envLookup,
	}

	// Non-interactive path: --provider supplied -> build NonInteractiveResult
	// and skip the TUI entirely. This path is what tests + CI use.
	if strings.TrimSpace(flags.Provider) != "" {
		result, err := buildNonInteractiveResult(flags, configPath)
		if err != nil {
			return err
		}
		wizardCfg.NonInteractiveResult = result
	}

	runFn := deps.RunWizardFn
	if runFn == nil {
		runFn = llm.RunWizard
	}

	result, err := runFn(ctx, wizardCfg)
	if err != nil {
		return fmt.Errorf("wizard: %w", err)
	}
	if result == nil || result.Cancelled {
		fmt.Fprintln(stdout, "wizard cancelled; no changes written.")
		return nil
	}

	if result.ConfigPath == "" {
		result.ConfigPath = configPath
	}

	writeFn := deps.WriteFn
	if writeFn == nil {
		writeFn = llm.WriteWizardConfig
	}
	overwriteFn := deps.OverwriteFn
	if overwriteFn == nil {
		overwriteFn = llm.OverwriteWizardConfig
	}

	// Try the create-only path first. On ErrExist, fall back to either a
	// stdin y/N prompt or a forced overwrite.
	if err := writeFn(result.ConfigPath, result); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("wizard: write config: %w", err)
		}
		// Existing file. Decide whether to overwrite.
		if !flags.Force {
			fmt.Fprintf(stdout, "Config already exists at %s. Overwrite? [y/N]: ", result.ConfigPath)
			ans, readErr := readSingleLine(stdin)
			if readErr != nil {
				return fmt.Errorf("wizard: read overwrite prompt: %w", readErr)
			}
			if !isYes(ans) {
				fmt.Fprintln(stdout, "wizard: keeping existing config; no changes written.")
				return nil
			}
		}
		if err := overwriteFn(result.ConfigPath, result); err != nil {
			return fmt.Errorf("wizard: overwrite config: %w", err)
		}
	}

	fmt.Fprintf(stdout, "wizard: wrote provider %q to %s\n", result.ProviderType, result.ConfigPath)
	return nil
}

// buildNonInteractiveResult assembles a WizardResult from flag-supplied values
// without going through the TUI. Returns an error if --provider is unknown or
// the supplied fields fail provider-specific validation.
func buildNonInteractiveResult(flags wizardCmdFlags, configPath string) (*llm.WizardResult, error) {
	ptype, err := llm.ParseCloudProviderType(flags.Provider)
	if err != nil {
		return nil, fmt.Errorf("wizard: %w", err)
	}

	params := map[string]interface{}{}
	entry := llm.ProviderConfigEntry{
		Type:       ptype,
		Enabled:    true,
		Parameters: params,
	}

	switch ptype {
	case llm.ProviderTypeAnthropic:
		if strings.TrimSpace(flags.APIKey) == "" {
			return nil, errors.New("wizard: --api-key is required for anthropic")
		}
		entry.APIKey = flags.APIKey
		params["api_key"] = flags.APIKey
	case llm.ProviderTypeBedrock:
		if strings.TrimSpace(flags.Region) == "" {
			return nil, errors.New("wizard: --region is required for bedrock")
		}
		params["region"] = flags.Region
	case llm.ProviderTypeVertexAI:
		if strings.TrimSpace(flags.Project) == "" {
			return nil, errors.New("wizard: --project is required for vertexai")
		}
		if strings.TrimSpace(flags.Location) == "" {
			return nil, errors.New("wizard: --location is required for vertexai")
		}
		params["project_id"] = flags.Project
		params["location"] = flags.Location
	case llm.ProviderTypeAzure:
		if strings.TrimSpace(flags.Endpoint) == "" {
			return nil, errors.New("wizard: --endpoint is required for azure")
		}
		if strings.TrimSpace(flags.APIKey) == "" {
			return nil, errors.New("wizard: --api-key is required for azure")
		}
		entry.APIKey = flags.APIKey
		params["endpoint"] = flags.Endpoint
		params["api_key"] = flags.APIKey
		if strings.TrimSpace(flags.APIVersion) != "" {
			params["api_version"] = flags.APIVersion
		} else {
			params["api_version"] = "2024-08-01-preview"
		}
	}

	return &llm.WizardResult{
		ProviderType: ptype,
		ConfigEntry:  entry,
		ConfigPath:   configPath,
		Cancelled:    false,
	}, nil
}

// readSingleLine reads up to the first newline from r and trims whitespace.
func readSingleLine(r io.Reader) (string, error) {
	br := bufio.NewReader(r)
	line, err := br.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func isYes(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	}
	return false
}
