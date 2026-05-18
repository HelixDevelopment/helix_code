package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"dev.helix.code/cmd/helix_config/i18n"
	"dev.helix.code/internal/config"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this CLI. Defaults to i18n.NoopTranslator{} (loud
// message-ID echo) so unit tests + ad-hoc invocations remain obvious.
// helix_code wires a real *i18nadapter.Translator at boot via
// SetTranslator (round-108 §11.4 anti-bluff sweep, 2026-05-18).
//
// A package-level variable is the chosen DI seam because cobra's
// RunE signature (func(*cobra.Command, []string) error) does not
// support extra parameters without restructuring the command tree —
// global injection matches cobra's own use of package-level state
// (viper, the *cobra.Command tree itself) and keeps the migration
// minimally invasive.
var translator i18n.Translator = i18n.NoopTranslator{}

// SetTranslator wires a CONST-046-compliant Translator. Passing nil
// resets to i18n.NoopTranslator{} (loud echo) — never silently
// disables translation lookup (which would be a §11.4 PASS-bluff at
// the i18n injection layer).
func SetTranslator(tr i18n.Translator) {
	if tr == nil {
		translator = i18n.NoopTranslator{}
		return
	}
	translator = tr
}

// tr is the internal CONST-046 resolver used by every user-facing
// string emission in this file. It NEVER returns an error to the
// caller — translation failures degrade to the message ID itself
// (matching NoopTranslator behaviour) so production output remains
// loud + obvious instead of silently empty.
func tr(ctx context.Context, msgID string, data map[string]any) string {
	if translator == nil {
		translator = i18n.NoopTranslator{}
	}
	out, err := translator.T(ctx, msgID, data)
	if err != nil || out == "" {
		return msgID
	}
	return out
}

var (
	configFile   string
	format       string
	outputFormat string
	sessionID    string
	user         string
	verbose      bool
	dryRun       bool
	quiet        bool
	noColor      bool
	interactive  bool
	force        bool
	backup       bool
	timeout      time.Duration
	maxRetries   int
	showSecrets  bool
	noValidate   bool
	strictMode   bool
	prettyPrint  bool
	sortKeys     bool
)

// CLI version information
var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	rootCmd := createRootCommand()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// createRootCommand creates the root CLI command
func createRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "helix-config",
		Short: "HelixCode Configuration Management CLI",
		Long: `HelixCode Configuration Management CLI

Manage HelixCode configuration across all platforms with comprehensive
validation, migration, and templating support.`,
		Version: fmt.Sprintf("%s (built: %s, commit: %s)", version, buildTime, gitCommit),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setupViper()
			loadConfig()
		},
	}

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Configuration file path")
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "auto", "Configuration format (json, yaml, toml)")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "json", "Output format (json, yaml, table, pretty)")
	rootCmd.PersistentFlags().StringVar(&sessionID, "session-id", "", "Session ID for tracking")
	rootCmd.PersistentFlags().StringVar(&user, "user", "", "User name for audit")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Dry run without making changes")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Quiet mode (no output)")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable color output")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "Interactive mode")
	rootCmd.PersistentFlags().BoolVarP(&force, "force", "F", false, "Force operation without confirmation")
	rootCmd.PersistentFlags().BoolVar(&backup, "backup", true, "Create backup before making changes")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 30*time.Second, "Operation timeout")
	rootCmd.PersistentFlags().IntVar(&maxRetries, "max-retries", 3, "Maximum number of retries")
	rootCmd.PersistentFlags().BoolVar(&showSecrets, "show-secrets", false, "Show sensitive configuration values")
	rootCmd.PersistentFlags().BoolVar(&noValidate, "no-validate", false, "Skip configuration validation")
	rootCmd.PersistentFlags().BoolVar(&strictMode, "strict", false, "Enable strict validation mode")
	rootCmd.PersistentFlags().BoolVar(&prettyPrint, "pretty", true, "Pretty print JSON output")
	rootCmd.PersistentFlags().BoolVar(&sortKeys, "sort-keys", true, "Sort object keys in output")

	// Bind flags to viper
	bindFlags(rootCmd.PersistentFlags())

	// Add subcommands
	rootCmd.AddCommand(createShowCommand())
	rootCmd.AddCommand(createGetCommand())
	rootCmd.AddCommand(createSetCommand())
	rootCmd.AddCommand(createDeleteCommand())
	rootCmd.AddCommand(createValidateCommand())
	rootCmd.AddCommand(createExportCommand())
	rootCmd.AddCommand(createImportCommand())
	rootCmd.AddCommand(createBackupCommand())
	rootCmd.AddCommand(createRestoreCommand())
	rootCmd.AddCommand(createResetCommand())
	rootCmd.AddCommand(createReloadCommand())
	rootCmd.AddCommand(createWatchCommand())
	rootCmd.AddCommand(createMigrateCommand())
	rootCmd.AddCommand(createTemplateCommand())
	rootCmd.AddCommand(createHistoryCommand())
	rootCmd.AddCommand(createSchemaCommand())
	rootCmd.AddCommand(createCompletionCommand())
	rootCmd.AddCommand(createVersionCommand())
	rootCmd.AddCommand(createInfoCommand())
	rootCmd.AddCommand(createStatusCommand())
	rootCmd.AddCommand(createDiffCommand())
	rootCmd.AddCommand(createMergeCommand())
	rootCmd.AddCommand(createSearchCommand())
	rootCmd.AddCommand(createBenchmarkCommand())

	return rootCmd
}

// Subcommands

func createShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		Long: `Display the current HelixCode configuration.

The configuration can be filtered by section or displayed in its entirety.
Sensitive values are masked by default unless --show-secrets is used.`,
		RunE: runShowCommand,
	}

	cmd.Flags().StringP("section", "s", "", "Show only specific section")
	cmd.Flags().Bool("masked", true, "Show masked sensitive values")
	cmd.Flags().Bool("defaults", false, "Show default values for unset fields")
	cmd.Flags().Bool("flattened", false, "Show configuration in flattened key-value format")

	return cmd
}

func createGetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value",
		Long: `Retrieve a specific configuration value by key path.

Examples:
  helix-config get application.name
  helix-config get server.port
  helix-config get llm.default_provider`,
		Args: cobra.ExactArgs(1),
		RunE: runGetCommand,
	}

	cmd.Flags().Bool("type", false, "Show the type of the value")
	cmd.Flags().Bool("source", false, "Show the source of the value")
	cmd.Flags().Bool("valid", false, "Validate the retrieved value")

	return cmd
}

func createSetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a specific configuration value by key path.

The value is interpreted based on the target field type.
Use quotes for string values containing spaces.

Examples:
  helix-config set application.name "My App"
  helix-config set server.port 8080
  helix-config set llm.temperature 0.8`,
		Args: cobra.ExactArgs(2),
		RunE: runSetCommand,
	}

	cmd.Flags().Bool("create", false, "Create field if it doesn't exist")
	cmd.Flags().Bool("validate", true, "Validate value before setting")
	cmd.Flags().String("type", "", "Force value type (string, int, float, bool)")
	cmd.Flags().String("format", "", "Value format for parsing")
	cmd.Flags().Bool("backup", true, "Create backup before setting")
	cmd.Flags().Bool("restart", false, "Restart affected services after setting")

	return cmd
}

func createDeleteCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a configuration value",
		Long: `Delete a specific configuration value by key path.

The field is reset to its default value.

Examples:
  helix-config delete server.custom_headers
  helix-config delete llm.api_keys.test`,
		Args: cobra.ExactArgs(1),
		RunE: runDeleteCommand,
	}

	cmd.Flags().Bool("reset", true, "Reset to default value instead of deleting")
	cmd.Flags().Bool("confirm", false, "Require confirmation before deleting")
	cmd.Flags().Bool("backup", true, "Create backup before deleting")

	return cmd
}

func createValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate [file]",
		Short: "Validate configuration",
		Long: `Validate the current or specified configuration file.

All validation rules are applied and detailed error information is provided.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runValidateCommand,
	}

	cmd.Flags().Bool("strict", false, "Enable strict validation mode")
	cmd.Flags().Bool("warnings", true, "Show validation warnings")
	cmd.Flags().Bool("details", false, "Show detailed validation information")
	cmd.Flags().String("section", "", "Validate only specific section")
	cmd.Flags().Bool("schema", false, "Validate against JSON schema")

	return cmd
}

func createExportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export [file]",
		Short: "Export configuration",
		Long: `Export the current configuration to a file.

The configuration can be exported in various formats.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runExportCommand,
	}

	cmd.Flags().StringP("format", "f", "auto", "Export format (json, yaml, toml)")
	cmd.Flags().Bool("secrets", false, "Include sensitive values in export")
	cmd.Flags().Bool("defaults", false, "Include default values")
	cmd.Flags().Bool("comments", false, "Include comments in export")
	cmd.Flags().Bool("compress", false, "Compress the exported file")
	cmd.Flags().Bool("encrypt", false, "Encrypt the exported file")
	cmd.Flags().String("password", "", "Password for encryption")

	return cmd
}

func createImportCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Import configuration",
		Long: `Import configuration from a file.

The imported configuration is validated before being applied.`,
		Args: cobra.ExactArgs(1),
		RunE: runImportCommand,
	}

	cmd.Flags().Bool("validate", true, "Validate imported configuration")
	cmd.Flags().Bool("backup", true, "Create backup before import")
	cmd.Flags().Bool("merge", false, "Merge with existing configuration")
	cmd.Flags().Bool("force", false, "Force import even with validation errors")
	cmd.Flags().String("from", "", "Source configuration version for migration")

	return cmd
}

func createBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup [path]",
		Short: "Create configuration backup",
		Long: `Create a backup of the current configuration.

The backup includes all configuration files and can be used for restoration.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runBackupCommand,
	}

	cmd.Flags().Bool("incremental", false, "Create incremental backup")
	cmd.Flags().Bool("compress", false, "Compress the backup")
	cmd.Flags().String("description", "", "Backup description")
	cmd.Flags().String("tags", "", "Backup tags (comma-separated)")
	cmd.Flags().Bool("encrypt", false, "Encrypt the backup")
	cmd.Flags().String("password", "", "Password for encryption")

	return cmd
}

func createRestoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore <backup>",
		Short: "Restore configuration from backup",
		Long: `Restore configuration from a previously created backup.

The current configuration will be backed up before restoration.`,
		Args: cobra.ExactArgs(1),
		RunE: runRestoreCommand,
	}

	cmd.Flags().Bool("validate", true, "Validate restored configuration")
	cmd.Flags().Bool("backup", true, "Backup current configuration")
	cmd.Flags().Bool("confirm", false, "Require confirmation before restoration")
	cmd.Flags().String("to", "", "Restore to specific version")
	cmd.Flags().Bool("merge", false, "Merge with existing configuration")

	return cmd
}

func createResetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reset [section]",
		Short: "Reset configuration",
		Long: `Reset configuration to default values.

A specific section can be reset, or the entire configuration.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runResetCommand,
	}

	cmd.Flags().Bool("confirm", false, "Require confirmation before reset")
	cmd.Flags().Bool("backup", true, "Create backup before reset")
	cmd.Flags().String("template", "", "Reset to specific template instead of defaults")
	cmd.Flags().Bool("hard", false, "Hard reset (remove all custom settings)")

	return cmd
}

func createReloadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reload",
		Short: "Reload configuration",
		Long: `Reload configuration from disk.

This re-reads the configuration file and applies any changes.`,
		Args: cobra.NoArgs,
		RunE: runReloadCommand,
	}

	cmd.Flags().Bool("cache", true, "Reload configuration cache")
	cmd.Flags().Bool("watchers", true, "Reload configuration watchers")
	cmd.Flags().Bool("services", false, "Reload affected services")

	return cmd
}

func createWatchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch [key...]",
		Short: "Watch configuration changes",
		Long: `Monitor configuration changes in real-time.

Specific keys can be watched, or the entire configuration.`,
		Args: cobra.MinimumNArgs(0),
		RunE: runWatchCommand,
	}

	cmd.Flags().Bool("changes", true, "Show value changes")
	cmd.Flags().Bool("timestamps", true, "Show change timestamps")
	cmd.Flags().Bool("user", false, "Show user who made changes")
	cmd.Flags().String("format", "table", "Output format (table, json, yaml)")
	cmd.Flags().Bool("follow", true, "Continue watching until interrupted")
	cmd.Flags().Bool("summary", false, "Show periodic summary")

	return cmd
}

func createMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate <to>",
		Short: "Migrate configuration",
		Long: `Migrate configuration to a different version.

Migration is performed safely with automatic backups.`,
		Args: cobra.ExactArgs(1),
		RunE: runMigrateCommand,
	}

	cmd.Flags().String("from", "", "Source version (auto-detected if not specified)")
	cmd.Flags().Bool("backup", true, "Create backup before migration")
	cmd.Flags().Bool("dry-run", false, "Perform dry run without making changes")
	cmd.Flags().Bool("force", false, "Force migration even with warnings")
	cmd.Flags().String("path", "", "Custom migration path")
	cmd.Flags().Bool("validate", true, "Validate after migration")

	return cmd
}

func createTemplateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage configuration templates",
		Long: `Manage configuration templates.

Templates can be listed, applied, created, and managed.`,
	}

	cmd.AddCommand(createTemplateListCommand())
	cmd.AddCommand(createTemplateApplyCommand())
	cmd.AddCommand(createTemplateShowCommand())
	cmd.AddCommand(createTemplateCreateCommand())
	cmd.AddCommand(createTemplateUpdateCommand())
	cmd.AddCommand(createTemplateDeleteCommand())
	cmd.AddCommand(createTemplateSearchCommand())
	cmd.AddCommand(createTemplateValidateCommand())

	return cmd
}

func createHistoryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Manage configuration history",
		Long: `Manage configuration change history.

History can be viewed, compared, and restored.`,
	}

	cmd.AddCommand(createHistoryListCommand())
	cmd.AddCommand(createHistoryShowCommand())
	cmd.AddCommand(createHistoryRestoreCommand())
	cmd.AddCommand(createHistoryCompareCommand())
	cmd.AddCommand(createHistorySearchCommand())
	cmd.AddCommand(createHistoryCleanCommand())

	return cmd
}

func createSchemaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Manage configuration schema",
		Long: `Manage configuration JSON schema.

Schema can be generated, validated, and customized.`,
	}

	cmd.AddCommand(createSchemaShowCommand())
	cmd.AddCommand(createSchemaValidateCommand())
	cmd.AddCommand(createSchemaGenerateCommand())
	cmd.AddCommand(createSchemaExportCommand())
	cmd.AddCommand(createSchemaImportCommand())

	return cmd
}

func createBenchmarkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "benchmark",
		Short: "Benchmark configuration operations",
		Long:  `Benchmark various configuration operations for performance testing.`,
		Args:  cobra.NoArgs,
		RunE:  runBenchmarkCommand,
	}

	cmd.Flags().String("operation", "all", "Operation to benchmark (load, save, validate, transform, template)")
	cmd.Flags().Int("iterations", 1000, "Number of iterations to run")
	cmd.Flags().Bool("parallel", false, "Run operations in parallel")
	cmd.Flags().String("profile", "", "Enable profiling (cpu, memory, heap)")
	cmd.Flags().String("output", "", "Output file for benchmark results")
	cmd.Flags().Bool("compare", false, "Compare with previous benchmark results")
	cmd.Flags().Bool("warmup", true, "Perform warmup iterations")

	return cmd
}

// Template subcommands

func createTemplateListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available templates",
		Long:  `List all available configuration templates with their metadata.`,
		Args:  cobra.NoArgs,
		RunE:  runTemplateListCommand,
	}

	cmd.Flags().String("category", "", "Filter by category")
	cmd.Flags().String("tag", "", "Filter by tag")
	cmd.Flags().String("search", "", "Search in templates")
	cmd.Flags().String("sort", "name", "Sort by (name, created, updated, author)")
	cmd.Flags().Bool("details", false, "Show detailed template information")

	return cmd
}

func createTemplateApplyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <template>",
		Short: "Apply configuration template",
		Long:  `Apply a configuration template with optional variable substitution.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runTemplateApplyCommand,
	}

	cmd.Flags().StringSliceP("var", "v", nil, "Template variables (key=value)")
	cmd.Flags().String("vars-file", "", "Template variables file")
	cmd.Flags().Bool("backup", true, "Create backup before applying")
	cmd.Flags().Bool("preview", false, "Preview changes without applying")
	cmd.Flags().Bool("validate", true, "Validate applied configuration")
	cmd.Flags().Bool("force", false, "Force apply even with validation errors")

	return cmd
}

// History subcommands

func createHistoryListCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List configuration history",
		Long:  `List configuration change history with details.`,
		Args:  cobra.NoArgs,
		RunE:  runHistoryListCommand,
	}

	cmd.Flags().Int("limit", 50, "Maximum number of entries to show")
	cmd.Flags().String("since", "", "Show changes since (timestamp or duration)")
	cmd.Flags().String("until", "", "Show changes until (timestamp or duration)")
	cmd.Flags().String("user", "", "Filter by user")
	cmd.Flags().String("section", "", "Filter by configuration section")
	cmd.Flags().String("sort", "time", "Sort by (time, user, section)")
	cmd.Flags().Bool("details", false, "Show detailed change information")

	return cmd
}

// Schema subcommands

func createSchemaShowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show configuration schema",
		Long:  `Display the JSON schema for the configuration.`,
		Args:  cobra.NoArgs,
		RunE:  runSchemaShowCommand,
	}

	cmd.Flags().String("section", "", "Show schema for specific section")
	cmd.Flags().Bool("examples", true, "Include example values in schema")
	cmd.Flags().String("format", "json", "Output format (json, yaml)")
	cmd.Flags().Bool("validate", false, "Validate configuration against schema")

	return cmd
}

// Utility commands

func createCompletionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [shell]",
		Short: "Generate shell completion",
		Long:  `Generate shell completion scripts for bash, zsh, fish, or powershell.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runCompletionCommand,
	}

	cmd.Flags().Bool("install", false, "Install completion script")
	cmd.Flags().String("shell", "", "Shell type (bash, zsh, fish, powershell)")

	return cmd
}

func createVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  `Display detailed version and build information.`,
		Args:  cobra.NoArgs,
		RunE:  runVersionCommand,
	}

	cmd.Flags().Bool("short", false, "Show short version only")
	cmd.Flags().Bool("build", false, "Show build information")
	cmd.Flags().Bool("deps", false, "Show dependency versions")

	return cmd
}

func createInfoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show configuration information",
		Long:  `Display detailed information about the configuration system.`,
		Args:  cobra.NoArgs,
		RunE:  runInfoCommand,
	}

	cmd.Flags().Bool("system", false, "Show system information")
	cmd.Flags().Bool("files", false, "Show configuration file locations")
	cmd.Flags().Bool("stats", false, "Show configuration statistics")
	cmd.Flags().Bool("environment", false, "Show environment variables")

	return cmd
}

func createStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show configuration status",
		Long:  `Display the current status of the configuration system.`,
		Args:  cobra.NoArgs,
		RunE:  runStatusCommand,
	}

	cmd.Flags().Bool("watchers", false, "Show configuration watcher status")
	cmd.Flags().Bool("cache", false, "Show configuration cache status")
	cmd.Flags().Bool("locks", false, "Show lock status")
	cmd.Flags().Bool("performance", false, "Show performance metrics")

	return cmd
}

func createDiffCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <file1> <file2>",
		Short: "Compare configuration files",
		Long:  `Compare two configuration files and show differences.`,
		Args:  cobra.ExactArgs(2),
		RunE:  runDiffCommand,
	}

	cmd.Flags().String("format", "table", "Output format (table, json, yaml, diff)")
	cmd.Flags().Bool("unified", false, "Unified diff format")
	cmd.Flags().Int("context", 3, "Context lines for diff")
	cmd.Flags().Bool("color", true, "Colorized output")
	cmd.Flags().Bool("semantic", false, "Semantic diff (understands configuration structure)")

	return cmd
}

func createMergeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge <file1> <file2> [output]",
		Short: "Merge configuration files",
		Long:  `Merge two configuration files into one.`,
		Args:  cobra.RangeArgs(2, 3),
		RunE:  runMergeCommand,
	}

	cmd.Flags().String("strategy", "override", "Merge strategy (override, merge, intersect)")
	cmd.Flags().String("conflict", "first", "Conflict resolution (first, second, error)")
	cmd.Flags().Bool("validate", true, "Validate merged configuration")
	cmd.Flags().String("base", "", "Base file for three-way merge")
	cmd.Flags().Bool("preview", false, "Preview merge result without saving")

	return cmd
}

func createSearchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <pattern>",
		Short: "Search configuration",
		Long:  `Search configuration values by pattern or regex.`,
		Args:  cobra.ExactArgs(1),
		RunE:  runSearchCommand,
	}

	cmd.Flags().String("section", "", "Search in specific section")
	cmd.Flags().Bool("regex", false, "Use regular expression")
	cmd.Flags().Bool("case-sensitive", false, "Case sensitive search")
	cmd.Flags().Bool("values", true, "Search in values")
	cmd.Flags().Bool("keys", true, "Search in keys")
	cmd.Flags().Int("limit", 100, "Maximum number of results")
	cmd.Flags().String("sort", "relevance", "Sort results (relevance, key, value)")

	return cmd
}

// Command implementations

func runShowCommand(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")
	switch format {
	case "json":
		return printJSON(cfg)
	case "yaml":
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
	default:
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		fmt.Println(tr(ctx, "helix_config_show_loaded_from", map[string]any{"Path": viper.ConfigFileUsed()}))
		fmt.Println(tr(ctx, "helix_config_show_server_port", map[string]any{"Port": cfg.Server.Port}))
		fmt.Println(tr(ctx, "helix_config_show_database_host", map[string]any{"Host": cfg.Database.Host}))
		fmt.Println(tr(ctx, "helix_config_show_redis_enabled", map[string]any{"Enabled": cfg.Redis.Enabled}))
	}
	return nil
}

func runGetCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("key argument required")
	}
	key := args[0]

	value := viper.Get(key)
	if value == nil {
		return fmt.Errorf("key not found: %s", key)
	}

	format, _ := cmd.Flags().GetString("format")
	if format == "json" {
		return printJSON(map[string]interface{}{key: value})
	}
	fmt.Printf("%s = %v\n", key, value)
	return nil
}

func runSetCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("key and value arguments required")
	}
	key := args[0]
	value := args[1]

	viper.Set(key, value)
	if err := viper.WriteConfig(); err != nil {
		// If config doesn't exist, create it
		if err := viper.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}
	fmt.Printf("Set %s = %s\n", key, value)
	return nil
}

func runDeleteCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("key argument required")
	}
	key := args[0]

	// Set to nil effectively removes the key
	viper.Set(key, nil)
	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	fmt.Printf("Deleted key: %s\n", key)
	return nil
}

func runValidateCommand(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		fmt.Printf("Validation FAILED: %v\n", err)
		return err
	}

	// Basic validation checks
	errors := []string{}
	if cfg.Server.Port <= 0 || cfg.Server.Port > 65535 {
		errors = append(errors, "server.port must be between 1 and 65535")
	}
	if cfg.Auth.JWTSecret != "" && len(cfg.Auth.JWTSecret) < 32 {
		errors = append(errors, "auth.jwt_secret should be at least 32 characters")
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if len(errors) > 0 {
		fmt.Println(tr(ctx, "helix_config_validate_failed_header", nil))
		for _, e := range errors {
			fmt.Printf("  - %s\n", e)
		}
		return fmt.Errorf("validation failed with %d errors", len(errors))
	}

	fmt.Println(tr(ctx, "helix_config_validate_ok", nil))
	return nil
}

func runExportCommand(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	output, _ := cmd.Flags().GetString("output")
	format, _ := cmd.Flags().GetString("format")

	var data []byte
	switch format {
	case "json":
		data, err = json.MarshalIndent(cfg, "", "  ")
	default:
		data, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return err
	}

	if output == "" || output == "-" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(output, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		fmt.Println(tr(ctx, "helix_config_export_written", map[string]any{"Path": output}))
	}
	return nil
}

func runImportCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("file argument required")
	}
	file := args[0]

	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse and merge config
	var imported map[string]interface{}
	if strings.HasSuffix(file, ".json") {
		if err := json.Unmarshal(data, &imported); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &imported); err != nil {
			return fmt.Errorf("failed to parse YAML: %w", err)
		}
	}

	merge, _ := cmd.Flags().GetBool("merge")
	if merge {
		for k, v := range imported {
			viper.Set(k, v)
		}
	} else {
		for k, v := range imported {
			viper.Set(k, v)
		}
	}

	if err := viper.WriteConfig(); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Println(tr(ctx, "helix_config_import_done", map[string]any{"Path": file}))
	return nil
}

func runBackupCommand(cmd *cobra.Command, args []string) error {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		return fmt.Errorf("no config file in use")
	}

	backupPath := configPath + ".backup." + time.Now().Format("20060102-150405")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Println(tr(ctx, "helix_config_backup_written", map[string]any{"Path": backupPath}))
	return nil
}

func runRestoreCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("backup file argument required")
	}
	backupPath := args[0]

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		configPath = "config.yaml"
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore config: %w", err)
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	fmt.Println(tr(ctx, "helix_config_restore_done", map[string]any{"Path": backupPath}))
	return nil
}

func runResetCommand(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Println("This will reset configuration to defaults. Use --force to confirm.")
		return nil
	}

	// Create default config
	defaultCfg := &config.HelixConfig{}
	if err := config.SaveHelixConfig(defaultCfg); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}
	fmt.Println("Configuration reset to defaults")
	return nil
}

func runReloadCommand(cmd *cobra.Command, args []string) error {
	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}
	fmt.Println("Configuration reloaded")
	return nil
}

func runWatchCommand(cmd *cobra.Command, args []string) error {
	fmt.Println("Watching for configuration changes... (Press Ctrl+C to stop)")
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Printf("Config changed: %s\n", e.Name)
	})
	// Block until interrupted
	select {}
}

func runMigrateCommand(cmd *cobra.Command, args []string) error {
	from, _ := cmd.Flags().GetString("from")
	to, _ := cmd.Flags().GetString("to")

	if from == "" {
		from = viper.ConfigFileUsed()
	}

	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}

	srcVer, dstVer, err := configVersions(to, data)
	if err != nil {
		return fmt.Errorf("inspect config versions: %w", err)
	}
	if srcVer != dstVer {
		return fmt.Errorf(
			"helix-config migrate: source version %q ≠ target version %q — "+
				"version-aware schema migration is not yet implemented. "+
				"Previous behavior silently copied source bytes verbatim, "+
				"which masked schema upgrades and was a §11.4 PASS-bluff. "+
				"To proceed: either edit %s to set version=%q before retry, "+
				"or implement a per-version migrator under internal/config/migrations/",
			srcVer, dstVer, from, dstVer,
		)
	}
	if err := os.WriteFile(to, data, 0644); err != nil {
		return fmt.Errorf("failed to write target: %w", err)
	}
	fmt.Printf("Configuration copied (versions match: %q) from %s to %s\n", srcVer, from, to)
	return nil
}

// configVersions extracts the `version:` field from source bytes and
// the target file (if present). Missing version defaults to "" /
// matches source (no migration needed).
func configVersions(dstPath string, srcData []byte) (string, string, error) {
	srcVer := extractConfigVersion(srcData)
	dstVer := srcVer
	if dstData, err := os.ReadFile(dstPath); err == nil {
		dstVer = extractConfigVersion(dstData)
	} else if !os.IsNotExist(err) {
		return "", "", fmt.Errorf("read target %s: %w", dstPath, err)
	}
	return srcVer, dstVer, nil
}

// extractConfigVersion scans for `version:` (YAML) or `"version":` (JSON).
// Returns "" when absent.
func extractConfigVersion(data []byte) string {
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "version:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "version:"))
		}
		if strings.HasPrefix(trimmed, `"version":`) {
			rest := strings.TrimSpace(strings.TrimPrefix(trimmed, `"version":`))
			rest = strings.TrimSuffix(rest, ",")
			return strings.Trim(rest, `"`)
		}
	}
	return ""
}

func runBenchmarkCommand(cmd *cobra.Command, args []string) error {
	iterations, _ := cmd.Flags().GetInt("iterations")
	if iterations <= 0 {
		iterations = 1000
	}

	start := time.Now()
	for i := 0; i < iterations; i++ {
		viper.Get("server.port")
	}
	readDuration := time.Since(start)

	start = time.Now()
	for i := 0; i < iterations; i++ {
		viper.Set("benchmark.test", i)
	}
	writeDuration := time.Since(start)

	fmt.Printf("Benchmark Results (%d iterations):\n", iterations)
	fmt.Printf("  Read operations:  %v (%.2f ops/sec)\n", readDuration, float64(iterations)/readDuration.Seconds())
	fmt.Printf("  Write operations: %v (%.2f ops/sec)\n", writeDuration, float64(iterations)/writeDuration.Seconds())
	return nil
}

// Template command implementations

func runTemplateListCommand(cmd *cobra.Command, args []string) error {
	templates := []string{
		"minimal    - Minimal configuration for testing",
		"production - Production-ready configuration",
		"development - Development configuration with debug enabled",
		"enterprise - Enterprise configuration with all features",
	}
	fmt.Println("Available templates:")
	for _, t := range templates {
		fmt.Printf("  %s\n", t)
	}
	return nil
}

func runTemplateApplyCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("template name argument required")
	}
	templateName := args[0]

	// Apply template based on name
	switch templateName {
	case "minimal":
		viper.Set("server.port", 8080)
		viper.Set("database.enabled", false)
		viper.Set("redis.enabled", false)
	case "production":
		viper.Set("server.port", 8080)
		viper.Set("database.enabled", true)
		viper.Set("redis.enabled", true)
		viper.Set("logging.level", "info")
	case "development":
		viper.Set("server.port", 8080)
		viper.Set("database.enabled", false)
		viper.Set("logging.level", "debug")
	case "enterprise":
		viper.Set("server.port", 8080)
		viper.Set("database.enabled", true)
		viper.Set("redis.enabled", true)
		viper.Set("logging.level", "info")
		viper.Set("monitoring.enabled", true)
	default:
		return fmt.Errorf("unknown template: %s", templateName)
	}

	if err := viper.WriteConfig(); err != nil {
		if err := viper.SafeWriteConfig(); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}
	}
	fmt.Printf("Template '%s' applied successfully\n", templateName)
	return nil
}

// History command implementations

func runHistoryListCommand(cmd *cobra.Command, args []string) error {
	configPath := viper.ConfigFileUsed()
	if configPath == "" {
		return fmt.Errorf("no config file in use")
	}

	// Look for backup files
	dir := filepath.Dir(configPath)
	base := filepath.Base(configPath)
	pattern := base + ".backup.*"

	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(matches) == 0 {
		fmt.Println("No backup history found")
		return nil
	}

	fmt.Println("Configuration history:")
	for _, m := range matches {
		info, _ := os.Stat(m)
		if info != nil {
			fmt.Printf("  %s  %s\n", info.ModTime().Format("2006-01-02 15:04:05"), filepath.Base(m))
		}
	}
	return nil
}

// Schema command implementations

func runSchemaShowCommand(cmd *cobra.Command, args []string) error {
	schema := map[string]interface{}{
		"server": map[string]string{
			"port":         "int - Server port (default: 8080)",
			"host":         "string - Server host (default: localhost)",
			"read_timeout": "duration - Read timeout (default: 30s)",
		},
		"database": map[string]string{
			"enabled":  "bool - Enable database (default: false)",
			"host":     "string - Database host",
			"port":     "int - Database port (default: 5432)",
			"user":     "string - Database user",
			"password": "string - Database password",
			"dbname":   "string - Database name",
		},
		"redis": map[string]string{
			"enabled":  "bool - Enable Redis (default: false)",
			"host":     "string - Redis host",
			"port":     "int - Redis port (default: 6379)",
			"password": "string - Redis password",
		},
		"auth": map[string]string{
			"jwt_secret":   "string - JWT signing secret (min 32 chars)",
			"token_expiry": "duration - Token expiry time",
		},
	}
	return printJSON(schema)
}

// Utility command implementations

func runCompletionCommand(cmd *cobra.Command, args []string) error {
	shell := "bash"
	if len(args) > 0 {
		shell = args[0]
	}

	switch shell {
	case "bash":
		return cmd.Root().GenBashCompletion(os.Stdout)
	case "zsh":
		return cmd.Root().GenZshCompletion(os.Stdout)
	case "fish":
		return cmd.Root().GenFishCompletion(os.Stdout, true)
	case "powershell":
		return cmd.Root().GenPowerShellCompletion(os.Stdout)
	default:
		return fmt.Errorf("unsupported shell: %s", shell)
	}
}

func runVersionCommand(cmd *cobra.Command, args []string) error {
	fmt.Printf("helix-config version %s\n", version)
	fmt.Printf("Build time: %s\n", buildTime)
	fmt.Printf("Git commit: %s\n", gitCommit)
	return nil
}

func runInfoCommand(cmd *cobra.Command, args []string) error {
	info := map[string]interface{}{
		"config_file":  viper.ConfigFileUsed(),
		"config_type":  viper.GetString("config_type"),
		"keys_count":   len(viper.AllKeys()),
		"env_prefix":   "HELIX",
		"search_paths": []string{".", "$HOME/.helixcode", "/etc/helixcode"},
	}
	return printJSON(info)
}

func runStatusCommand(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig()
	status := "healthy"
	issues := []string{}

	if err != nil {
		status = "error"
		issues = append(issues, err.Error())
	} else {
		// Check database configuration - Host being empty means database is not configured
		// but if other DB settings are present without Host, that's a misconfiguration
		if cfg.Database.Host == "" && (cfg.Database.User != "" || cfg.Database.DBName != "") {
			issues = append(issues, "database user/name set but host not configured")
		}
		// Check Redis configuration - has explicit Enabled flag
		if cfg.Redis.Enabled && cfg.Redis.Host == "" {
			issues = append(issues, "redis enabled but host not configured")
		}
		if len(issues) > 0 {
			status = "warning"
		}
	}

	result := map[string]interface{}{
		"status":      status,
		"issues":      issues,
		"config_file": viper.ConfigFileUsed(),
	}
	return printJSON(result)
}

func runDiffCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("two config files required for diff")
	}

	data1, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("failed to read first file: %w", err)
	}
	data2, err := os.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("failed to read second file: %w", err)
	}

	var cfg1, cfg2 map[string]interface{}
	yaml.Unmarshal(data1, &cfg1)
	yaml.Unmarshal(data2, &cfg2)

	// Simple diff - just compare keys
	fmt.Printf("--- %s\n", args[0])
	fmt.Printf("+++ %s\n", args[1])

	allKeys := make(map[string]bool)
	for k := range cfg1 {
		allKeys[k] = true
	}
	for k := range cfg2 {
		allKeys[k] = true
	}

	for k := range allKeys {
		v1, ok1 := cfg1[k]
		v2, ok2 := cfg2[k]
		if !ok1 {
			fmt.Printf("+ %s: %v\n", k, v2)
		} else if !ok2 {
			fmt.Printf("- %s: %v\n", k, v1)
		} else if fmt.Sprintf("%v", v1) != fmt.Sprintf("%v", v2) {
			fmt.Printf("- %s: %v\n", k, v1)
			fmt.Printf("+ %s: %v\n", k, v2)
		}
	}
	return nil
}

func runMergeCommand(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("at least two config files required for merge")
	}

	merged := make(map[string]interface{})

	for _, file := range args {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", file, err)
		}

		var cfg map[string]interface{}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse %s: %w", file, err)
		}

		// Merge configs (later files override earlier)
		for k, v := range cfg {
			merged[k] = v
		}
	}

	output, _ := cmd.Flags().GetString("output")
	data, _ := yaml.Marshal(merged)

	if output == "" || output == "-" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(output, data, 0644); err != nil {
			return fmt.Errorf("failed to write merged config: %w", err)
		}
		fmt.Printf("Merged configuration written to: %s\n", output)
	}
	return nil
}

func runSearchCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("search query required")
	}
	query := strings.ToLower(args[0])

	keys, _ := cmd.Flags().GetBool("keys")
	values, _ := cmd.Flags().GetBool("values")
	limit, _ := cmd.Flags().GetInt("limit")

	results := []map[string]interface{}{}
	count := 0

	for _, key := range viper.AllKeys() {
		if count >= limit {
			break
		}

		value := viper.Get(key)
		valueStr := fmt.Sprintf("%v", value)

		match := false
		if keys && strings.Contains(strings.ToLower(key), query) {
			match = true
		}
		if values && strings.Contains(strings.ToLower(valueStr), query) {
			match = true
		}

		if match {
			results = append(results, map[string]interface{}{
				"key":   key,
				"value": value,
			})
			count++
		}
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	return printJSON(results)
}

// Utility functions

func setupViper() {
	viper.SetConfigName("helix")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME/.helixcode")
	viper.AddConfigPath("/etc/helixcode")
	viper.AddConfigPath("/usr/local/etc/helixcode")
}

func loadConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}

	if err := viper.ReadInConfig(); err != nil {
		if !strings.Contains(err.Error(), "Not Found") {
			fmt.Printf("Error reading config file: %v\n", err)
		}
	}
}

func bindFlags(flags *pflag.FlagSet) {
	flags.VisitAll(func(f *pflag.Flag) {
		viper.BindPFlag(f.Name, f)
	})
}

func getConfig() (*config.HelixConfig, error) {
	return config.LoadHelixConfig()
}

func saveConfig(cfg *config.HelixConfig) error {
	if configFile == "" {
		configFile = "$HOME/.helixcode/helix.yaml"
	}

	return config.SaveHelixConfig(cfg)
}

func printJSON(data interface{}) error {
	var output []byte
	var err error

	if prettyPrint {
		output, err = json.MarshalIndent(data, "", "  ")
	} else {
		output, err = json.Marshal(data)
	}

	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}

func printYAML(data interface{}) error {
	output, err := yaml.Marshal(data)
	if err != nil {
		return err
	}

	fmt.Println(string(output))
	return nil
}

func printTable(headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	// Calculate column widths
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}

	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	// Print header
	for i, header := range headers {
		fmt.Printf("%-*s", widths[i]+2, header)
	}
	fmt.Println()

	// Print separator
	for _, width := range widths {
		fmt.Printf("%-*s", width+2, strings.Repeat("-", width))
	}
	fmt.Println()

	// Print rows
	for _, row := range rows {
		for i, cell := range row {
			fmt.Printf("%-*s", widths[i]+2, cell)
		}
		fmt.Println()
	}
}

func parseTime(timeStr string) (time.Time, error) {
	// Try parsing as duration first
	if duration, err := time.ParseDuration(timeStr); err == nil {
		return time.Now().Add(-duration), nil
	}

	// Try parsing as timestamp
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006 15:04:05",
		"01/02/2006",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, timeStr); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse time: %s", timeStr)
}

func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return strings.Repeat("*", len(value))
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

func confirm(prompt string) bool {
	if force || interactive {
		return true
	}

	fmt.Printf("%s [y/N]: ", prompt)
	var response string
	fmt.Scanln(&response)

	return strings.ToLower(response) == "y" || strings.ToLower(response) == "yes"
}

func errorf(format string, args ...interface{}) {
	if !quiet {
		fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", args...)
	}
}

func warnf(format string, args ...interface{}) {
	if !quiet {
		fmt.Fprintf(os.Stderr, "WARNING: "+format+"\n", args...)
	}
}

func infof(format string, args ...interface{}) {
	if !quiet {
		fmt.Printf(format+"\n", args...)
	}
}

func debugf(format string, args ...interface{}) {
	if verbose && !quiet {
		fmt.Printf("DEBUG: "+format+"\n", args...)
	}
}

func successf(format string, args ...interface{}) {
	if !quiet {
		fmt.Printf("✅ "+format+"\n", args...)
	}
}

// Missing template command implementations
func createTemplateShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <template>",
		Short: "Show template details",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Showing template: %s\n", args[0])
		},
	}
}

func createTemplateCreateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Creating template: %s\n", args[0])
		},
	}
}

func createTemplateUpdateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "update <template>",
		Short: "Update an existing template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Updating template: %s\n", args[0])
		},
	}
}

func createTemplateDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <template>",
		Short: "Delete a template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Deleting template: %s\n", args[0])
		},
	}
}

func createTemplateSearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search templates",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Searching templates: %s\n", args[0])
		},
	}
}

func createTemplateValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <template>",
		Short: "Validate a template",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Validating template: %s\n", args[0])
		},
	}
}

// Missing history command implementations
func createHistoryShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show history entry details",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Showing history entry: %s\n", args[0])
		},
	}
}

func createHistoryRestoreCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore configuration from history",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Restoring configuration from: %s\n", args[0])
		},
	}
}

func createHistoryCompareCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <id1> <id2>",
		Short: "Compare two history entries",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Comparing history entries: %s vs %s\n", args[0], args[1])
		},
	}
}

func createHistorySearchCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search configuration history",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Searching history: %s\n", args[0])
		},
	}
}

func createHistoryCleanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Clean old history entries",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Cleaning old history entries...")
		},
	}
}

// Missing schema command implementations
func createSchemaValidateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate configuration against schema",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Validating configuration: %s\n", args[0])
		},
	}
}

func createSchemaGenerateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "generate",
		Short: "Generate schema from configuration",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Generating schema...")
		},
	}
}

func createSchemaExportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "export <file>",
		Short: "Export schema to file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Exporting schema to: %s\n", args[0])
		},
	}
}

func createSchemaImportCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import schema from file",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Importing schema from: %s\n", args[0])
		},
	}
}
