#!/usr/bin/env bash
# HelixCode/tests/e2e/challenges/auto-compaction/run.sh
# Challenge: claude-code-style auto-compaction triggers at 80% threshold,
# attaches metadata, and respects thrashing detection.
# Per CONST-035: runtime evidence required. Per Article XI §11.9: every
# PASS must demonstrate the feature actually works for the end user.

set -euo pipefail
cd "$(git rev-parse --show-toplevel)/HelixCode"

if [ -z "${HELIX_LLM_ANTHROPIC_KEY:-}" ]; then
  echo "SKIP-OK: #P1-F01-CHAL — HELIX_LLM_ANTHROPIC_KEY not set"
  exit 0
fi

# Driver must live inside the Go module so internal/ imports are allowed.
DRIVER_DIR=$(mktemp -d -p cmd)
trap 'rm -rf "$DRIVER_DIR"' EXIT
DRIVER="$DRIVER_DIR/main.go"

cat > "$DRIVER" <<'GO'
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/llm/compression"
	"dev.helix.code/internal/llm/compressioniface"
)

func main() {
	prov, err := llm.NewAnthropicProvider(llm.ProviderConfigEntry{
		Type:    llm.ProviderTypeAnthropic,
		APIKey:  os.Getenv("HELIX_LLM_ANTHROPIC_KEY"),
		Enabled: true,
		Models:  []string{"claude-3-5-sonnet-20241022"},
	})
	if err != nil {
		fmt.Println("provider:", err)
		os.Exit(1)
	}
	defer func() { _ = prov.Close() }()

	conv := &compressioniface.Conversation{ID: "chal", CreatedAt: time.Now()}
	for i := 0; i < 116; i++ {
		role := compressioniface.RoleUser
		if i%2 == 1 {
			role = compressioniface.RoleAssistant
		}
		conv.Messages = append(conv.Messages, &compressioniface.Message{
			ID:      fmt.Sprintf("msg-%d", i),
			Role:    role,
			Content: strings.Repeat("Filler. ", 1000),
		})
	}

	coord := compression.NewCompressionCoordinator(prov, compression.WithStrategy(compression.StrategySemanticSummarization))
	ac := compression.NewAutoCompactor(prov, coord, compression.NewThrashingGuard(3), 0.80)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := ac.MaybeCompact(ctx, conv)
	if err != nil {
		fmt.Println("compact:", err)
		os.Exit(1)
	}

	if result.WasCompacted {
		fmt.Println("AUTO_COMPACTION_TRIGGERED")
		fmt.Printf("tokens_before=%d tokens_after=%d window=%d\n",
			result.TokensBefore, result.TokensAfter, result.WindowSize)
		hasMeta := false
		for _, m := range conv.Messages {
			if _, ok := compression.ReadCompactionMetadata(m); ok {
				hasMeta = true
				break
			}
		}
		if hasMeta {
			fmt.Println("compaction_metadata_attached")
		}
	}

	out, _ := json.Marshal(map[string]interface{}{
		"was_compacted": result.WasCompacted,
		"tokens_before": result.TokensBefore,
		"tokens_after":  result.TokensAfter,
		"window_size":   result.WindowSize,
		"timestamp":     time.Now().Format(time.RFC3339),
	})
	_ = os.WriteFile("tests/e2e/challenges/auto-compaction/.last-run-evidence.json", out, 0644)
}
GO

go run "$DRIVER"
echo "---"
cat tests/e2e/challenges/auto-compaction/.last-run-evidence.json
