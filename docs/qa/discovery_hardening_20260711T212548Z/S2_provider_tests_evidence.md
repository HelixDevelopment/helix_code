# F-D1-02 — huggingface / together provider unit tests — evidence

Date: 2026-07-12
Scope touched: ONLY new `*_test.go` files (no production source edited).

## Files created

- `helix_code/internal/llm/providers/huggingface/client_test.go`
- `helix_code/internal/llm/providers/together/client_test.go`

Both are white-box (`package huggingface` / `package together`) so tests exercise
the REAL unexported request/response types (`hfRequest`/`hfResponse`,
`togetherRequest`/`togetherMessage`/`togetherChoice`/`togetherResponse`) and the
REAL `Client.Generate` method — no reimplementation of client logic. Each test
spins a real `httptest.Server`, points the real client at it by overwriting the
unexported `baseURL` field (same package, no reflection), and asserts BOTH the
real request the client built (method, path/headers/body) and the real response
parsing (content extraction, error wrapping).

## Command + full output

```
cd helix_code && go test -tags=nogui ./internal/llm/providers/huggingface/ ./internal/llm/providers/together/ -v -count=1 -coverprofile=/tmp/s2_cov_final.out
```

```
=== RUN   TestClient_Generate_RequestAndResponse
=== RUN   TestClient_Generate_RequestAndResponse/default_model_used_when_request_has_no_model,_prompt_is_last_message
=== RUN   TestClient_Generate_RequestAndResponse/custom_model_honored,_last_of_multiple_messages_used_as_prompt
=== RUN   TestClient_Generate_RequestAndResponse/no_messages_produces_empty_prompt_but_still_calls_default_model
--- PASS: TestClient_Generate_RequestAndResponse (0.00s)
    --- PASS: TestClient_Generate_RequestAndResponse/default_model_used_when_request_has_no_model,_prompt_is_last_message (0.00s)
    --- PASS: TestClient_Generate_RequestAndResponse/custom_model_honored,_last_of_multiple_messages_used_as_prompt (0.00s)
    --- PASS: TestClient_Generate_RequestAndResponse/no_messages_produces_empty_prompt_but_still_calls_default_model (0.00s)
=== RUN   TestClient_Generate_NonOKStatus
--- PASS: TestClient_Generate_NonOKStatus (0.00s)
=== RUN   TestClient_Generate_EmptyResultsArray
--- PASS: TestClient_Generate_EmptyResultsArray (0.00s)
=== RUN   TestClient_Generate_MalformedJSON
--- PASS: TestClient_Generate_MalformedJSON (0.00s)
=== RUN   TestNewClient
--- PASS: TestNewClient (0.00s)
PASS
coverage: 92.6% of statements
ok  	dev.helix.code/internal/llm/providers/huggingface	0.011s	coverage: 92.6% of statements
=== RUN   TestClient_Generate_RequestAndResponse
=== RUN   TestClient_Generate_RequestAndResponse/default_model_and_defaults_applied_when_unset
=== RUN   TestClient_Generate_RequestAndResponse/custom_model,_explicit_max_tokens/temperature_preserved,_all_messages_converted_in_order
--- PASS: TestClient_Generate_RequestAndResponse (0.00s)
    --- PASS: TestClient_Generate_RequestAndResponse/default_model_and_defaults_applied_when_unset (0.00s)
    --- PASS: TestClient_Generate_RequestAndResponse/custom_model,_explicit_max_tokens/temperature_preserved,_all_messages_converted_in_order (0.00s)
=== RUN   TestClient_Generate_NonOKStatus
--- PASS: TestClient_Generate_NonOKStatus (0.00s)
=== RUN   TestClient_Generate_EmptyChoices
--- PASS: TestClient_Generate_EmptyChoices (0.00s)
=== RUN   TestClient_Generate_MalformedJSON
--- PASS: TestClient_Generate_MalformedJSON (0.00s)
=== RUN   TestNewClient
--- PASS: TestNewClient (0.00s)
PASS
coverage: 96.4% of statements
ok  	dev.helix.code/internal/llm/providers/together	0.010s	coverage: 96.4% of statements
```

## Coverage by function

```
go tool cover -func=/tmp/s2_cov_final.out | tail -6

dev.helix.code/internal/llm/providers/huggingface/client.go:22:	NewClient	100.0%
dev.helix.code/internal/llm/providers/huggingface/client.go:38:	Generate	92.3%
dev.helix.code/internal/llm/providers/together/client.go:22:	NewClient	100.0%
dev.helix.code/internal/llm/providers/together/client.go:50:	Generate	96.3%
total:								(statements)	94.5%
```

huggingface package: 92.6% of statements. together package: 96.4% of statements.
Combined total (both packages): 94.5% of statements.

## §1.1 self-check (paired-mutation proof the tests are not tautological)

### huggingface

Mutated `internal/llm/providers/huggingface/client_test.go` line asserting the
Authorization header:
```
assert.Equal(t, "Bearer "+apiKey, gotAuth)   ->   assert.Equal(t, "Bearer WRONG", gotAuth)
```
Re-ran `go test -run TestClient_Generate_RequestAndResponse -v`:
```
--- FAIL: TestClient_Generate_RequestAndResponse (0.00s)
    --- FAIL: TestClient_Generate_RequestAndResponse/default_model_used_when_request_has_no_model,_prompt_is_last_message (0.00s)
    --- FAIL: TestClient_Generate_RequestAndResponse/custom_model_honored,_last_of_multiple_messages_used_as_prompt (0.00s)
    --- FAIL: TestClient_Generate_RequestAndResponse/no_messages_produces_empty_prompt_but_still_calls_default_model (0.00s)
FAIL
```
Restored the file from backup, re-ran the same command: all 3 subtests PASS again.

### together

Mutated `internal/llm/providers/together/client_test.go` line asserting the
decoded request body:
```
assert.Equal(t, tt.wantBody, gotBody)   ->   assert.Equal(t, togetherRequest{Model:"WRONG"}, gotBody)
```
Re-ran `go test -run TestClient_Generate_RequestAndResponse -v`:
```
--- FAIL: TestClient_Generate_RequestAndResponse (0.00s)
    --- FAIL: TestClient_Generate_RequestAndResponse/default_model_and_defaults_applied_when_unset (0.00s)
    --- FAIL: TestClient_Generate_RequestAndResponse/custom_model,_explicit_max_tokens/temperature_preserved,_all_messages_converted_in_order (0.00s)
FAIL
```
Restored the file from backup, re-ran the same command: both subtests PASS again.

Both mutations prove the tests genuinely assert on the real client's built
request/parsed response rather than being tautological placeholders.

## Scope compliance

- No `git add`/`git commit` performed.
- No non-test source file edited (production `client.go` files in both packages
  are untouched — confirmed by not appearing in any diff produced by this work).
- No `./...` run; only the two target packages were tested.
- No `--force` flags used anywhere.
