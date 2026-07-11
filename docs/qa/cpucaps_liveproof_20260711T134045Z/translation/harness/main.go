// Phase-3 CPU translation (NMT) end-to-end proof harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of the
// CPU translation provider (design: docs/research/07.2026/00_master/
// TRANSLATION_PROVIDER.md §4 — the CX-05 anti-gaming triple). It boots a stock
// LibreTranslate (Argos/OPUS-MT on CTranslate2) CPU container THROUGH the
// containers submodule compose.Orchestrator (§11.4.76, NOT ad-hoc podman),
// drives the LibreTranslate `/translate` + `/detect` routes for a known golden
// reference set, and asserts the three-criterion anti-gaming signature:
//
//	(1) not-identity + detected-target-language  (kills copy/passthrough)
//	(2) forward chrF vs an independent golden reference >= floor
//	(3) back-translation metamorphic chrF(B,S) >= margin
//
// plus determinism (§11.4.50). It carries the §11.4.107(10) golden-good/
// golden-bad analyzer self-validation and the §11.4.115 RED polarity (an
// identity-passthrough "warming" record MUST FAIL the analyzer — the exact
// untranslated-passthrough bluff the design §2.5 forbids).
//
// chrF is deterministic, reference-based, language-agnostic character-n-gram
// F-score (sacrebleu-style, char_order=6, beta=2, whitespace-stripped), so the
// analyzer is pure and self-validatable.
//
// Subcommands:
//
//	boot-up   <compose-file> <project>              boot via containers submodule
//	boot-down <compose-file> <project>              tear down (single-owner cleanup)
//	boot-status <compose-file> <project>            print service status
//	probe     <base-url> <pair-idx> <out-record>    forward+detect+back -> record JSON
//	analyze   <record.json> <chrfFloor> <backMargin> analyze one record -> PASS/FAIL
//	determinism <recA.json> <recB.json>             assert forward+back byte-identical
//	selfvalidate <good-record.json> <chrfFloor> <backMargin>
//	              PASS on the good record; derive identity/wrong-lang/garbage/empty
//	              BAD variants, each MUST FAIL. Proves the analyzer cannot be fooled.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
)

// pairFixture is a known translation probe: a source sentence, its language, a
// target language, and an INDEPENDENT human golden reference translation
// (authored before observing the model output — the reference-based forward
// adequacy oracle, not tuned to the model's output). These are TEST FIXTURE
// probes, not user-facing content, so they are legitimately literal here.
type pairFixture struct {
	Name       string
	Source     string
	SourceLang string
	Target     string
	Golden     string
}

var fixtures = []pairFixture{
	{
		Name:       "en->fr",
		Source:     "The book is on the table.",
		SourceLang: "en",
		Target:     "fr",
		Golden:     "Le livre est sur la table.",
	},
	{
		Name:       "en->de",
		Source:     "The book is on the table.",
		SourceLang: "en",
		Target:     "de",
		Golden:     "Das Buch ist auf dem Tisch.",
	},
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3translate <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "boot-up":
		cmdBoot(true)
	case "boot-down":
		cmdBoot(false)
	case "boot-status":
		cmdStatus()
	case "probe":
		cmdProbe()
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
	// args[2]=compose-file, args[3]=project-name
	if len(os.Args) < 4 {
		fatal("need <compose-file> <project>")
	}
	return compose.ComposeProject{
		Name:     os.Args[3],
		File:     os.Args[2],
		Services: []string{"libretranslate"},
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
		// Detach-only + host-side readiness poll (the proven Phase-2/Phase-3 path
		// on this host's podman-compose shim, where `up --wait` is unsupported).
		// NOTE: no WithForceRecreate — under this host's podman-compose provider
		// `up --force-recreate` creates the pod but leaves it unstarted (the pod
		// infra that owns the host-port mapping never comes up, so the service is
		// unreachable). Freshness is instead guaranteed by a UNIQUE per-run
		// project name + a pre-clean boot-down (§11.4.108/§11.4.139), so plain
		// `up -d` always starts a fresh, correctly-rendered container.
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s libretranslate via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s libretranslate (volumes removed) via containers submodule orchestrator\n", p.Name)
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

// ---- LibreTranslate HTTP client (form-urlencoded — the lowest-common-
// denominator body every LibreTranslate version accepts via request.values) ----

func httpForm(base, path string, form url.Values) ([]byte, int) {
	httpc := &http.Client{Timeout: 90 * time.Second}
	resp, err := httpc.Post(base+path,
		"application/x-www-form-urlencoded",
		bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		fatal("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode
}

// translate posts /translate and returns the translatedText (scalar q shape).
func translate(base, q, source, target string) string {
	form := url.Values{}
	form.Set("q", q)
	form.Set("source", source)
	form.Set("target", target)
	form.Set("format", "text")
	body, code := httpForm(base, "/translate", form)
	if code != http.StatusOK {
		fatal("POST /translate status=%d body=%s", code, string(body))
	}
	var r struct {
		TranslatedText string `json:"translatedText"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		fatal("parse /translate response %q: %v", string(body), err)
	}
	return r.TranslatedText
}

// detectLang posts /detect and returns the top {language, confidence}.
func detectLang(base, q string) (string, float64) {
	form := url.Values{}
	form.Set("q", q)
	body, code := httpForm(base, "/detect", form)
	if code != http.StatusOK {
		fatal("POST /detect status=%d body=%s", code, string(body))
	}
	var arr []struct {
		Confidence float64 `json:"confidence"`
		Language   string  `json:"language"`
	}
	if err := json.Unmarshal(body, &arr); err != nil {
		fatal("parse /detect response %q: %v", string(body), err)
	}
	if len(arr) == 0 {
		return "", 0
	}
	return arr[0].Language, arr[0].Confidence
}

// ---- probe record ----

type probeRecord struct {
	Pair                      string  `json:"pair"`
	Source                    string  `json:"source"`
	SourceLang                string  `json:"source_lang"`
	Target                    string  `json:"target"`
	Golden                    string  `json:"golden"`
	Forward                   string  `json:"forward"`
	ForwardDetected           string  `json:"forward_detected"`
	ForwardDetectedConfidence float64 `json:"forward_detected_confidence"`
	Back                      string  `json:"back"`
}

// cmdProbe drives the REAL service: forward /translate, /detect on the forward
// text, then a back /translate, and writes the assembled record.
func cmdProbe() {
	if len(os.Args) < 5 {
		fatal("usage: probe <base-url> <pair-idx> <out-record.json>")
	}
	base := os.Args[2]
	idx, err := strconv.Atoi(os.Args[3])
	if err != nil || idx < 0 || idx >= len(fixtures) {
		fatal("pair-idx out of range 0..%d", len(fixtures)-1)
	}
	out := os.Args[4]
	fx := fixtures[idx]

	fwd := translate(base, fx.Source, fx.SourceLang, fx.Target)
	det, conf := detectLang(base, fwd)
	back := translate(base, fwd, fx.Target, fx.SourceLang)

	rec := probeRecord{
		Pair:                      fx.Name,
		Source:                    fx.Source,
		SourceLang:                fx.SourceLang,
		Target:                    fx.Target,
		Golden:                    fx.Golden,
		Forward:                   fwd,
		ForwardDetected:           det,
		ForwardDetectedConfidence: conf,
		Back:                      back,
	}
	b, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("PROBE-OK %s wrote %s\n", fx.Name, out)
	fmt.Printf("    source   = %q\n", fx.Source)
	fmt.Printf("    forward  = %q (detected=%s conf=%.1f)\n", fwd, det, conf)
	fmt.Printf("    back     = %q\n", back)
	fmt.Printf("    golden   = %q\n", fx.Golden)
}

// ---- chrF: char-n-gram F-score (sacrebleu-style, char_order=6, beta=2,
// whitespace-stripped), deterministic, reference-based, language-agnostic ----

func stripWS(s string) []rune {
	var out []rune
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			continue
		}
		out = append(out, r)
	}
	return out
}

func charNgrams(rs []rune, n int) map[string]int {
	m := map[string]int{}
	for i := 0; i+n <= len(rs); i++ {
		m[string(rs[i:i+n])]++
	}
	return m
}

// chrF returns the character-n-gram F-beta score in [0,1].
func chrF(hyp, ref string, order int, beta float64) float64 {
	h := stripWS(hyp)
	r := stripWS(ref)
	if len(h) == 0 || len(r) == 0 {
		return 0
	}
	var sumP, sumR float64
	var nP, nR int
	for n := 1; n <= order; n++ {
		hg := charNgrams(h, n)
		rg := charNgrams(r, n)
		var totalH, totalR, match int
		for g, c := range hg {
			totalH += c
			if rc, ok := rg[g]; ok {
				match += min(c, rc)
			}
		}
		for _, c := range rg {
			totalR += c
		}
		if totalH > 0 {
			sumP += float64(match) / float64(totalH)
			nP++
		}
		if totalR > 0 {
			sumR += float64(match) / float64(totalR)
			nR++
		}
	}
	if nP == 0 || nR == 0 {
		return 0
	}
	avgP := sumP / float64(nP)
	avgR := sumR / float64(nR)
	if avgP == 0 && avgR == 0 {
		return 0
	}
	b2 := beta * beta
	denom := b2*avgP + avgR
	if denom == 0 {
		return 0
	}
	return (1 + b2) * avgP * avgR / denom
}

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// ---- analyzer (pure, deterministic) — the CX-05 anti-gaming triple ----

type analyzeResult struct {
	pass        bool
	reasons     []string
	notIdentity bool
	targetMatch bool
	fwdChrF     float64
	backChrF    float64
	chrFFloor   float64
	backMargin  float64
}

func analyze(rec probeRecord, chrfFloor, backMargin float64) analyzeResult {
	res := analyzeResult{pass: true, chrFFloor: chrfFloor, backMargin: backMargin}

	// Criterion 1: not-identity + detected target language.
	res.notIdentity = normalize(rec.Forward) != normalize(rec.Source) && strings.TrimSpace(rec.Forward) != ""
	res.targetMatch = strings.EqualFold(rec.ForwardDetected, rec.Target)
	if !res.notIdentity {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"identity/passthrough: forward %q == source %q (or empty)", rec.Forward, rec.Source))
	}
	if !res.targetMatch {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"detected language %q != requested target %q", rec.ForwardDetected, rec.Target))
	}

	// Criterion 2: forward chrF vs independent golden reference.
	res.fwdChrF = chrF(rec.Forward, rec.Golden, 6, 2)
	if res.fwdChrF < chrfFloor {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"forward chrF %.4f < floor %.4f (forward not adequate vs golden)", res.fwdChrF, chrfFloor))
	}

	// Criterion 3: back-translation metamorphic chrF(B,S) vs source.
	res.backChrF = chrF(rec.Back, rec.Source, 6, 2)
	if res.backChrF < backMargin {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"back-translation chrF %.4f < margin %.4f (round-trip not semantically close)", res.backChrF, backMargin))
	}
	return res
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s notIdentity=%v targetMatch=%v fwdChrF=%.4f(floor %.2f) backChrF=%.4f(margin %.2f)\n",
		tag, verdict, res.notIdentity, res.targetMatch,
		res.fwdChrF, res.chrFFloor, res.backChrF, res.backMargin)
	for _, r := range res.reasons {
		fmt.Printf("    reason: %s\n", r)
	}
}

func parseRecord(path string) probeRecord {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var r probeRecord
	if err := json.Unmarshal(b, &r); err != nil {
		fatal("parse %s: %v", path, err)
	}
	return r
}

func cmdAnalyze() {
	if len(os.Args) < 5 {
		fatal("usage: analyze <record.json> <chrfFloor> <backMargin>")
	}
	floor, _ := strconv.ParseFloat(os.Args[3], 64)
	margin, _ := strconv.ParseFloat(os.Args[4], 64)
	rec := parseRecord(os.Args[2])
	res := analyze(rec, floor, margin)
	printResult("RUNTIME-SIGNATURE("+rec.Pair+")", res)
	if !res.pass {
		os.Exit(1)
	}
}

func cmdDeterminism() {
	if len(os.Args) < 4 {
		fatal("usage: determinism <recA.json> <recB.json>")
	}
	a := parseRecord(os.Args[2])
	b := parseRecord(os.Args[3])
	if a.Forward != b.Forward {
		fmt.Printf("[DETERMINISM] FAIL: forward differs:\n  A=%q\n  B=%q\n", a.Forward, b.Forward)
		os.Exit(1)
	}
	if a.Back != b.Back {
		fmt.Printf("[DETERMINISM] FAIL: back differs:\n  A=%q\n  B=%q\n", a.Back, b.Back)
		os.Exit(1)
	}
	fmt.Printf("[DETERMINISM] PASS: forward+back byte-identical across two identical requests (%s)\n", a.Pair)
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing AND the
// §11.4.115 RED polarity: the golden-good real record MUST PASS, and each
// deliberately-degraded golden-bad variant MUST FAIL. If a bad variant PASSes,
// the analyzer is a bluff gate and this command exits non-zero.
func cmdSelfValidate() {
	if len(os.Args) < 5 {
		fatal("usage: selfvalidate <golden-good.json> <chrfFloor> <backMargin>")
	}
	floor, _ := strconv.ParseFloat(os.Args[3], 64)
	margin, _ := strconv.ParseFloat(os.Args[4], 64)
	good := parseRecord(os.Args[2])

	ok := true

	// golden-good MUST PASS.
	gr := analyze(good, floor, margin)
	printResult("GOLDEN-GOOD(expect PASS)", gr)
	if !gr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good did not PASS")
	}

	// golden-bad #1: identity / passthrough (the untranslated-"warming" bluff,
	// design §2.5). Forward == Source, detected == source lang. Must FAIL crit 1.
	id := good
	id.Forward = good.Source
	id.ForwardDetected = good.SourceLang
	id.Back = good.Source
	ir := analyze(id, floor, margin)
	printResult("GOLDEN-BAD-IDENTITY(expect FAIL)", ir)
	if ir.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: identity passthrough PASSed the analyzer")
	}

	// golden-bad #2: wrong-language. Fluent text in the WRONG target language
	// (fixed non-target string) + detected != target. Must FAIL crit 1 + crit 2.
	wl := good
	wl.Forward = "Der Hund läuft im Garten." // German where French (or vice-versa) is expected
	wl.ForwardDetected = "de"
	if good.Target == "de" {
		wl.Forward = "Le chien court dans le jardin." // French where German expected
		wl.ForwardDetected = "fr"
	}
	wl.Back = "The dog runs in the garden." // fluent but unrelated back
	wr := analyze(wl, floor, margin)
	printResult("GOLDEN-BAD-WRONGLANG(expect FAIL)", wr)
	if wr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: wrong-language PASSed the analyzer")
	}

	// golden-bad #3: garbage. Canned garbage forward + garbage back, but the
	// attacker FAKES detected==target (worst case). Must still FAIL crit 2 + 3.
	gb := good
	gb.Forward = "xqz plrrt zzzk wvv."
	gb.ForwardDetected = good.Target // faked to defeat criterion 1 in isolation
	gb.Back = "qqq wvv zzk plr."
	gbr := analyze(gb, floor, margin)
	printResult("GOLDEN-BAD-GARBAGE(expect FAIL)", gbr)
	if gbr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: garbage PASSed the analyzer")
	}

	// golden-bad #4: empty forward. Must FAIL all forward criteria.
	em := good
	em.Forward = ""
	em.ForwardDetected = ""
	em.Back = ""
	emr := analyze(em, floor, margin)
	printResult("GOLDEN-BAD-EMPTY(expect FAIL)", emr)
	if emr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: empty response PASSed the analyzer")
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
