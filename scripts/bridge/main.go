// Package main implements helix-bridge — a thin multi-provider LLM client
// the AI agent uses for testing / debugging / fixing / polishing work
// against the HelixCode ecosystem. Per operator mandate 2026-05-13:
//
//   "Make sure you create a 'bridge' between you and the HelixTrack
//    System and its Submodule components (Sybsystems) so you can do
//    all LLM related work during the testing, debugging, fixing and
//    polishing!"
//
// Loads API keys from `$HOME/api_keys.sh` (preferred) or `.env`
// (fallback) via the existing scripts/load_api_keys.sh contract,
// then picks the first provider whose key is present and exercises
// it through its native HTTP API. No mocks, no simulation, no
// hardcoded model lists — each provider call goes against the real
// upstream endpoint and returns the live response.
//
// Usage:
//
//   helix-bridge ask "Why is the sky blue?"
//   helix-bridge providers          # list providers w/ key-set status
//   helix-bridge probe              # ping each configured provider
//
// Anti-bluff (CONST-035): a PASS in `probe` means the provider
// returned a 200 response to a real /models or /chat probe; not a
// trivial "API key envvar is non-empty" check.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider describes one LLM endpoint with a model usable for chat completions.
type Provider struct {
	Name      string
	EnvVar    string
	BaseURL   string
	Model     string // a small/cheap default; the CLI prefers it
	IsAvailable func(apiKey string) bool
	Ask       func(apiKey, prompt string) (string, error)
}

var providers = []Provider{
	{
		Name: "groq", EnvVar: "GROQ_API_KEY",
		BaseURL: "https://api.groq.com/openai/v1/chat/completions",
		Model:   "llama-3.3-70b-versatile",
		Ask:     askOpenAI("https://api.groq.com/openai/v1/chat/completions"),
	},
	{
		Name: "gemini", EnvVar: "GEMINI_API_KEY",
		BaseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
		Model:   "gemini-2.5-flash",
		Ask:     askGemini,
	},
	{
		Name: "mistral", EnvVar: "MISTRAL_API_KEY",
		BaseURL: "https://api.mistral.ai/v1/chat/completions",
		Model:   "mistral-small-latest",
		Ask:     askOpenAI("https://api.mistral.ai/v1/chat/completions"),
	},
	{
		Name: "deepseek", EnvVar: "DEEPSEEK_API_KEY",
		BaseURL: "https://api.deepseek.com/chat/completions",
		Model:   "deepseek-chat",
		Ask:     askOpenAI("https://api.deepseek.com/chat/completions"),
	},
	{
		Name: "openrouter", EnvVar: "OPENROUTER_API_KEY",
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
		// openrouter's "google/gemini-2.0-flash-exp:free" 404'd on the
		// initial probe; meta-llama/llama-3.3-70b-instruct is consistently
		// served by openrouter as a stable free-tier endpoint.
		Model: "meta-llama/llama-3.3-70b-instruct:free",
		Ask:   askOpenAI("https://openrouter.ai/api/v1/chat/completions"),
	},
}

func init() {
	for i := range providers {
		p := &providers[i]
		model := p.Model
		p.IsAvailable = func(k string) bool { return k != "" }
		// Wrap Ask to inject the chosen model.
		base := p.Ask
		p.Ask = func(k, prompt string) (string, error) {
			return base(k, makeBody(p.Name, model, prompt))
		}
	}
}

type openAIMessage struct {
	// Lowercase JSON keys required by OpenAI-compatible APIs (Groq,
	// Mistral, DeepSeek, OpenRouter). Without explicit tags Go marshals
	// the field names verbatim ("Role"/"Content"), which providers
	// reject with HTTP 400 "missing field role".
	Role    string `json:"role"`
	Content string `json:"content"`
}
type openAIReq struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}
type openAIResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func makeBody(provider, model, prompt string) string {
	if provider == "gemini" {
		body, _ := json.Marshal(map[string]any{
			"contents": []any{map[string]any{
				"parts": []any{map[string]string{"text": prompt}},
			}},
		})
		return string(body)
	}
	body, _ := json.Marshal(openAIReq{
		Model:    model,
		Messages: []openAIMessage{{Role: "user", Content: prompt}},
	})
	return string(body)
}

func askOpenAI(url string) func(apiKey, body string) (string, error) {
	return func(apiKey, body string) (string, error) {
		req, err := http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		buf, _ := io.ReadAll(resp.Body)
		if resp.StatusCode/100 != 2 {
			return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(buf), 400))
		}
		var r openAIResp
		if err := json.Unmarshal(buf, &r); err != nil {
			return "", fmt.Errorf("decode: %w (body: %s)", err, truncate(string(buf), 400))
		}
		if r.Error != nil {
			return "", errors.New(r.Error.Message)
		}
		if len(r.Choices) == 0 {
			return "", fmt.Errorf("empty choices (body: %s)", truncate(string(buf), 400))
		}
		return r.Choices[0].Message.Content, nil
	}
}

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct{ Text string } `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct{ Message string } `json:"error,omitempty"`
}

func askGemini(apiKey, body string) (string, error) {
	// Google switched the preferred auth to the `x-goog-api-key` header
	// in v1beta. The legacy `?key=` query-param works for some keys but
	// not consumer ones — observed a 400 "API Key not found" on our key
	// when query-param-authenticated. Header form works for both.
	url := "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent"
	req, err := http.NewRequest("POST", url, bytes.NewReader([]byte(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", apiKey)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncate(string(buf), 400))
	}
	var r geminiResp
	if err := json.Unmarshal(buf, &r); err != nil {
		return "", fmt.Errorf("decode: %w (body: %s)", err, truncate(string(buf), 400))
	}
	if r.Error != nil {
		return "", errors.New(r.Error.Message)
	}
	if len(r.Candidates) == 0 || len(r.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("empty candidates (body: %s)", truncate(string(buf), 400))
	}
	return r.Candidates[0].Content.Parts[0].Text, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func cmdProviders() {
	fmt.Println("HELIX-BRIDGE PROVIDERS:")
	for _, p := range providers {
		k := os.Getenv(p.EnvVar)
		status := "ABSENT"
		if k != "" {
			status = fmt.Sprintf("PRESENT (%d chars)", len(k))
		}
		fmt.Printf("  %-12s %-22s %s\n", p.Name, p.EnvVar, status)
	}
}

func cmdAsk(prompt string) {
	for _, p := range providers {
		k := os.Getenv(p.EnvVar)
		if k == "" {
			continue
		}
		fmt.Fprintf(os.Stderr, "[helix-bridge] dispatching to %s (model=%s)\n", p.Name, p.Model)
		out, err := p.Ask(k, prompt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[helix-bridge]   %s FAIL: %v\n", p.Name, err)
			continue
		}
		fmt.Println(out)
		return
	}
	fmt.Fprintln(os.Stderr, "[helix-bridge] no provider responded successfully")
	os.Exit(1)
}

func cmdProbe() {
	probePrompt := "Reply with the exact string OK and nothing else."
	failed := 0
	for _, p := range providers {
		k := os.Getenv(p.EnvVar)
		if k == "" {
			fmt.Printf("  [%-12s] SKIP (no key)\n", p.Name)
			continue
		}
		out, err := p.Ask(k, probePrompt)
		if err != nil {
			fmt.Printf("  [%-12s] FAIL: %v\n", p.Name, truncate(err.Error(), 120))
			failed++
			continue
		}
		short := strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
		if len(short) > 80 {
			short = short[:80] + "..."
		}
		fmt.Printf("  [%-12s] OK: %s\n", p.Name, short)
	}
	if failed > 0 {
		fmt.Fprintf(os.Stderr, "%d providers failed\n", failed)
		os.Exit(1)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: helix-bridge {ask <prompt> | providers | probe}")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "ask":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: helix-bridge ask <prompt>")
			os.Exit(2)
		}
		cmdAsk(strings.Join(os.Args[2:], " "))
	case "providers":
		cmdProviders()
	case "probe":
		cmdProbe()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}
