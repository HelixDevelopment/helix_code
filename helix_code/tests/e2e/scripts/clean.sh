#!/bin/bash

# E2E Testing Framework - Clean Script
# Cleans up build artifacts and temporary files

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_ROOT="$(dirname "$SCRIPT_DIR")"

# §11.4.98 self-driving: clean.sh must never block on interactive input in
# automated / CI / pipeline runs. --force / -f (or CLEAN_FORCE=1) removes
# test-results non-interactively; otherwise an interactive TTY is prompted and a
# non-interactive run KEEPS results (safe default) instead of hanging on `read`.
FORCE="${CLEAN_FORCE:-0}"
for arg in "$@"; do
    case "$arg" in
        -f|--force) FORCE=1 ;;
        -h|--help)
            echo "Usage: clean.sh [--force|-f]"
            echo "  --force / -f   remove test-results non-interactively (CLEAN_FORCE=1 env equivalent)"
            exit 0 ;;
    esac
done

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "========================================="
echo "Cleaning E2E Testing Framework"
echo "========================================="
echo ""

# Stop services first
if [ -d "$E2E_ROOT/.pids" ]; then
    echo "Stopping services..."
    "$SCRIPT_DIR/stop-services.sh"
    echo ""
fi

# Clean orchestrator
if [ -d "$E2E_ROOT/orchestrator/bin" ]; then
    echo "Cleaning orchestrator..."
    rm -rf "$E2E_ROOT/orchestrator/bin"
    echo -e "${GREEN}✓ Orchestrator cleaned${NC}"
fi

# Clean mock LLM provider
if [ -d "$E2E_ROOT/mocks/llm-provider/bin" ]; then
    echo "Cleaning Mock LLM Provider..."
    rm -rf "$E2E_ROOT/mocks/llm-provider/bin"
    echo -e "${GREEN}✓ Mock LLM Provider cleaned${NC}"
fi

# Clean mock Slack service
if [ -d "$E2E_ROOT/mocks/slack/bin" ]; then
    echo "Cleaning Mock Slack Service..."
    rm -rf "$E2E_ROOT/mocks/slack/bin"
    echo -e "${GREEN}✓ Mock Slack Service cleaned${NC}"
fi

# Clean PIDs directory
if [ -d "$E2E_ROOT/.pids" ]; then
    echo "Cleaning PID files..."
    rm -rf "$E2E_ROOT/.pids"
    echo -e "${GREEN}✓ PID files cleaned${NC}"
fi

# Clean test results (destructive — gated; self-driving per §11.4.98)
if [ -d "$E2E_ROOT/test-results" ]; then
    remove=0
    if [ "$FORCE" = "1" ]; then
        remove=1
    elif [ -t 0 ]; then
        # Interactive TTY: ask the human.
        read -p "Remove test results? (y/N) " -n 1 -r
        echo
        [[ $REPLY =~ ^[Yy]$ ]] && remove=1
    else
        # Non-interactive (CI/pipeline): keep results; pass --force to remove.
        echo -e "${YELLOW}✓ Test results kept (non-interactive; pass --force to remove)${NC}"
    fi
    if [ "$remove" = "1" ]; then
        rm -rf "$E2E_ROOT/test-results"
        echo -e "${GREEN}✓ Test results cleaned${NC}"
    elif [ "$FORCE" != "1" ] && [ -t 0 ]; then
        echo -e "${YELLOW}✓ Test results kept${NC}"
    fi
fi

echo ""
echo -e "${GREEN}Cleanup complete!${NC}"
echo ""
echo "To rebuild everything, run: ./scripts/setup.sh"
echo ""
