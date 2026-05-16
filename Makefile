.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all \
        scan-sonarqube scan-snyk scan-all scan-gosec scan-trivy scan-secrets scan-secrets-root \
        verify-llmsverifier-pin-parity verify-governance-cascade bluff-detector verify-foundation

no-silent-skips:
	@bash scripts/no-silent-skips.sh

no-silent-skips-warn:
	@NO_SILENT_SKIPS_WARN_ONLY=1 bash scripts/no-silent-skips.sh

demo-all:
	@bash scripts/demo-all.sh

demo-all-warn:
	@DEMO_ALL_WARN_ONLY=1 DEMO_ALLOW_TODO=1 bash scripts/demo-all.sh

demo-one:
	@DEMO_MODULES="$(MOD)" bash scripts/demo-all.sh

ci-validate-all: no-silent-skips-warn demo-all-warn verify-foundation
	@echo "ci-validate-all: all gates executed"

# ---------------------------------------------------------------------------
# Top-level scan convenience targets (P0-T08.7)
# These cd into helix_code/ and invoke the inner Makefile security targets.
# Usage: make scan-sonarqube   (from root)
# ---------------------------------------------------------------------------

scan-sonarqube: ## Run SonarQube analysis (requires SONAR_TOKEN in helix_code/.env)
	$(MAKE) -C HelixCode security-scan-sonarqube

scan-snyk: ## Run Snyk vulnerability scan (requires SNYK_TOKEN in helix_code/.env)
	$(MAKE) -C HelixCode security-scan-snyk

scan-all: ## Run all HelixCode security scanners
	$(MAKE) -C HelixCode security-scan-all

scan-gosec: ## Run gosec on HelixCode
	$(MAKE) -C HelixCode security-scan-gosec

scan-trivy: ## Run trivy on HelixCode
	$(MAKE) -C HelixCode security-scan-trivy

scan-secrets: ## Run scan-secrets.sh credential scanner
	$(MAKE) -C HelixCode secrets-scan

# ---------------------------------------------------------------------------
# Phase-0 Foundation Gates (P0-15)
# Individual verification gates wired from scripts/
# Usage: make verify-foundation
# ---------------------------------------------------------------------------

verify-llmsverifier-pin-parity: ## Verify LLMsVerifier submodule pins are in parity
	@bash scripts/verify-llmsverifier-pin-parity.sh

verify-governance-cascade: ## Verify governance cascade (CONST-042/043) across owned submodules
	@bash scripts/verify-governance-cascade.sh

bluff-detector: ## Run bluff-detector (Phase 4 deliverable; stub-tolerant)
	@if [ -x scripts/bluff-detector.sh ]; then \
	  bash scripts/bluff-detector.sh; \
	else \
	  echo "bluff-detector.sh not yet implemented (Phase 4 deliverable); skipping"; \
	fi

# scan-secrets-root: run the root-level credential scanner (scripts/scan-secrets.sh).
# Used by verify-foundation. Distinct from scan-secrets which runs the inner
# helix_code/scripts/scan-secrets.sh targeting only the Go app subdirectory.
scan-secrets-root: ## Run root scripts/scan-secrets.sh (whole-repo credential scan)
	@bash scripts/scan-secrets.sh

# Composite Phase-0 foundation gate.
# Depends on: no-silent-skips-warn, scan-secrets-root, verify-llmsverifier-pin-parity,
#             verify-governance-cascade, bluff-detector.
#
# NOTE: As of P0-15, this gate exits 1 because verify-llmsverifier-pin-parity
# reports the known LLMsVerifier dual-pin divergence (documented parking-lot item
# in docs/improvements/PROGRESS.md). Resolution is a P0-16 close-out dependency.
verify-foundation: no-silent-skips-warn scan-secrets-root verify-llmsverifier-pin-parity verify-governance-cascade bluff-detector
	@echo "verify-foundation: all gates passed"
