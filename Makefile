.PHONY: no-silent-skips no-silent-skips-warn demo-all demo-all-warn demo-one ci-validate-all \
        scan-sonarqube scan-snyk scan-all scan-gosec scan-trivy scan-secrets

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

ci-validate-all: no-silent-skips-warn demo-all-warn
	@echo "ci-validate-all: all gates executed"

# ---------------------------------------------------------------------------
# Top-level scan convenience targets (P0-T08.7)
# These cd into HelixCode/ and invoke the inner Makefile security targets.
# Usage: make scan-sonarqube   (from root)
# ---------------------------------------------------------------------------

scan-sonarqube: ## Run SonarQube analysis (requires SONAR_TOKEN in HelixCode/.env)
	$(MAKE) -C HelixCode security-scan-sonarqube

scan-snyk: ## Run Snyk vulnerability scan (requires SNYK_TOKEN in HelixCode/.env)
	$(MAKE) -C HelixCode security-scan-snyk

scan-all: ## Run all HelixCode security scanners
	$(MAKE) -C HelixCode security-scan-all

scan-gosec: ## Run gosec on HelixCode
	$(MAKE) -C HelixCode security-scan-gosec

scan-trivy: ## Run trivy on HelixCode
	$(MAKE) -C HelixCode security-scan-trivy

scan-secrets: ## Run scan-secrets.sh credential scanner
	$(MAKE) -C HelixCode secrets-scan
