// Phase-3 CPU embeddings end-to-end proof harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the CPU embeddings provider (design: docs/research/07.2026/00_master/
// EMBEDDINGS_PROVIDER.md §4). It boots the HF Text Embeddings Inference (TEI)
// CPU container THROUGH the containers submodule compose.Orchestrator
// (§11.4.76, NOT ad-hoc podman), POSTs a real sentence triple to the
// OpenAI-compatible /v1/embeddings route, and asserts the semantic-order
// cosine signature plus non-zero-norm, dimension, and determinism. It also
// carries the §11.4.107(10) golden-good/golden-bad analyzer self-validation
// and the §11.4.115 RED polarity (the assertion FAILs on a zero-vector
// embedding — the exact dim-1536 gateway stub the provider replaces).
//
// Subcommands:
//
//	boot-up   <compose-file> <project>            boot via containers submodule
//	boot-down <compose-file> <project>            tear down (single-owner cleanup)
//	boot-status <compose-file> <project>          print service status
//	embed     <base-url> <out-response.json>      POST the triple, capture raw JSON
//	cosine    <response.json> <expDim> <margin>   analyze one response -> PASS/FAIL
//	determinism <respA.json> <respB.json>         assert identical vectors
//	selfvalidate <golden-good.json> <margin>
//	              PASS on the good fixture; derive zero/shuffle/wrong-dim BAD
//	              variants, each MUST FAIL. Proves the analyzer cannot be fooled.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"digital.vasic.containers/pkg/compose"
)

// The sentence triple that is the runtime signature (design §4).
// A and A' are paraphrases (should be close); U is unrelated (should be far).
// These are TEST FIXTURE inputs to the model, not user-facing content, so they
// are legitimately literal here (they are the probe, not product text).
const (
	sentA  = "The cat sat on the mat."
	sentAp = "A feline rested on the rug."
	sentU  = "Quarterly revenue rose four percent."
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3embed <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "boot-up":
		cmdBoot(true)
	case "boot-down":
		cmdBoot(false)
	case "boot-status":
		cmdStatus()
	case "embed":
		cmdEmbed()
	case "cosine":
		cmdCosine()
	case "determinism":
		cmdDeterminism()
	case "selfvalidate":
		cmdSelfValidate()
	default:
		fatal("unknown subcommand: %s", os.Args[1])
	}
}

// ---- container boot via the containers submodule (§11.4.76) ----

func project() compose.ComposeProject {
	// args[2]=compose-file, args[3]=project-name
	if len(os.Args) < 4 {
		fatal("need <compose-file> <project>")
	}
	return compose.ComposeProject{
		Name:     os.Args[3],
		File:     os.Args[2],
		Services: []string{"tei-embed"},
	}
}

func cmdBoot(up bool) {
	orch, err := compose.NewDefaultOrchestrator(".", nil)
	if err != nil {
		fatal("orchestrator: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	p := project()
	if up {
		// Detach-only + host-side readiness poll (the Phase-2 proven path on
		// this host's podman-compose shim, where `up --wait` is unsupported).
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
			// Force-recreate so every boot starts a FRESH container reflecting
			// the current TEI_MODEL_ID — never a stale leftover container from
			// a prior run (§11.4.108/§11.4.139 clean-target integrity).
			compose.WithForceRecreate(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s tei-embed via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s tei-embed (volumes removed) via containers submodule orchestrator\n", p.Name)
}

func cmdStatus() {
	orch, err := compose.NewDefaultOrchestrator(".", nil)
	if err != nil {
		fatal("orchestrator: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sts, err := orch.Status(ctx, project())
	if err != nil {
		fatal("status: %v", err)
	}
	for _, s := range sts {
		fmt.Printf("%s state=%s health=%s ports=%v exit=%d\n",
			s.Name, s.State, s.Health, s.Ports, s.ExitCode)
	}
	if len(sts) == 0 {
		fmt.Println("(no services reported)")
	}
}

// ---- OpenAI /v1/embeddings shapes ----

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingItem struct {
	Object    string    `json:"object"`
	Index     int       `json:"index"`
	Embedding []float64 `json:"embedding"`
}

type embeddingResponse struct {
	Object string          `json:"object"`
	Data   []embeddingItem `json:"data"`
	Model  string          `json:"model"`
	Usage  json.RawMessage `json:"usage"`
}

func cmdEmbed() {
	if len(os.Args) < 4 {
		fatal("usage: embed <base-url> <out-response.json>")
	}
	base := os.Args[2]
	out := os.Args[3]
	reqBody, _ := json.Marshal(embeddingRequest{
		Model: "helix-embed",
		Input: []string{sentA, sentAp, sentU},
	})
	httpc := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpc.Post(base+"/v1/embeddings",
		"application/json", bytes.NewReader(reqBody))
	if err != nil {
		fatal("POST /v1/embeddings: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/embeddings status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("EMBED-OK: status=200 wrote %s (%d bytes)\n", out, len(body))
}

// ---- analyzer (pure, deterministic) ----

func parseResponse(path string) embeddingResponse {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var r embeddingResponse
	if err := json.Unmarshal(b, &r); err != nil {
		fatal("parse %s: %v", path, err)
	}
	return r
}

func l2norm(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x * x
	}
	return math.Sqrt(s)
}

func cosine(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return math.NaN()
	}
	var dot float64
	for i := range a {
		dot += a[i] * b[i]
	}
	na, nb := l2norm(a), l2norm(b)
	if na == 0 || nb == 0 {
		return math.NaN()
	}
	return dot / (na * nb)
}

// analyzeResult is the machine-checkable verdict of the runtime signature.
type analyzeResult struct {
	pass       bool
	reasons    []string
	dim        int
	normA      float64
	normAp     float64
	normU      float64
	cosAAp     float64 // related pair
	cosAU      float64 // unrelated pair
	margin     float64
}

// analyze applies the §4 runtime signature to a parsed response.
// expDim <= 0 means "do not pin an exact dimension, only require >0 and equal".
func analyze(r embeddingResponse, expDim int, minMargin float64) analyzeResult {
	res := analyzeResult{pass: true}
	if len(r.Data) < 3 {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf("need 3 vectors, got %d", len(r.Data)))
		return res
	}
	// Order by index so A=0, A'=1, U=2 regardless of array order.
	byIdx := map[int][]float64{}
	for _, d := range r.Data {
		byIdx[d.Index] = d.Embedding
	}
	va, vap, vu := byIdx[0], byIdx[1], byIdx[2]
	if va == nil || vap == nil || vu == nil {
		res.pass = false
		res.reasons = append(res.reasons, "missing index 0/1/2")
		return res
	}
	res.dim = len(va)
	// Dimension check: all equal, >0, and == expDim when pinned.
	if len(va) == 0 || len(vap) != len(va) || len(vu) != len(va) {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"dimension mismatch: %d/%d/%d", len(va), len(vap), len(vu)))
	}
	if expDim > 0 && len(va) != expDim {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"expected dim %d, got %d", expDim, len(va)))
	}
	// Non-zero L2 norm (kills the zero-vector stub).
	res.normA, res.normAp, res.normU = l2norm(va), l2norm(vap), l2norm(vu)
	const eps = 1e-9
	if res.normA < eps || res.normAp < eps || res.normU < eps {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"zero/near-zero norm A=%.6g A'=%.6g U=%.6g",
			res.normA, res.normAp, res.normU))
	}
	// Semantic-order cosine margin.
	res.cosAAp = cosine(va, vap)
	res.cosAU = cosine(va, vu)
	res.margin = res.cosAAp - res.cosAU
	if math.IsNaN(res.margin) {
		res.pass = false
		res.reasons = append(res.reasons, "cosine NaN (zero-norm or dim mismatch)")
	} else if res.margin < minMargin {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"semantic-order margin %.4f < required %.4f", res.margin, minMargin))
	}
	return res
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s dim=%d |A|=%.4f |A'|=%.4f |U|=%.4f cos(A,A')=%.4f cos(A,U)=%.4f margin=%.4f\n",
		tag, verdict, res.dim, res.normA, res.normAp, res.normU,
		res.cosAAp, res.cosAU, res.margin)
	for _, r := range res.reasons {
		fmt.Printf("    reason: %s\n", r)
	}
}

func cmdCosine() {
	if len(os.Args) < 5 {
		fatal("usage: cosine <response.json> <expDim> <margin>")
	}
	expDim, _ := strconv.Atoi(os.Args[3])
	margin, _ := strconv.ParseFloat(os.Args[4], 64)
	res := analyze(parseResponse(os.Args[2]), expDim, margin)
	printResult("RUNTIME-SIGNATURE", res)
	if !res.pass {
		os.Exit(1)
	}
}

func cmdDeterminism() {
	if len(os.Args) < 4 {
		fatal("usage: determinism <respA.json> <respB.json>")
	}
	a := parseResponse(os.Args[2])
	b := parseResponse(os.Args[3])
	if len(a.Data) != len(b.Data) {
		fmt.Printf("[DETERMINISM] FAIL: vector count %d != %d\n", len(a.Data), len(b.Data))
		os.Exit(1)
	}
	for i := range a.Data {
		va, vb := a.Data[i].Embedding, b.Data[i].Embedding
		if len(va) != len(vb) {
			fmt.Printf("[DETERMINISM] FAIL: idx %d dim %d != %d\n", i, len(va), len(vb))
			os.Exit(1)
		}
		for j := range va {
			if va[j] != vb[j] {
				fmt.Printf("[DETERMINISM] FAIL: idx %d elem %d %.17g != %.17g\n",
					i, j, va[j], vb[j])
				os.Exit(1)
			}
		}
	}
	fmt.Printf("[DETERMINISM] PASS: %d vectors byte-identical across two identical requests\n", len(a.Data))
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing AND the
// §11.4.115 RED polarity: the golden-good real response MUST PASS, and each
// deliberately-degraded golden-bad variant MUST FAIL. If a bad variant PASSes,
// the analyzer is a bluff gate and this command exits non-zero.
func cmdSelfValidate() {
	if len(os.Args) < 4 {
		fatal("usage: selfvalidate <golden-good.json> <margin>")
	}
	margin, _ := strconv.ParseFloat(os.Args[3], 64)
	good := parseResponse(os.Args[2])
	// The expected dimension is DERIVED from the served model's own output
	// (config-injected model → whatever dim it emits), never a hardcoded
	// guess (§11.4.6). The wrong-dim golden-bad below truncates by one and
	// must then fail this derived-dim check.
	expDim := 0
	if len(good.Data) > 0 {
		expDim = len(good.Data[0].Embedding)
	}

	ok := true

	// golden-good MUST PASS.
	gr := analyze(good, expDim, margin)
	printResult("GOLDEN-GOOD(expect PASS)", gr)
	if !gr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good did not PASS")
	}

	// golden-bad #1: zero-vector (the exact dim-1536 gateway stub shape).
	// This is the §11.4.115 RED baseline — the analyzer MUST FAIL it.
	zero := cloneResp(good)
	for i := range zero.Data {
		zero.Data[i].Embedding = make([]float64, 1536) // dim-1536 zeros = stub
	}
	zr := analyze(zero, 0, margin)
	printResult("GOLDEN-BAD-ZEROVEC(expect FAIL)", zr)
	if zr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: zero-vector stub PASSed the analyzer")
	}

	// golden-bad #2: shuffled order — make A closer to U than to A'.
	// Swap the A' and U vectors so cos(A,A') < cos(A,U): semantic order broken.
	shuf := cloneResp(good)
	byIdx := map[int]int{}
	for i, d := range shuf.Data {
		byIdx[d.Index] = i
	}
	if ia, iu := byIdx[1], byIdx[2]; shuf.Data != nil {
		shuf.Data[ia].Embedding, shuf.Data[iu].Embedding =
			shuf.Data[iu].Embedding, shuf.Data[ia].Embedding
	}
	sr := analyze(shuf, expDim, margin)
	printResult("GOLDEN-BAD-SHUFFLED(expect FAIL)", sr)
	if sr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: shuffled-order PASSed the analyzer")
	}

	// golden-bad #3: wrong-dimension — truncate every vector by one element.
	wd := cloneResp(good)
	for i := range wd.Data {
		e := wd.Data[i].Embedding
		if len(e) > 1 {
			wd.Data[i].Embedding = e[:len(e)-1]
		}
	}
	wr := analyze(wd, expDim, margin) // expDim pinned -> truncation fails dim check
	printResult("GOLDEN-BAD-WRONGDIM(expect FAIL)", wr)
	if wr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: wrong-dimension PASSed the analyzer")
	}

	if !ok {
		fmt.Println("[SELF-VALIDATION] FAIL")
		os.Exit(1)
	}
	fmt.Println("[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures")
}

func cloneResp(r embeddingResponse) embeddingResponse {
	out := r
	out.Data = make([]embeddingItem, len(r.Data))
	for i, d := range r.Data {
		nd := d
		nd.Embedding = append([]float64(nil), d.Embedding...)
		out.Data[i] = nd
	}
	return out
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
	os.Exit(2)
}
