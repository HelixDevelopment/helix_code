package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dev.helix.code/cmd/server/i18n"
	"dev.helix.code/internal/config"
	"dev.helix.code/internal/database"
	"dev.helix.code/internal/redis"
	"dev.helix.code/internal/server"
)

var (
	version   = "1.0.0"
	buildTime = "unknown"
	gitCommit = "unknown"
)

// translator resolves CONST-046 message IDs for every user-facing
// string emitted by this process. Defaults to i18n.NoopTranslator{}
// (loud message-ID echo) so unit tests + ad-hoc invocations remain
// obvious. helix_code wires a real *i18nadapter.Translator at boot
// via SetTranslator (round-134 §11.4 anti-bluff sweep, 2026-05-18).
//
// A package-level variable is the chosen DI seam because main() flows
// through fmt.Printf / log.Printf / log.Fatalf directly — global
// injection matches log's own use of package-level state and keeps
// the migration minimally invasive.
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

	fmt.Println(tr(ctx, "server_startup_banner_version", map[string]any{"Version": version}))
	fmt.Println(tr(ctx, "server_startup_banner_build", map[string]any{"BuildTime": buildTime}))
	fmt.Println(tr(ctx, "server_startup_banner_commit", map[string]any{"GitCommit": gitCommit}))
	fmt.Println()

	// Load configuration.
	// Speed programme P2-T07: config.Get() loads + caches the config once
	// per process so any later config consumer reuses the same *Config
	// instead of re-reading YAML / re-churning viper.
	cfg, err := config.Get()
	if err != nil {
		log.Fatal(tr(ctx, "server_fatal_load_config", map[string]any{"Err": err}))
	}

	// Initialize database (optional for testing)
	var db *database.Database
	if cfg.Database.Host != "" {
		db, err = database.New(cfg.Database)
		if err != nil {
			log.Print(tr(ctx, "server_warn_db_init_skipped", map[string]any{"Err": err}))
		} else {
			defer db.Close()

			// Initialize database schema
			if err := db.InitializeSchema(); err != nil {
				log.Printf("⚠️  Failed to initialize database schema (continuing without): %v", err)
				db = nil
			}
		}
	}

	// Initialize Redis
	rds, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Fatal(tr(ctx, "server_fatal_redis_init", map[string]any{"Err": err}))
	}
	defer rds.Close()

	// Create HTTP server
	srv := server.New(cfg, db, rds)

	// Start server in a goroutine
	go func() {
		log.Print(tr(ctx, "server_runtime_http_start", map[string]any{"Address": cfg.Server.Address}))
		if err := srv.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatal(tr(ctx, "server_fatal_http_start", map[string]any{"Err": err}))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print(tr(ctx, "server_lifecycle_shutting_down", nil))

	// Give outstanding requests a deadline for completion
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	log.Print(tr(ctx, "server_lifecycle_exited_properly", nil))
}
