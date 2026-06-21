#!/usr/bin/env bash
# ============================================================================
# codegraph_setup.sh — Initialize and index CodeGraph for HelixCode
# ============================================================================
# Constitution §11.4.78, §11.4.79, §11.4.80
#
# Usage: bash scripts/codegraph_setup.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CODEGRAPH_DIR="$PROJECT_ROOT/.codegraph"
CONFIG_FILE="$CODEGRAPH_DIR/config.json"

echo "=== CodeGraph Setup ==="
echo "Project root: $PROJECT_ROOT"

# 1. Check codegraph is installed
if ! command -v codegraph &>/dev/null; then
    echo "ERROR: codegraph not found. Install with: npm install -g @colbymchenry/codegraph"
    exit 1
fi

echo "codegraph version: $(codegraph --version 2>/dev/null || echo 'unknown')"

# 2. Initialize if needed
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Initializing codegraph..."
    cd "$PROJECT_ROOT" && codegraph init
fi

# 3. Verify config exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "ERROR: codegraph config not created"
    exit 1
fi

echo "Config: $CONFIG_FILE"

# 4. Index the project
echo "Indexing project (this may take a while)..."
cd "$PROJECT_ROOT" && codegraph index

# 5. Show status
echo ""
echo "=== CodeGraph Status ==="
cd "$PROJECT_ROOT" && codegraph status

echo ""
echo "=== Setup Complete ==="
echo "Database: $CODEGRAPH_DIR/codegraph.db"
echo "MCP server: codegraph serve --mcp --path $PROJECT_ROOT"
