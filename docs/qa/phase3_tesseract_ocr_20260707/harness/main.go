// Phase-3 CPU OCR (Tesseract) end-to-end proof harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the CPU OCR capability (submodules/helix_llm/services/ocr/). It boots the
// helix-ocr container THROUGH the containers submodule compose.Orchestrator
// (§11.4.76, NOT ad-hoc podman/docker — the compose file's own `build:`
// section is driven by `up --build`), renders KNOWN-text fixtures via the
// service's own /v1/render endpoint (so font/version drift between harness
// host and served container can never produce a broken fixture), OCRs them
// via /v1/ocr, and asserts the normalized-token + mean-confidence runtime
// signature. It also carries the §11.4.107(10) golden-good/golden-bad
// analyzer self-validation and the §11.4.115 RED polarity (a stub returning
// empty text/zero confidence MUST fail the analyzer BEFORE the real lane is
// even booted).
//
// Subcommands:
//
//	boot-up     <compose-file> <project>                 boot via containers submodule (--build)
//	boot-down   <compose-file> <project>                 tear down (single-owner cleanup)
//	boot-status <compose-file> <project>                 print service status
//	render      <base-url> <out.png> <mode> <text>        POST /v1/render -> save PNG
//	ocr         <base-url> <image.png> <out-response.json> POST /v1/ocr -> save raw JSON
//	analyze     <response.json> <expTokensCSV> <minMeanConf>  PASS/FAIL runtime signature
//	determinism <respA.json> <respB.json>                 assert identical OCR output
//	selfvalidate <golden-good.json> <expTokensCSV> <minMeanConf> <bad1.json> <bad2.json> <bad3.json>
//	              PASS on golden-good; each golden-bad fixture MUST FAIL.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3ocr <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "boot-up":
		cmdBoot(true)
	case "boot-down":
		cmdBoot(false)
	case "boot-status":
		cmdStatus()
	case "render":
		cmdRender()
	case "ocr":
		cmdOCR()
	case "analyze":
		cmdAnalyze()
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
	if len(os.Args) < 4 {
		fatal("need <compose-file> <project>")
	}
	return compose.ComposeProject{
		Name:     os.Args[3],
		File:     os.Args[2],
		Services: []string{"ocr"},
	}
}

func cmdBoot(up bool) {
	orch, err := compose.NewDefaultOrchestrator(".", nil)
	if err != nil {
		fatal("orchestrator: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	p := project()
	if up {
		// --build: the compose file's `build:` section (Containerfile in
		// submodules/helix_llm/services/ocr/) is built by the orchestrator's
		// own detected compose command — never a bare `podman build`
		// (§11.4.76/§11.4.161). No WithForceRecreate (Phase-3 embeddings/
		// translation lesson: on this host's podman-compose shim it leaves
		// the pod created-but-unstarted); freshness instead comes from a
		// unique per-run project name + pre-clean boot-down.
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
			compose.WithBuildFirst(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s ocr via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s ocr via containers submodule orchestrator\n", p.Name)
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

// ---- /v1/render + /v1/ocr client calls ----

type renderRequest struct {
	Text      string `json:"text"`
	Mode      string `json:"mode"`
	PointSize int    `json:"pointsize"`
}

func cmdRender() {
	if len(os.Args) < 6 {
		fatal("usage: render <base-url> <out.png> <mode> <text>")
	}
	base, out, mode, text := os.Args[2], os.Args[3], os.Args[4], os.Args[5]
	reqBody, _ := json.Marshal(renderRequest{Text: text, Mode: mode, PointSize: 48})
	httpc := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpc.Post(base+"/v1/render", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		fatal("POST /v1/render: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/render status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("RENDER-OK: mode=%s wrote %s (%d bytes)\n", mode, out, len(body))
}

func cmdOCR() {
	if len(os.Args) < 5 {
		fatal("usage: ocr <base-url> <image.png> <out-response.json>")
	}
	base, img, out := os.Args[2], os.Args[3], os.Args[4]
	imgBytes, err := os.ReadFile(img)
	if err != nil {
		fatal("read %s: %v", img, err)
	}
	httpc := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpc.Post(base+"/v1/ocr", "application/octet-stream", bytes.NewReader(imgBytes))
	if err != nil {
		fatal("POST /v1/ocr: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/ocr status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("OCR-OK: status=200 wrote %s (%d bytes)\n", out, len(body))
}

// ---- analyzer (pure, deterministic) ----

type ocrWord struct {
	Text   string  `json:"text"`
	Conf   float64 `json:"conf"`
	Left   int     `json:"left"`
	Top    int     `json:"top"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	Line   int     `json:"line"`
	Block  int     `json:"block"`
}

type ocrResponse struct {
	Engine   string    `json:"engine"`
	Config   string    `json:"config"`
	Words    []ocrWord `json:"words"`
	FullText string    `json:"full_text"`
	MeanConf float64   `json:"mean_conf"`
}

func parseResponse(path string) ocrResponse {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var r ocrResponse
	if err := json.Unmarshal(b, &r); err != nil {
		fatal("parse %s: %v", path, err)
	}
	return r
}

// normalizeTokens uppercases the text, replaces every non-alphanumeric rune
// with a space, collapses whitespace, and returns the resulting token SET
// (not merely a substring haystack) — whole-word containment is a stronger
// anti-bluff guarantee than substring matching (a big garbage OCR blob could
// accidentally contain a short digit/letter run as a SUBSTRING; it is far
// less likely to contain it as a whole, whitespace-delimited token).
func normalizeTokens(s string) map[string]bool {
	upper := strings.ToUpper(s)
	var b strings.Builder
	for _, r := range upper {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	set := map[string]bool{}
	for _, tok := range strings.Fields(b.String()) {
		set[tok] = true
	}
	return set
}

// analyzeResult is the machine-checkable verdict of the OCR runtime signature.
type analyzeResult struct {
	pass          bool
	reasons       []string
	meanConf      float64
	fullText      string
	foundTokens   []string
	missingTokens []string
}

// analyze applies the runtime signature: ALL expected tokens present as
// whole words in the normalized extracted text, AND mean_conf >= the
// calibrated floor. Both criteria independently defeat different bluffs:
// token-presence defeats an empty/garbage/wrong-content response;
// mean-confidence defeats a response that happens to contain the right
// substrings via noise (extremely unlikely, but not assumed away).
func analyze(r ocrResponse, expectedTokens []string, minMeanConf float64) analyzeResult {
	res := analyzeResult{pass: true, meanConf: r.MeanConf, fullText: r.FullText}
	got := normalizeTokens(r.FullText)
	for _, tok := range expectedTokens {
		if got[tok] {
			res.foundTokens = append(res.foundTokens, tok)
		} else {
			res.missingTokens = append(res.missingTokens, tok)
		}
	}
	if len(res.missingTokens) > 0 {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"missing expected token(s): %s (normalized text=%q)",
			strings.Join(res.missingTokens, ","), r.FullText))
	}
	if r.MeanConf < minMeanConf {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"mean_conf %.2f < required floor %.2f", r.MeanConf, minMeanConf))
	}
	return res
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s mean_conf=%.2f found=%v missing=%v full_text=%q\n",
		tag, verdict, res.meanConf, res.foundTokens, res.missingTokens, res.fullText)
	for _, r := range res.reasons {
		fmt.Printf("    reason: %s\n", r)
	}
}

func parseTokensCSV(csv string) []string {
	var out []string
	for _, t := range strings.Split(csv, ",") {
		t = strings.TrimSpace(strings.ToUpper(t))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func cmdAnalyze() {
	if len(os.Args) < 5 {
		fatal("usage: analyze <response.json> <expTokensCSV> <minMeanConf>")
	}
	tokens := parseTokensCSV(os.Args[3])
	minConf, _ := strconv.ParseFloat(os.Args[4], 64)
	res := analyze(parseResponse(os.Args[2]), tokens, minConf)
	printResult("RUNTIME-SIGNATURE", res)
	if !res.pass {
		os.Exit(1)
	}
}

// expectFail is like cmdAnalyze but the CALLER declares this fixture is
// EXPECTED to fail (a golden-bad case); it prints the result and exits 0 if
// the analyzer correctly FAILED, exits 1 (SELF-VALIDATION VIOLATION) if the
// analyzer wrongly PASSED a fixture it should have rejected.
func expectFail(tag string, r ocrResponse, tokens []string, minConf float64) bool {
	res := analyze(r, tokens, minConf)
	printResult(tag+"(expect FAIL)", res)
	if res.pass {
		fmt.Printf("    SELF-VALIDATION VIOLATION: %s PASSed the analyzer\n", tag)
		return false
	}
	return true
}

func cmdDeterminism() {
	if len(os.Args) < 4 {
		fatal("usage: determinism <respA.json> <respB.json>")
	}
	a := parseResponse(os.Args[2])
	b := parseResponse(os.Args[3])
	if a.FullText != b.FullText {
		fmt.Printf("[DETERMINISM] FAIL: full_text %q != %q\n", a.FullText, b.FullText)
		os.Exit(1)
	}
	if a.MeanConf != b.MeanConf {
		fmt.Printf("[DETERMINISM] FAIL: mean_conf %.4f != %.4f\n", a.MeanConf, b.MeanConf)
		os.Exit(1)
	}
	if len(a.Words) != len(b.Words) {
		fmt.Printf("[DETERMINISM] FAIL: word count %d != %d\n", len(a.Words), len(b.Words))
		os.Exit(1)
	}
	fmt.Printf("[DETERMINISM] PASS: identical full_text (%d chars), mean_conf=%.2f, %d words across two identical requests\n",
		len(a.FullText), a.MeanConf, len(a.Words))
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing: the
// golden-good real response MUST PASS, and each deliberately-degraded
// golden-bad fixture (blank / noise / wrong-content) MUST FAIL. If any bad
// fixture PASSes, the analyzer is a bluff gate and this command exits
// non-zero.
func cmdSelfValidate() {
	if len(os.Args) < 8 {
		fatal("usage: selfvalidate <golden-good.json> <expTokensCSV> <minMeanConf> <bad-blank.json> <bad-noise.json> <bad-wrongtext.json>")
	}
	good := parseResponse(os.Args[2])
	tokens := parseTokensCSV(os.Args[3])
	minConf, _ := strconv.ParseFloat(os.Args[4], 64)
	blank := parseResponse(os.Args[5])
	noise := parseResponse(os.Args[6])
	wrong := parseResponse(os.Args[7])

	ok := true

	gr := analyze(good, tokens, minConf)
	printResult("GOLDEN-GOOD(expect PASS)", gr)
	if !gr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good did not PASS")
	}

	if !expectFail("GOLDEN-BAD-BLANK", blank, tokens, minConf) {
		ok = false
	}
	if !expectFail("GOLDEN-BAD-NOISE", noise, tokens, minConf) {
		ok = false
	}
	if !expectFail("GOLDEN-BAD-WRONGTEXT", wrong, tokens, minConf) {
		ok = false
	}

	if !ok {
		fmt.Println("[SELF-VALIDATION] FAIL")
		os.Exit(1)
	}
	fmt.Println("[SELF-VALIDATION] PASS: analyzer PASSes golden-good and FAILs all golden-bad fixtures")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
	os.Exit(2)
}
