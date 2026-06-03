# Stream FLAT-BUILD-SUBS — build-verify helix_agent + helix_qa (Bash ok; NO Edit — propose patches as text)

Repo: /Users/milosvasic/Projects/HelixCode. Two relocated Go submodules now at `submodules/helix_agent/` and `submodules/helix_qa/`.
- helix_agent/go.mod: 37 replaces rewritten `../dependencies/{org}/X` → sibling `../X`; plus `../dependencies/HelixDevelopment/llms_verifier/llm-verifier` → `../llms_verifier/llm-verifier`.
- helix_qa/go.mod: dual capital/lowercase replace blocks normalized to ONE lowercase block; `LLMsVerifier` → `llms_verifier`; sibling `../X` form.

Pasted terminal output is REQUIRED evidence — no verdict without actual output.

A) helix_agent:
1. `cat submodules/helix_agent/go.mod` — paste replace section; flag any line still containing `dependencies/` or a capitalized leaf.
2. `cd /Users/milosvasic/Projects/HelixCode/submodules/helix_agent && go build ./... 2>&1 | tail -120 && echo "---VET---" && go vet ./... 2>&1 | tail -60`. PASS/FAIL + output.
3. Files `.trivy.yaml` and `challenges/scripts/challenge_framework.sh` had stale refs — quote lines; state decoupled-own-layout (leave per CONST-051) vs genuinely broken. Do NOT propose parent-layout rewrites for decoupled files.

B) helix_qa:
1. `cat submodules/helix_qa/go.mod` — paste replace section; confirm single lowercase block (no duplicate capital block), no `LLMsVerifier`, sibling `../X`. Flag deviations.
2. `cd /Users/milosvasic/Projects/HelixCode/submodules/helix_qa && go build ./... 2>&1 | tail -120 && echo "---VET---" && go vet ./... 2>&1 | tail -60`. If go.work errors on workspace, note it + the modified go.work.sum state. PASS/FAIL + output.
3. File `helix-deps.yaml`: quote relevant lines; decoupled-vs-broken verdict.

Per submodule: verdict + minimal go.mod patch proposal if needed. Note: a fix here = a commit INSIDE that submodule repo (I apply serially).

Output: compact markdown.
