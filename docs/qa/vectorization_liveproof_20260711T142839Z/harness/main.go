// Vectorization (raster->SVG, vtracer default path) end-to-end proof
// harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the CPU vectorization capability (submodules/helix_llm/services/vectorize/).
// It boots the helix-vectorize container THROUGH the containers submodule
// compose.Orchestrator (§11.4.76, NOT ad-hoc podman/docker), vectorizes a
// REAL repo image asset via the service's own /v1/vectorize endpoint,
// re-rasterizes the produced SVG via the service's own /v1/rasterize
// endpoint (so renderer-version drift between harness host and served
// container can never confound the comparison), and asserts a windowed-SSIM
// fidelity runtime signature between the source raster and the
// re-rasterized round-trip. It also carries the §11.4.107(10)
// golden-good/golden-bad analyzer self-validation, a §1.1 paired mutation
// (the fidelity check is temporarily neutered to prove it is load-bearing,
// then reverted), and a §11.4.50 determinism check (identical input ->
// byte-identical SVG across two calls).
//
// Subcommands:
//
//	boot-up      <compose-file> <project>                         boot via containers submodule (--build)
//	boot-down    <compose-file> <project>                         tear down (single-owner cleanup)
//	boot-status  <compose-file> <project>                         print service status
//	vectorize    <base-url> <image.png> <out-response.json> [preset]   POST /v1/vectorize -> save raw JSON
//	rasterize    <base-url> <svg-path> <out.png> <width> <height> POST /v1/rasterize?w=&h= -> save PNG
//	analyze      <source.png> <candidate.png> <minSSIM>            PASS/FAIL windowed-SSIM runtime signature
//	determinism  <respA.json> <respB.json>                         assert byte-identical SVG across two calls
//	selfvalidate <source.png> <good.png> <minSSIM> <bad1.png> <bad2.png>
//	             PASS on golden-good; each golden-bad fixture MUST FAIL.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
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
		fatal("usage: vectorizeliveproof <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "boot-up":
		cmdBoot(true)
	case "boot-down":
		cmdBoot(false)
	case "boot-status":
		cmdStatus()
	case "vectorize":
		cmdVectorize()
	case "rasterize":
		cmdRasterize()
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
		Services: []string{"vectorize"},
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
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
			compose.WithBuildFirst(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s vectorize via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s vectorize via containers submodule orchestrator\n", p.Name)
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

// ---- /v1/vectorize + /v1/rasterize client calls ----

type vectorizeResponse struct {
	Engine       string `json:"engine"`
	Preset       string `json:"preset"`
	SourceFormat string `json:"source_format"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	SVG          string `json:"svg"`
}

func cmdVectorize() {
	if len(os.Args) < 5 {
		fatal("usage: vectorize <base-url> <image.png> <out-response.json> [preset]")
	}
	base, img, out := os.Args[2], os.Args[3], os.Args[4]
	preset := ""
	if len(os.Args) >= 6 {
		preset = os.Args[5]
	}
	imgBytes, err := os.ReadFile(img)
	if err != nil {
		fatal("read %s: %v", img, err)
	}
	url := base + "/v1/vectorize"
	if preset != "" {
		url += "?preset=" + preset
	}
	httpc := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpc.Post(url, "application/octet-stream", bytes.NewReader(imgBytes))
	if err != nil {
		fatal("POST /v1/vectorize: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/vectorize status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	var vr vectorizeResponse
	_ = json.Unmarshal(body, &vr)
	fmt.Printf("VECTORIZE-OK: engine=%s preset=%q %dx%d svg_bytes=%d wrote %s\n",
		vr.Engine, vr.Preset, vr.Width, vr.Height, len(vr.SVG), out)
}

func cmdRasterize() {
	if len(os.Args) < 7 {
		fatal("usage: rasterize <base-url> <svg-path> <out.png> <width> <height>")
	}
	base, svgPath, out, wS, hS := os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]
	svgBytes, err := os.ReadFile(svgPath)
	if err != nil {
		fatal("read %s: %v", svgPath, err)
	}
	url := fmt.Sprintf("%s/v1/rasterize?w=%s&h=%s", base, wS, hS)
	httpc := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpc.Post(url, "image/svg+xml", bytes.NewReader(svgBytes))
	if err != nil {
		fatal("POST /v1/rasterize: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/rasterize status=%d body=%s", resp.StatusCode, string(body))
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("RASTERIZE-OK: status=200 wrote %s (%d bytes)\n", out, len(body))
}

// ---- analyzer (pure, deterministic, windowed SSIM) ----

// loadGray decodes a PNG file into a grayscale float64 matrix (BT.601 luma),
// row-major [y][x].
func loadGray(path string) [][]float64 {
	f, err := os.Open(path)
	if err != nil {
		fatal("open %s: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		fatal("decode %s: %v", path, err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([][]float64, h)
	for y := 0; y < h; y++ {
		row := make([]float64, w)
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			// r,g,b are 16-bit (0-65535); scale to 8-bit range then apply
			// standard BT.601 luma weights.
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(bl>>8)
			row[x] = 0.299*rf + 0.587*gf + 0.114*bf
		}
		out[y] = row
	}
	return out
}

// ssimWindowed computes the mean SSIM (structural similarity, Wang et al.
// 2004 formula, applied per non-overlapping 8x8 block, unweighted average
// across blocks — a simplified but legitimate structural-similarity
// estimator; C1/C2 are the paper's standard 8-bit-range constants) between
// two equal-dimension grayscale matrices.
func ssimWindowed(a, b [][]float64) (float64, error) {
	h := len(a)
	if h == 0 || len(b) != h {
		return 0, fmt.Errorf("height mismatch: %d vs %d", h, len(b))
	}
	w := len(a[0])
	if len(b[0]) != w {
		return 0, fmt.Errorf("width mismatch: %d vs %d", w, len(b[0]))
	}
	const win = 8
	const C1 = 6.5025  // (0.01*255)^2
	const C2 = 58.5225 // (0.03*255)^2

	var sum float64
	var n int
	for y := 0; y+win <= h; y += win {
		for x := 0; x+win <= w; x += win {
			var sumA, sumB, sumAA, sumBB, sumAB float64
			for dy := 0; dy < win; dy++ {
				for dx := 0; dx < win; dx++ {
					va := a[y+dy][x+dx]
					vb := b[y+dy][x+dx]
					sumA += va
					sumB += vb
					sumAA += va * va
					sumBB += vb * vb
					sumAB += va * vb
				}
			}
			const N = float64(win * win)
			muA, muB := sumA/N, sumB/N
			varA := sumAA/N - muA*muA
			varB := sumBB/N - muB*muB
			covAB := sumAB/N - muA*muB
			s := ((2*muA*muB + C1) * (2*covAB + C2)) / ((muA*muA + muB*muB + C1) * (varA + varB + C2))
			sum += s
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("no 8x8 windows fit in %dx%d image", w, h)
	}
	return sum / float64(n), nil
}

// analyzeResult is the machine-checkable verdict of the fidelity runtime
// signature.
type analyzeResult struct {
	pass bool
	ssim float64
	err  error
}

// analyze applies the fidelity runtime signature: the windowed SSIM between
// the source raster and the candidate (re-rasterized SVG round-trip or a
// golden-bad fixture) must clear the calibrated floor. This function is
// NEVER neutered/bypassed in this tracked source — the §1.1 paired-mutation
// proof that this check is load-bearing is performed via a disposable /tmp
// scratch binary (see run_proof.sh step "PAIRED MUTATION") that duplicates
// this logic with the check deliberately broken, so no bypass switch of any
// kind is ever present in committed code (§11.4.84).
func analyze(sourcePath, candidatePath string, minSSIM float64) analyzeResult {
	a := loadGray(sourcePath)
	b := loadGray(candidatePath)
	s, err := ssimWindowed(a, b)
	if err != nil {
		return analyzeResult{pass: false, err: err}
	}
	return analyzeResult{pass: s >= minSSIM, ssim: s}
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	if res.err != nil {
		fmt.Printf("[%s] FAIL (error: %v)\n", tag, res.err)
		return
	}
	fmt.Printf("[%s] %s ssim=%.4f\n", tag, verdict, res.ssim)
}

func cmdAnalyze() {
	if len(os.Args) < 5 {
		fatal("usage: analyze <source.png> <candidate.png> <minSSIM>")
	}
	minSSIM, err := strconv.ParseFloat(os.Args[4], 64)
	if err != nil {
		fatal("bad minSSIM: %v", err)
	}
	res := analyze(os.Args[2], os.Args[3], minSSIM)
	printResult("RUNTIME-SIGNATURE", res)
	if !res.pass {
		os.Exit(1)
	}
}

// expectFail is like cmdAnalyze but the CALLER declares this fixture is
// EXPECTED to fail (a golden-bad case); returns true if the analyzer
// correctly FAILED it, false (SELF-VALIDATION VIOLATION) if the analyzer
// wrongly PASSED a fixture it should have rejected.
func expectFail(tag, sourcePath, candidatePath string, minSSIM float64) bool {
	res := analyze(sourcePath, candidatePath, minSSIM)
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
	ab, err := os.ReadFile(os.Args[2])
	if err != nil {
		fatal("read %s: %v", os.Args[2], err)
	}
	bb, err := os.ReadFile(os.Args[3])
	if err != nil {
		fatal("read %s: %v", os.Args[3], err)
	}
	var a, b vectorizeResponse
	if err := json.Unmarshal(ab, &a); err != nil {
		fatal("parse %s: %v", os.Args[2], err)
	}
	if err := json.Unmarshal(bb, &b); err != nil {
		fatal("parse %s: %v", os.Args[3], err)
	}
	if a.SVG != b.SVG {
		fmt.Printf("[DETERMINISM] FAIL: SVG bytes differ (%d vs %d chars) across two identical /v1/vectorize calls\n",
			len(a.SVG), len(b.SVG))
		os.Exit(1)
	}
	if strings.TrimSpace(a.SVG) == "" {
		fmt.Println("[DETERMINISM] FAIL: SVG is empty")
		os.Exit(1)
	}
	fmt.Printf("[DETERMINISM] PASS: byte-identical SVG (%d chars) across two independent /v1/vectorize calls on the same input\n", len(a.SVG))
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing: the
// golden-good real re-rasterized PNG MUST PASS, and each deliberately
// degenerate golden-bad SVG's rasterized PNG (blank canvas, flat-color
// rect — genuinely no traced structure) MUST FAIL. If any bad fixture
// PASSes, the analyzer is a bluff gate and this command exits non-zero.
func cmdSelfValidate() {
	if len(os.Args) < 7 {
		fatal("usage: selfvalidate <source.png> <good.png> <minSSIM> <bad1.png> <bad2.png>")
	}
	source, good := os.Args[2], os.Args[3]
	minSSIM, err := strconv.ParseFloat(os.Args[4], 64)
	if err != nil {
		fatal("bad minSSIM: %v", err)
	}
	bad1, bad2 := os.Args[5], os.Args[6]

	ok := true

	gr := analyze(source, good, minSSIM)
	printResult("GOLDEN-GOOD(expect PASS)", gr)
	if !gr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good did not PASS")
	}

	if !expectFail("GOLDEN-BAD-BLANK", source, bad1, minSSIM) {
		ok = false
	}
	if !expectFail("GOLDEN-BAD-FLATCOLOR", source, bad2, minSSIM) {
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
