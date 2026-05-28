#!/bin/bash
###############################################################################
# HelixCode Submodule Integration Script
# ALL integrations via SSH (NOT HTTPS)
# Run from HelixCode repository root
###############################################################################

set -euo pipefail

REPO_ROOT="$(pwd)"
SCRIPT_NAME="$(basename "$0")"
LOG_FILE="${REPO_ROOT}/submodule-integration-$(date +%Y%m%d-%H%M%S).log"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1" | tee -a "$LOG_FILE"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1" | tee -a "$LOG_FILE"
}

warn() {
    echo -e "${YELLOW}[WARNING]${NC} $1" | tee -a "$LOG_FILE"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" | tee -a "$LOG_FILE"
}

###############################################################################
# PRE-FLIGHT CHECKS
###############################################################################

log "=== HelixCode Submodule Integration via SSH ==="
log "Log file: $LOG_FILE"
log ""

# Check we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    error "Not in a git repository. Please run from HelixCode root."
    exit 1
fi

# Check SSH access to GitHub
log "Checking SSH access to GitHub..."
if ! ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
    warn "SSH access to GitHub not confirmed. Ensure ~/.ssh/id_rsa or ~/.ssh/id_ed25519 is configured."
    warn "To configure: ssh-keygen -t ed25519 -C 'your-email'; then add to GitHub Settings -> SSH Keys"
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

# Check git version
GIT_VERSION=$(git --version | awk '{print $3}')
log "Git version: $GIT_VERSION"

# Check current submodules
log "Current submodule status:"
git submodule status || true
log ""

###############################################################################
# PHASE 0A: ADD MISSING SUBMODULES (4 total)
###############################################################################

log "=== PHASE 0A: Adding 4 Missing Submodules via SSH ==="

# P0-T2: Add HelixAgent
# NOTE: HelixCode is actually a child of HelixAgent, so we use a read-only reference
log "Adding HelixAgent submodule..."
if [ ! -d "dependencies/HelixDevelopment/HelixAgent" ]; then
    git submodule add --force \
        git@github.com:HelixDevelopment/HelixAgent.git \
        dependencies/HelixDevelopment/HelixAgent
    success "HelixAgent submodule added"
else
    warn "HelixAgent directory already exists"
fi

# P0-T2: Add HelixLLM
log "Adding HelixLLM submodule..."
if [ ! -d "dependencies/HelixDevelopment/helix_llm" ]; then
    git submodule add --force \
        git@github.com:HelixDevelopment/HelixLLM.git \
        dependencies/HelixDevelopment/helix_llm
    success "HelixLLM submodule added"
else
    warn "HelixLLM directory already exists"
fi

# P0-T2: Add HelixMemory
log "Adding HelixMemory submodule..."
if [ ! -d "dependencies/HelixDevelopment/helix_memory" ]; then
    git submodule add --force \
        git@github.com:HelixDevelopment/HelixMemory.git \
        dependencies/HelixDevelopment/helix_memory
    success "HelixMemory submodule added"
else
    warn "HelixMemory directory already exists"
fi

# P0-T2: Add HelixSpecifier
log "Adding HelixSpecifier submodule..."
if [ ! -d "dependencies/HelixDevelopment/helix_specifier" ]; then
    git submodule add --force \
        git@github.com:HelixDevelopment/HelixSpecifier.git \
        dependencies/HelixDevelopment/helix_specifier
    success "HelixSpecifier submodule added"
else
    warn "HelixSpecifier directory already exists"
fi

###############################################################################
# PHASE 0B: INITIALIZE ALL SUBMODULES RECURSIVELY
###############################################################################

log ""
log "=== PHASE 0B: Recursive Submodule Initialization ==="

# Initialize all submodules (existing + new)
log "Running: git submodule update --init --recursive"
git submodule update --init --recursive || {
    error "Submodule initialization failed. Common causes:"
    error "  1. SSH key not added to ssh-agent: eval $(ssh-agent) && ssh-add ~/.ssh/id_ed25519"
    error "  2. GitHub access denied: Check SSH key is added to GitHub account"
    error "  3. Network issues: Retry with GIT_TRACE=1 git submodule update --init"
    exit 1
}

success "All submodules initialized recursively"

###############################################################################
# PHASE 0C: VERIFY SSH URLS (NO HTTPS ALLOWED)
###############################################################################

log ""
log "=== PHASE 0C: Verifying All Submodules Use SSH ==="

# Check .gitmodules for any HTTPS URLs
HTTPS_COUNT=$(grep -c "https://github.com" .gitmodules || true)
if [ "$HTTPS_COUNT" -gt 0 ]; then
    error "Found $HTTPS_COUNT HTTPS URLs in .gitmodules! ALL must be SSH."
    grep "https://github.com" .gitmodules || true
    error "Fixing by converting to SSH..."
    sed -i 's|https://github.com/|git@github.com:|g' .gitmodules
    sed -i 's|https://github.com:|git@github.com:|g' .gitmodules
    git add .gitmodules
    success "Converted all URLs to SSH format"
else
    success "All submodule URLs are SSH. No HTTPS found."
fi

# Verify each submodule's origin is SSH
log "Verifying individual submodule origins..."
git submodule foreach '
    ORIGIN=$(git remote get-url origin 2>/dev/null || echo "none")
    if echo "$ORIGIN" | grep -q "https://"; then
        echo "WARNING: $name has HTTPS origin: $ORIGIN"
        NEW_URL=$(echo "$ORIGIN" | sed "s|https://github.com/|git@github.com:|g")
        git remote set-url origin "$NEW_URL"
        echo "FIXED: $name -> $NEW_URL"
    else
        echo "OK: $name -> $ORIGIN"
    fi
'

###############################################################################
# PHASE 0D: BUILD VERIFICATION
###############################################################################

log ""
log "=== PHASE 0D: Build Verification ==="

# Check Go version
GO_VERSION=$(go version | awk '{print $3}')
log "Go version: $GO_VERSION"

# Create go.work if it doesn't exist
if [ ! -f "go.work" ]; then
    log "Creating go.work workspace file..."
    cat > go.work << 'EOF'
go 1.26

use (
	.
	./HelixCode
	./dependencies/HelixDevelopment/llms_verifier
	./dependencies/HelixDevelopment/HelixAgent
	./dependencies/HelixDevelopment/helix_llm
	./dependencies/HelixDevelopment/helix_memory
	./dependencies/HelixDevelopment/helix_specifier
	./HelixQA
	./Challenges
	./Containers
)
EOF
    success "go.work created"
else
    log "go.work already exists"
fi

# Sync workspace
log "Running go work sync..."
go work sync || warn "go work sync had issues (may need manual resolution)"

# Build root module
log "Building root module..."
go build ./... || {
    error "Root module build failed"
    exit 1
}
success "Root module builds successfully"

###############################################################################
# PHASE 0E: SUBMODULE STATUS REPORT
###############################################################################

log ""
log "=== PHASE 0E: Final Submodule Status ==="

TOTAL=$(git submodule status | wc -l)
INITIALIZED=$(git submodule status | awk '{print $1}' | grep -v '^-' | wc -l)
UNINITIALIZED=$(git submodule status | awk '{print $1}' | grep '^-' | wc -l)

log "Total submodules: $TOTAL"
log "Initialized: $INITIALIZED"
log "Uninitialized: $UNINITIALIZED"

if [ "$UNINITIALIZED" -eq 0 ]; then
    success "ALL $TOTAL submodules are initialized!"
else
    warn "$UNINITIALIZED submodules are not initialized"
    git submodule status | grep '^-' || true
fi

###############################################################################
# COMPLETION
###############################################################################

log ""
log "=== Integration Complete ==="
success "Submodule integration log: $LOG_FILE"
log ""
log "Next steps:"
log "  1. Review .gitmodules changes: git diff --cached .gitmodules"
log "  2. Commit changes: git add .gitmodules go.work && git commit -m 'Add missing submodules via SSH'"
log "  3. Push: git push origin main"
log "  4. Proceed to Phase 1: Core Infrastructure (see integration plan)"
log ""

exit 0
