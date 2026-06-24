# docs/qa/DeepSeek/ — §11.4.83 end-user evidence

**Run-id:** `DeepSeek` (literal substring of the shipping commit subject
`fix(server): resolve default LLM model from the verifier catalog (DeepSeek 502)`,
commit `a52a523a`; greppable via `git log --grep DeepSeek` / `ls docs/qa/DeepSeek`).

**Feature shipped:** server resolves the DEFAULT LLM model from the
LLMsVerifier-sourced provider catalog (CONST-036/037 — no hardcoded list)
when a `/api/v1/llm/generate` or `/api/v1/llm/stream` request omits the
`model` field. Previously an omitted model was passed upstream EMPTY, which
the DeepSeek provider rejected (HTTP 400 → handler 502).

**Source:** `helix_code/internal/server/llm_generate.go` →
`resolveDefaultModel(provider, requested)`, wired into both `generateLLM`
and `streamLLM`.

---

## End-user path exercised (same interface as production)

The production interface is the HTTP endpoint a client (web/CLI/desktop)
calls:

```
POST /api/v1/llm/generate
Content-Type: application/json
{ "prompt": "What is 2+2?", "provider": "deepseek" }     # NOTE: no "model"
```

Before the fix the server forwarded `model: ""` to DeepSeek, whose response
body was the real upstream rejection (captured in the commit message that
shipped this fix):

```
400 supported API model names are deepseek-v4-pro or deepseek-v4-flash,
    but you passed .
```

→ surfaced to the end user as **HTTP 502**. After the fix the server
resolves the first verifier-available catalog model (`deepseek-v4-flash`)
and the request returns a real answer.

---

## Bidirectional captured evidence (RED → GREEN, §11.4.115)

The behaviour is pinned by `internal/server/llm_default_model_regression_test.go`
— a §11.4.115 polarity-switch regression test exercising the same handler
the HTTP endpoint uses (`generateLLM` / `streamLLM`).

### RED — defect reproduced on the pre-fix resolution path (`RED_MODE=1`)

Command:

```
RED_MODE=1 go test -count=1 -v -run 'TestDefaultModelResolution_PolaritySwitch$' ./internal/server/
```

Captured output (`red_mode_reproduction.txt`):

```
    llm_default_model_regression_test.go:125: RED reproduced: pre-fix resolution yields empty model "" (would 400 upstream)
--- PASS: TestDefaultModelResolution_PolaritySwitch (0.00s)
ok  	dev.helix.code/internal/server	0.427s
```

The RED-mode substitution restores the pre-fix resolution (model stays
empty when the request omits it) and the test asserts the empty-model
defect is genuinely present — the same empty model that produced the
upstream 400 → 502.

### GREEN — defect absent on the shipped artifact (`RED_MODE=0`, default)

Command:

```
go test -count=1 -v -run 'TestDefaultModelResolution|TestResolveDefaultModel' ./internal/server/
```

Captured output (`green_regression_passing.txt`):

```
--- PASS: TestDefaultModelResolution_PolaritySwitch (0.00s)
--- PASS: TestDefaultModelResolution_Stream_PolaritySwitch (0.00s)
--- PASS: TestResolveDefaultModel_ExtendAcrossProviders (0.00s)
    --- PASS: .../deepseek_omitted (0.00s)
    --- PASS: .../openai_omitted (0.00s)
    --- PASS: .../mistral_omitted (0.00s)
    --- PASS: .../groq_omitted (0.00s)
    --- PASS: .../deepseek_explicit (0.00s)
    --- PASS: .../openai_explicit (0.00s)
    --- PASS: .../whitespace_treated_as_omitted (0.00s)
    --- PASS: .../blank_name_falls_back_to_id (0.00s)
    --- PASS: .../skips_blank_leading_entry (0.00s)
    --- PASS: .../empty_catalog_left_empty (0.00s)
    --- PASS: .../empty_catalog_explicit_kept (0.00s)
ok  	dev.helix.code/internal/server	0.434s
```

The §11.4.146 STEP-3 extend cases prove the resolution across
deepseek/openai/mistral/groq + boundary cases (explicit model honoured,
whitespace-as-omitted, blank-name→id fallback, empty-catalog honest
boundary left empty so the provider's own honest-error path takes over —
the server never invents a model).

---

## Materials in this directory

| File | What it is |
|---|---|
| `transcript.md` | This bidirectional record (request → upstream rejection → fix → resolved). |
| `red_mode_reproduction.txt` | Live `RED_MODE=1` output proving the defect is real pre-fix. |
| `green_regression_passing.txt` | Live `RED_MODE=0` output proving the defect is absent on the shipped artifact + the extend matrix. |

## Honest boundary (§11.4.6)

This evidence proves the empty-model→502 defect is reproduced pre-fix and
absent post-fix at the handler layer the HTTP endpoint uses, plus the
cross-provider extend matrix. A full live HTTP round-trip against a real
DeepSeek key (network + credential) is the LLMsVerifier-catalog concern;
the captured upstream 400 string in the shipping commit is the real
provider rejection that motivated the fix.

## Sources verified

- 2026-06-24: `helix_code/internal/server/llm_generate.go` (shipped fix) +
  `helix_code/internal/server/llm_default_model_regression_test.go`
  (regression test), commit `a52a523a`.
- Constitution submodule §11.4.83 (docs/qa/ evidence), §11.4.115 (RED-baseline
  polarity switch), §11.4.146 (reproduce-first + extend), CONST-036/037
  (LLMsVerifier single source of truth — no hardcoded model list).
