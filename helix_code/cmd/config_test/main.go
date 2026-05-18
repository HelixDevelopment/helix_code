package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"dev.helix.code/cmd/config_test/i18n"
	"dev.helix.code/internal/config"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this CLI. Defaults to i18n.NoopTranslator{} (loud
// message-ID echo) so unit tests + ad-hoc invocations remain obvious.
// helix_code wires a real *i18nadapter.Translator at boot via
// SetTranslator (round-141 §11.4 anti-bluff sweep, 2026-05-18).
//
// A package-level variable is the chosen DI seam to keep the
// migration minimally invasive — main()'s linear call graph does
// not warrant a constructor-injected struct.
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

func main() {
	ctx := context.Background()
	fmt.Println(tr(ctx, "config_test_header_title", nil))
	fmt.Println(tr(ctx, "config_test_header_divider", nil))

	// Load initial configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(tr(ctx, "config_test_load_failed", map[string]any{"Err": err}))
	}

	fmt.Println(tr(ctx, "config_test_initial_loaded", nil))
	printConfigInfo(ctx, cfg)

	// Configuration watcher not implemented in current API
	configPath := config.GetConfigPath()

	fmt.Println(tr(ctx, "config_test_config_path", map[string]any{"Path": configPath}))
	fmt.Println(tr(ctx, "config_test_press_ctrl_c", nil))

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println()
	fmt.Println(tr(ctx, "config_test_shutting_down", nil))
}

func printConfigInfo(ctx context.Context, cfg *config.Config) {
	// ConfigInfo is empty struct, so we'll print directly from cfg
	fmt.Println(tr(ctx, "config_test_info_server", map[string]any{
		"Address": cfg.Server.Address,
		"Port":    cfg.Server.Port,
	}))
	fmt.Println(tr(ctx, "config_test_info_database", map[string]any{
		"Host":   cfg.Database.Host,
		"Port":   cfg.Database.Port,
		"DBName": cfg.Database.DBName,
	}))
	fmt.Println(tr(ctx, "config_test_info_redis", map[string]any{
		"Host":    cfg.Redis.Host,
		"Port":    cfg.Redis.Port,
		"Enabled": cfg.Redis.Enabled,
	}))
	fmt.Println(tr(ctx, "config_test_info_auth", map[string]any{
		"Length": len(cfg.Auth.JWTSecret),
	}))
	fmt.Println(tr(ctx, "config_test_info_llm", map[string]any{
		"Provider":    cfg.LLM.DefaultProvider,
		"MaxTokens":   cfg.LLM.MaxTokens,
		"Temperature": cfg.LLM.Temperature,
	}))
}
