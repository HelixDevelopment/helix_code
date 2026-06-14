package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"dev.helix.code/internal/approval"
	"dev.helix.code/internal/llm"
	"dev.helix.code/internal/tools"
)

// tool_loop.go — a REUSABLE multi-turn agentic tool-execution loop driver.
//
// RunToolLoop drives the canonical agent loop: Generate → (if tool calls) run
// them through the tool registry → feed the results back → Generate again, up to
// MaxTurns, until the model produces a final answer with no tool calls. It is
// decoupled from any UI (no TUI imports) and never hardcodes a model name — the
// model defaults to the provider's first advertised model (CONST-036/046).
//
// It reuses the existing single-turn driver DispatchTurn (tool_dispatch.go),
// which routes a turn's tool calls through tools.ToolRegistry's parallel
// dispatch and returns per-call results (raw Result + Err) mapped back in the
// LLM-requested order — exactly what the loop needs to (a) record a trace entry
// per call and (b) feed each result back to the next Generate.

const (
	// defaultMaxTurns bounds the loop when MaxTurns <= 0.
	defaultMaxTurns = 6
	// traceOutputMaxLen caps the per-call Output excerpt recorded in the trace.
	traceOutputMaxLen = 300
	// defaultMaxToolResultChars bounds the MODEL-FACING tool-result feedback
	// content (the role:"tool" message) when ToolLoopOptions.MaxToolResultChars
	// <= 0. A large tool output (e.g. fs_read of a 40KB governance file, or a
	// long grep / git log) fed back in full accumulates across turns and
	// overflows the model's context window — especially for smaller-context
	// ensemble members ("Please reduce the length of the messages or
	// completion"). Bounding each fed-back result keeps accumulated multi-turn
	// context bounded regardless of tool / model / file size.
	defaultMaxToolResultChars = 4000
)

// MaxTurnsExceededMarker is appended to FinalContent when the loop hits MaxTurns
// without the model producing a tool-call-free final answer. Callers can detect
// the truncation by checking ToolLoopResult.MaxTurnsHit (preferred) or by
// scanning FinalContent for this marker.
const MaxTurnsExceededMarker = "[tool-loop: max turns reached before a final answer]"

// ToolTraceEntry records one executed tool call inside the loop.
type ToolTraceEntry struct {
	// ToolName is the dispatched tool's name.
	ToolName string
	// Arguments is the argument map the model supplied for the call.
	Arguments map[string]interface{}
	// Output is the tool's output text, truncated to traceOutputMaxLen runes.
	Output string
	// Err is the tool's error rendered as a string ("" when the call succeeded).
	Err string
}

// ToolLoopResult is the outcome of a completed (or turn-capped) tool loop.
type ToolLoopResult struct {
	// FinalContent is the model's final text answer (or the last content seen
	// when MaxTurns was hit, with MaxTurnsExceededMarker appended).
	FinalContent string
	// Trace has one entry per executed tool call, in execution order across all
	// turns.
	Trace []ToolTraceEntry
	// Turns is the number of Generate calls made (>= 1).
	Turns int
	// MaxTurnsHit is true when the loop stopped because it reached MaxTurns
	// without a tool-call-free final answer.
	MaxTurnsHit bool
	// FinalMetadata carries the ProviderMetadata of the FINAL provider.Generate
	// response — the tool-call-free answer turn (or the last response seen when
	// MaxTurns was hit). It lets a caller render ensemble member info (e.g.
	// ensemble_participants / ensemble_selected_provider) from the response that
	// actually produced the answer. nil when the final response carried no
	// ProviderMetadata.
	FinalMetadata map[string]interface{}
}

// ToolLoopOptions configures RunToolLoop. The zero value is valid: MaxTurns
// defaults to 6, MaxConcurrency to the registry default, Model to the provider's
// first advertised model, and SystemPrompt is omitted when empty.
type ToolLoopOptions struct {
	// Model is the model to request. Empty ⇒ provider.GetModels()[0].Name.
	Model string
	// MaxTurns caps the number of Generate calls. <= 0 ⇒ defaultMaxTurns (6).
	MaxTurns int
	// MaxConcurrency bounds parallel tool dispatch within a turn. <= 0 ⇒ the
	// registry default.
	MaxConcurrency int
	// SystemPrompt, when non-empty, is prepended as a system message on the
	// FIRST Generate call (and carried through subsequent turns).
	SystemPrompt string
	// ReadOnlyOnly, when true, restricts the loop to read-only tools
	// (RequiresApproval() == approval.LevelReadOnly) at BOTH the offer step and
	// the execute step (defense-in-depth, §11.4.133 target safety):
	//   - OFFER: only LevelReadOnly tools are converted to []llm.Tool and
	//     advertised, so the model never even SEES write/shell tools.
	//   - EXECUTE: every requested tool call is re-checked against the registry;
	//     a call whose tool is not LevelReadOnly (or is unknown) is NOT
	//     dispatched — its trace entry carries a "not permitted" error fed back
	//     to the model, and the loop continues.
	// This guarantees an unattended loop (e.g. the TUI's nil-approval-manager
	// registry, where applyApprovalGate would otherwise let every tool through)
	// can never reach a write/shell tool. Default (false) ⇒ behaviour unchanged.
	ReadOnlyOnly bool
	// MaxToolResultChars bounds, in runes, the MODEL-FACING content of each
	// fed-back role:"tool" message (both permitted-tool outputs AND read-only
	// "not permitted" refusals). A tool output longer than this is truncated on
	// a rune boundary with a clear "…[truncated N of M chars]" marker before it
	// is appended to the conversation, so accumulated multi-turn context stays
	// bounded regardless of tool / model / file size. <= 0 ⇒
	// defaultMaxToolResultChars (4000). This is independent of the
	// ToolTraceEntry.Output display excerpt, which keeps its own
	// traceOutputMaxLen (~300) cap.
	MaxToolResultChars int
}

// RunToolLoop runs a multi-turn agentic tool-execution loop against provider.
//
// Behaviour:
//   - Builds []llm.Tool from registry (each registered tool's name/description/
//     JSON schema) and sets them on every Generate request. A nil registry ⇒ no
//     tools are offered and the loop reduces to a single plain Generate.
//   - Each turn calls provider.Generate. If the response has no ToolCalls, the
//     loop returns the response Content as FinalContent.
//   - Otherwise it appends the assistant turn, runs the tool calls through the
//     registry (via DispatchTurn), appends each tool's result as a message the
//     next Generate can read, records a ToolTraceEntry per call, and loops.
//   - If MaxTurns is reached without a final answer, it returns the last content
//     (with MaxTurnsExceededMarker appended) + the trace, MaxTurnsHit=true, and a
//     nil error.
//
// Model defaults to provider.GetModels()[0].Name when opts.Model == "".
func RunToolLoop(ctx context.Context, provider llm.Provider, registry *tools.ToolRegistry, messages []llm.Message, opts ToolLoopOptions) (*ToolLoopResult, error) {
	if provider == nil {
		return nil, fmt.Errorf("tool loop: provider is nil")
	}

	model := opts.Model
	if model == "" {
		if models := provider.GetModels(); len(models) > 0 {
			model = models[0].Name
		}
	}

	maxTurns := opts.MaxTurns
	if maxTurns <= 0 {
		maxTurns = defaultMaxTurns
	}

	// maxToolResultChars bounds the model-facing tool-result feedback content so
	// large outputs cannot accumulate across turns and overflow the context.
	maxToolResultChars := opts.MaxToolResultChars
	if maxToolResultChars <= 0 {
		maxToolResultChars = defaultMaxToolResultChars
	}

	// readOnlyAllow is the set of tool names that may be offered AND executed
	// when opts.ReadOnlyOnly is true. nil when ReadOnlyOnly is false (no
	// restriction). Computed once from the registry's per-tool approval levels.
	var readOnlyAllow map[string]bool
	if opts.ReadOnlyOnly {
		readOnlyAllow = readOnlyToolNames(registry)
	}

	llmTools := registryToLLMTools(registry, readOnlyAllow)

	// Build the working conversation: optional system prompt then the caller's
	// messages. The slice is copied so the caller's input is never mutated.
	convo := make([]llm.Message, 0, len(messages)+1)
	if opts.SystemPrompt != "" {
		convo = append(convo, llm.Message{Role: "system", Content: opts.SystemPrompt})
	}
	convo = append(convo, messages...)

	result := &ToolLoopResult{}
	lastContent := ""
	var lastMetadata map[string]interface{}

	for turn := 0; turn < maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		req := &llm.LLMRequest{
			Model:    model,
			Messages: convo,
			Tools:    llmTools,
		}

		resp, err := provider.Generate(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("tool loop: generate (turn %d): %w", turn+1, err)
		}
		result.Turns++
		if resp == nil {
			return nil, fmt.Errorf("tool loop: provider returned nil response (turn %d)", turn+1)
		}
		lastContent = resp.Content
		lastMetadata = resp.ProviderMetadata

		// No tool calls ⇒ this is the final answer.
		if len(resp.ToolCalls) == 0 {
			// Never return an empty (or whitespace-only) FinalContent. A final
			// turn with empty content + no tool calls is a real provider
			// outcome (an under-specified prompt, a model that ran its tools
			// then emitted nothing). If that empty string escapes, a caller
			// (e.g. the TUI) stores it as an assistant turn — and a strict
			// provider (HelixAgent: "assistant message must have content or
			// tool_calls") then 400s the NEXT request, breaking the whole
			// conversation. Synthesize a non-empty answer from the trace so the
			// assistant turn is always well-formed.
			result.FinalContent = nonEmptyFinal(resp.Content, result.Trace)
			result.FinalMetadata = resp.ProviderMetadata
			return result, nil
		}

		// Record the assistant turn so the next Generate sees the model's
		// own tool-call request in context. The assistant message MUST carry
		// resp.ToolCalls so each fed-back role:"tool" result below has a
		// matching tool_call (the OpenAI/Groq protocol requires every
		// tool_call_id to reference a tool_call on the preceding assistant
		// turn). Content may be empty when tool_calls are present — that is
		// valid; the message is NOT dropped.
		convo = append(convo, llm.Message{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// EXECUTE-step guard (defense-in-depth). In read-only-only mode, split
		// the requested calls into permitted (read-only) and refused
		// (non-read-only / unknown). Only permitted calls are dispatched through
		// the real registry; refused calls are NEVER executed — they get a
		// synthesized "not permitted" result fed back to the model. This holds
		// even if a write/shell tool slipped past the offer-filter, so a write
		// tool can never EXECUTE under ReadOnlyOnly.
		permittedIdx, dispatched := dispatchPermitted(ctx, registry, resp.ToolCalls, opts.MaxConcurrency, readOnlyAllow)

		// permPos maps an original call index → its position in the dispatched
		// slice (DispatchTurn returns results in the order of the permitted
		// calls). Refused calls are not present in dispatched.
		permPos := make(map[int]int, len(permittedIdx))
		for pos, origIdx := range permittedIdx {
			permPos[origIdx] = pos
		}

		for i, call := range resp.ToolCalls {
			var output, errStr string
			if pos, ok := permPos[i]; ok {
				output, errStr = callOutput(dispatched, pos, call)
			} else {
				// Refused: not executed. Synthesize a "not permitted" result.
				errStr = fmt.Sprintf("tool %q not permitted in read-only mode", call.Function.Name)
			}
			result.Trace = append(result.Trace, ToolTraceEntry{
				ToolName:  call.Function.Name,
				Arguments: call.Function.Arguments,
				Output:    truncate(output, traceOutputMaxLen),
				Err:       errStr,
			})
			// Feed the tool result back so the next Generate can read it. The
			// role:"tool" message MUST carry ToolCallID = the assistant
			// tool_call's id (call.ID) — the OpenAI/Groq protocol rejects a
			// role:"tool" message without a matching tool_call_id, and rejects
			// a dangling assistant tool_call that has no following tool result.
			// This holds for REFUSED (read-only) calls too: they still get a
			// role:"tool" message carrying call.ID + the "not permitted"
			// content, so the conversation stays well-formed and no tool_call
			// is left unanswered.
			feedback := output
			if errStr != "" {
				feedback = "error: " + errStr
			}
			// Bound the MODEL-FACING feedback so a large tool output cannot
			// accumulate across turns and overflow the model's context window.
			// Applied uniformly to permitted outputs AND read-only refusals.
			feedback = truncateForModel(feedback, maxToolResultChars)
			convo = append(convo, llm.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Name:       call.Function.Name,
				Content:    feedback,
			})
		}
	}

	// MaxTurns exhausted without a tool-call-free final answer. Carry the last
	// response's metadata so the caller still sees the ensemble/provider info
	// from the final turn that was made.
	result.MaxTurnsHit = true
	result.FinalMetadata = lastMetadata
	if strings.TrimSpace(lastContent) != "" {
		result.FinalContent = lastContent + "\n\n" + MaxTurnsExceededMarker
	} else {
		// No usable final content from the last turn. Synthesize an answer from
		// the trace (so the operator gets the gathered tool output, not a bare
		// marker) and still flag the truncation.
		synth := nonEmptyFinal("", result.Trace)
		result.FinalContent = synth + "\n\n" + MaxTurnsExceededMarker
	}
	return result, nil
}

// nonEmptyFinal returns a guaranteed-non-empty final answer. When content is
// non-blank it is returned unchanged. When content is empty/whitespace-only it
// synthesizes a brief answer from the trace: the last successful tool result is
// the most relevant artefact for the user's question (the model just ran it),
// so it is summarized; if no successful tool ran, a clear fallback sentence is
// returned. The result is NEVER empty — that is the whole point: an empty
// assistant turn breaks strict providers (HelixAgent) on the next request.
func nonEmptyFinal(content string, trace []ToolTraceEntry) string {
	if strings.TrimSpace(content) != "" {
		return content
	}
	// Walk the trace backwards for the last successful (Err=="") tool result.
	for i := len(trace) - 1; i >= 0; i-- {
		e := trace[i]
		if e.Err == "" && strings.TrimSpace(e.Output) != "" {
			return fmt.Sprintf("I inspected your codebase using the %s tool. Here is what it returned:\n\n%s",
				e.ToolName, strings.TrimSpace(e.Output))
		}
	}
	// No content and no usable tool output. Return a clear, non-empty fallback
	// rather than an empty string (CONST: never feed an empty assistant turn).
	return "I could not produce a final answer for this request. Please try rephrasing or asking a more specific question."
}

// registryToLLMTools converts every tool registered in registry into an
// []llm.Tool the provider can advertise. Each entry carries the tool's name,
// description, and JSON-schema parameters (sourced from Tool.Schema()). A nil
// registry returns nil (the loop then offers no tools).
//
// No existing registry→[]llm.Tool converter was found in internal/llm or
// internal/tools (the existing seam — ExecuteToolBatch — goes the other
// direction, executing calls), so this small converter is provided here.
// When allow is non-nil, ONLY tools whose Name() is a key in allow are offered
// (the read-only-only offer-filter). A nil allow offers every registered tool.
func registryToLLMTools(registry *tools.ToolRegistry, allow map[string]bool) []llm.Tool {
	if registry == nil {
		return nil
	}
	registered := registry.List()
	out := make([]llm.Tool, 0, len(registered))
	for _, tool := range registered {
		if allow != nil && !allow[tool.Name()] {
			continue
		}
		schema := tool.Schema()
		params := map[string]interface{}{
			"type":       schema.Type,
			"properties": schema.Properties,
		}
		if len(schema.Required) > 0 {
			params["required"] = schema.Required
		}
		out = append(out, llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  params,
			},
		})
	}
	return out
}

// readOnlyToolNames returns the set of registered tool names whose approval
// level is approval.LevelReadOnly — the only tools an ReadOnlyOnly loop may
// offer or execute. A nil registry returns an empty (non-nil) set so the caller
// treats every tool as not-allowed. Per-tool levels are read via the Tool
// interface's RequiresApproval() exposed by registry.List().
func readOnlyToolNames(registry *tools.ToolRegistry) map[string]bool {
	allow := make(map[string]bool)
	if registry == nil {
		return allow
	}
	for _, tool := range registry.List() {
		if tool.RequiresApproval() == approval.LevelReadOnly {
			allow[tool.Name()] = true
		}
	}
	return allow
}

// dispatchPermitted runs ONLY the calls permitted by allow through the real
// registry. When allow is nil (ReadOnlyOnly disabled) every call is permitted
// and the behaviour is identical to dispatching the whole turn. It returns the
// indices (into calls) of the permitted calls — in dispatch order — alongside
// the per-call DispatchTurn results for those permitted calls. Refused calls
// are never dispatched (so a write/shell tool never executes under the guard).
func dispatchPermitted(ctx context.Context, registry *tools.ToolRegistry, calls []llm.ToolCall, maxConcurrency int, allow map[string]bool) ([]int, []ToolDispatchResult) {
	if allow == nil {
		idx := make([]int, len(calls))
		for i := range calls {
			idx[i] = i
		}
		dispatched, _ := DispatchTurn(ctx, registry, calls, maxConcurrency)
		return idx, dispatched
	}

	permittedIdx := make([]int, 0, len(calls))
	permittedCalls := make([]llm.ToolCall, 0, len(calls))
	for i, call := range calls {
		if allow[call.Function.Name] {
			permittedIdx = append(permittedIdx, i)
			permittedCalls = append(permittedCalls, call)
		}
	}
	if len(permittedCalls) == 0 {
		return permittedIdx, nil
	}
	dispatched, _ := DispatchTurn(ctx, registry, permittedCalls, maxConcurrency)
	return permittedIdx, dispatched
}

// callOutput extracts the i-th dispatched call's output text + error string.
// DispatchTurn returns results in the SAME order as the input calls, so index i
// maps to call i. Defensive against a short/empty result slice (e.g. nil
// registry path, though RunToolLoop only dispatches when calls exist).
func callOutput(dispatched []ToolDispatchResult, i int, call llm.ToolCall) (output, errStr string) {
	if i >= len(dispatched) {
		return "", fmt.Sprintf("no dispatch result for tool %q", call.Function.Name)
	}
	d := dispatched[i]
	if d.Err != nil {
		return "", d.Err.Error()
	}
	return stringifyResult(d.Result), ""
}

// stringifyResult renders a tool's return value as text for the trace + the
// feedback message fed back to the model. It renders common types readably so
// a tool returning a struct/[]byte/Stringer is never delivered to the model as
// a raw %v dump (the canonical bug: fs_read returns *filesystem.FileContent,
// whose Content []byte field rendered via %v as a decimal byte array
// "&{/path [35 32 ...]}", which models misread as a binary/corrupted file).
//
// Order matters: nil → ""; string → as-is; error → .Error(); []byte →
// string(b); fmt.Stringer → .String() (this is the FileContent path, now that
// it implements String()); otherwise marshal to JSON, falling back to %v only
// if marshalling fails. This is defense-in-depth: ANY tool returning a
// Stringer, a []byte, or a JSON-able value renders readably for the model.
func stringifyResult(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case error:
		return t.Error()
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	}
	if b, err := json.MarshalIndent(v, "", "  "); err == nil {
		return string(b)
	}
	return fmt.Sprintf("%v", v)
}

// truncate caps s to at most n runes, appending an ellipsis marker when cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// truncateForModel caps the MODEL-FACING content s to at most max runes,
// appending a clear "…[truncated N of M chars]" marker when cut so the model
// knows the result was bounded (N = runes kept, M = original rune count). It
// truncates on a RUNE boundary so a multibyte rune is never split. max <= 0 is
// treated as the default bound (defaultMaxToolResultChars) — the caller already
// normalises, but this keeps the helper safe in isolation.
func truncateForModel(s string, max int) string {
	if max <= 0 {
		max = defaultMaxToolResultChars
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + fmt.Sprintf("\n…[truncated %d of %d chars]", max, len(r))
}
