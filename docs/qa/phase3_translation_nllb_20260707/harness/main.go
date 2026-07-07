// Phase-3 CPU translation NLLB-200-CTranslate2 PRIMARY-lane end-to-end proof
// harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the design-default PRIMARY translation lane (design:
// docs/research/07.2026/00_master/TRANSLATION_PROVIDER.md §1.2/§4): NLLB-200
// via CTranslate2, CPU-only, served behind a thin HTTP shim
// (harness/shim/{Dockerfile,server.py}). This EXTENDS the already-shipped
// LibreTranslate FALLBACK lane proof (docs/qa/phase3_translation_20260707/).
//
// It boots the shim container THROUGH the containers submodule
// compose.Orchestrator (§11.4.76, NOT ad-hoc podman), POSTs known source
// sentences to /translate for two language pairs, and asserts an UNFAKEABLE
// keyword-substring + not-identity runtime signature (task spec, verbatim):
//
//	en->de "The house is blue."  => forward MUST contain "haus" AND ANY OF
//	                                {"blau","blaues","blaue"}, and MUST NOT
//	                                equal the source.
//	en->fr "The cat sleeps."     => forward MUST contain "chat" AND ANY OF
//	                                {"dort","dormir"}, and MUST NOT equal the
//	                                source.
//
// plus determinism (§11.4.50: identical request twice => byte-identical
// output). It carries the §11.4.107(10) golden-good/golden-bad analyzer
// self-validation (identity/echo, empty, wrong-language variants, each MUST
// FAIL) and the §11.4.115 RED polarity (an echo-stub record — the "warming
// passthrough" bluff design §2.5 forbids — MUST FAIL the analyzer BEFORE the
// real lane is asked to PASS).
//
// Subcommands:
//
//	boot-up   <compose-file> <project>              boot via containers submodule
//	boot-down <compose-file> <project>              tear down (single-owner cleanup)
//	boot-status <compose-file> <project>            print service status
//	probe     <base-url> <pair-name> <out-record>   POST /translate -> record JSON
//	analyze   <record.json>                          analyze one record -> PASS/FAIL
//	determinism <recA.json> <recB.json>             assert forward byte-identical
//	selfvalidate <good.json> <other-good.json>
//	              PASS on the good record; derive identity/empty/wrong-language
//	              BAD variants (the last borrowing the OTHER pair's REAL
//	              captured forward text), each MUST FAIL. Proves the analyzer
//	              cannot be fooled.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
)

// pairFixture is a known translation probe: a source sentence in FLORES-200
// source/target codes, plus the UNFAKEABLE keyword contract the task spec
// mandates (a real translation must contain these; an echo/identity/garbage
// response cannot). These are TEST FIXTURE inputs, not user-facing content,
// so they are legitimately literal here (they are the probe, not product
// text).
type pairFixture struct {
	Name       string
	Source     string
	SourceLang string // FLORES-200 code
	Target     string // FLORES-200 code
	MustAll    []string // every one of these MUST appear (case-insensitive substring) in the forward translation
	MustAny    []string // AT LEAST ONE of these MUST appear (case-insensitive substring)
}

var fixtures = []pairFixture{
	{
		Name:       "en->de",
		Source:     "The house is blue.",
		SourceLang: "eng_Latn",
		Target:     "deu_Latn",
		MustAll:    []string{"haus"},
		MustAny:    []string{"blau", "blaues", "blaue"},
	},
	{
		Name:       "en->fr",
		Source:     "The cat sleeps.",
		SourceLang: "eng_Latn",
		Target:     "fra_Latn",
		MustAll:    []string{"chat"},
		MustAny:    []string{"dort", "dormir"},
	},
}

func fixtureByName(name string) pairFixture {
	for _, fx := range fixtures {
		if fx.Name == name {
			return fx
		}
	}
	fatal("unknown fixture pair %q", name)
	return pairFixture{}
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3translatenllb <subcommand> [args...]")
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
		Services: []string{"nllb-shim"},
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
		// Detach-only + host-side readiness poll (the proven Phase-2/Phase-3
		// path on this host's podman-compose shim, where `up --wait` is
		// unsupported). NOTE: no WithForceRecreate — on this host's
		// podman-compose provider `up --force-recreate` creates the pod but
		// leaves it unstarted. Freshness is instead guaranteed by a UNIQUE
		// per-run project name + a pre-clean boot-down (§11.4.108/§11.4.139),
		// so plain `up -d` always starts a fresh, correctly-rendered container.
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s nllb-shim via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s nllb-shim (volumes removed) via containers submodule orchestrator\n", p.Name)
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

// ---- shim HTTP client ----

func httpJSON(base, path string, reqBody any) ([]byte, int) {
	b, _ := json.Marshal(reqBody)
	httpc := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpc.Post(base+path, "application/json", bytes.NewReader(b))
	if err != nil {
		fatal("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode
}

func translate(base, q, source, target string) string {
	body, code := httpJSON(base, "/translate", map[string]string{
		"q": q, "source": source, "target": target,
	})
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

// ---- probe record ----

type probeRecord struct {
	Pair       string `json:"pair"`
	Source     string `json:"source"`
	SourceLang string `json:"source_lang"`
	Target     string `json:"target"`
	Forward    string `json:"forward"`
}

// cmdProbe drives the REAL service: POST /translate, capture the forward
// translation, write the record.
func cmdProbe() {
	if len(os.Args) < 5 {
		fatal("usage: probe <base-url> <pair-name> <out-record.json>")
	}
	base := os.Args[2]
	pairName := os.Args[3]
	out := os.Args[4]
	fx := fixtureByName(pairName)

	fwd := translate(base, fx.Source, fx.SourceLang, fx.Target)

	rec := probeRecord{
		Pair:       fx.Name,
		Source:     fx.Source,
		SourceLang: fx.SourceLang,
		Target:     fx.Target,
		Forward:    fwd,
	}
	b, _ := json.MarshalIndent(rec, "", "  ")
	if err := os.WriteFile(out, b, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	fmt.Printf("PROBE-OK %s wrote %s\n", fx.Name, out)
	fmt.Printf("    source  = %q\n", fx.Source)
	fmt.Printf("    forward = %q\n", fwd)
}

// ---- analyzer (pure, deterministic) — the task's UNFAKEABLE signature ----

func normalize(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

type analyzeResult struct {
	pass        bool
	reasons     []string
	notIdentity bool
	allOK       bool
	anyOK       bool
	missingAll  []string
}

func analyze(rec probeRecord, fx pairFixture) analyzeResult {
	res := analyzeResult{pass: true}
	fwdNorm := normalize(rec.Forward)
	srcNorm := normalize(rec.Source)
	lower := strings.ToLower(rec.Forward)

	// Criterion 1: not-identity / real-translation (kills the echo/passthrough bluff).
	res.notIdentity = strings.TrimSpace(rec.Forward) != "" && fwdNorm != srcNorm
	if !res.notIdentity {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"identity/passthrough: forward %q == source %q (or empty)", rec.Forward, rec.Source))
	}

	// Criterion 2: every MustAll keyword present (case-insensitive substring).
	res.allOK = true
	for _, kw := range fx.MustAll {
		if !strings.Contains(lower, strings.ToLower(kw)) {
			res.allOK = false
			res.missingAll = append(res.missingAll, kw)
		}
	}
	if !res.allOK {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"missing required keyword(s) %v in forward %q", res.missingAll, rec.Forward))
	}

	// Criterion 3: at least one MustAny keyword present.
	res.anyOK = len(fx.MustAny) == 0
	for _, kw := range fx.MustAny {
		if strings.Contains(lower, strings.ToLower(kw)) {
			res.anyOK = true
			break
		}
	}
	if !res.anyOK {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf(
			"none of the alternative keyword(s) %v present in forward %q", fx.MustAny, rec.Forward))
	}

	return res
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s notIdentity=%v allKeywordsOK=%v anyKeywordOK=%v\n",
		tag, verdict, res.notIdentity, res.allOK, res.anyOK)
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
	if len(os.Args) < 3 {
		fatal("usage: analyze <record.json>")
	}
	rec := parseRecord(os.Args[2])
	fx := fixtureByName(rec.Pair)
	res := analyze(rec, fx)
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
		fmt.Printf("[DETERMINISM] FAIL (%s): forward differs:\n  A=%q\n  B=%q\n", a.Pair, a.Forward, b.Forward)
		os.Exit(1)
	}
	fmt.Printf("[DETERMINISM] PASS: forward byte-identical across two identical requests (%s) = %q\n", a.Pair, a.Forward)
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing AND the
// §11.4.115 RED polarity: the golden-good real record MUST PASS, and each
// deliberately-degraded golden-bad variant MUST FAIL. If a bad variant
// PASSes, the analyzer is a bluff gate and this command exits non-zero.
//
// good  = the REAL captured record for one pair (e.g. en->de)
// other = the REAL captured record for the OTHER pair (e.g. en->fr) — its
//
//	genuine forward text becomes the "wrong-language" bad fixture for
//	`good`'s pair: real fluent translation, but in the WRONG language, so
//	`good`'s MustAll/MustAny keywords are absent.
func cmdSelfValidate() {
	if len(os.Args) < 4 {
		fatal("usage: selfvalidate <good.json> <other-good.json>")
	}
	good := parseRecord(os.Args[2])
	other := parseRecord(os.Args[3])
	fx := fixtureByName(good.Pair)

	ok := true

	// golden-good MUST PASS.
	gr := analyze(good, fx)
	printResult("GOLDEN-GOOD(expect PASS)", gr)
	if !gr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good did not PASS")
	}

	// golden-bad #1: identity / echo (the exact "warming passthrough" bluff
	// this harness's RED baseline also reproduces). forward == source.
	id := good
	id.Forward = good.Source
	ir := analyze(id, fx)
	printResult("GOLDEN-BAD-IDENTITY/ECHO(expect FAIL)", ir)
	if ir.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: identity/echo PASSed the analyzer")
	}

	// golden-bad #2: empty string.
	em := good
	em.Forward = ""
	er := analyze(em, fx)
	printResult("GOLDEN-BAD-EMPTY(expect FAIL)", er)
	if er.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: empty response PASSed the analyzer")
	}

	// golden-bad #3: wrong-language — a REAL, fluent, genuinely-translated
	// forward text (the OTHER pair's real captured output), but in the wrong
	// target language for THIS pair's keyword expectations. Proves the
	// analyzer rejects "a real translation, just not the one asked for" —
	// not only rejects garbage.
	wl := good
	wl.Forward = other.Forward
	wr := analyze(wl, fx)
	printResult("GOLDEN-BAD-WRONGLANG(expect FAIL)", wr)
	if wr.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: wrong-language PASSed the analyzer")
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
