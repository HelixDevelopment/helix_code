// Phase-3 CPU Speech-To-Text (Whisper) end-to-end proof harness.
//
// Deterministic, reproducible harness for the §11.4.108 runtime signature of
// the CPU STT capability (engine choice + citations: repo-root
// docs/qa/phase3_whisper_stt_20260707/RESULTS.md). It boots the HelixLLM
// faster-whisper CPU service (built from submodules/helix_llm/container/
// Containerfile.whisper — §11.4.74 code lives in the submodule) THROUGH the
// containers submodule compose.Orchestrator (§11.4.76, NOT ad-hoc podman),
// synthesizes KNOWN speech deterministically with espeak-ng, POSTs it to the
// OpenAI-compatible /v1/audio/transcriptions route, and asserts that the
// recovered transcript contains the expected key content words — a real
// recovery of known audio, unfakeable because the harness itself generated
// the audio from known text (§11.4.107 unfakeable-proof pattern).
//
// It also carries the §11.4.107(10) golden-good/golden-bad analyzer
// self-validation (real silence + real white-noise HTTP transcriptions, plus
// in-memory wrong-content/empty derivations) and the §11.4.115 RED polarity
// (a canned empty/wrong-text stub MUST FAIL the analyzer BEFORE the real
// green lane PASSes).
//
// Subcommands:
//
//	boot-up   <compose-file> <project>              boot via containers submodule
//	boot-down <compose-file> <project>              tear down (single-owner cleanup)
//	boot-status <compose-file> <project>            print service status
//	synth        <fixture-idx> <out.wav>            espeak-ng TTS of known text -> WAV
//	synth-silence <seconds> <out.wav>                ffmpeg anullsrc -> silence WAV
//	synth-noise   <seconds> <out.wav>                ffmpeg anoisesrc -> white-noise WAV
//	transcribe   <base-url> <wav> <out-record.json> POST multipart -> capture raw JSON
//	analyze      <record.json> <fixture-idx>        analyze one record -> PASS/FAIL
//	determinism  <recA.json> <recB.json>            assert identical transcript text
//	selfvalidate <good0.json> <good1.json> <silence.json> <noise.json>
//	              PASS on both real golden-good fixtures; the real silence/
//	              noise HTTP responses plus in-memory wrong-content/empty
//	              derivations MUST FAIL. Proves the analyzer cannot be fooled.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/pkg/compose"
)

// The two-utterance fixture set — the §11.4.108 runtime signature. These are
// TEST FIXTURE inputs synthesized by the harness itself (espeak-ng renders of
// literal known text), not user-facing product content, so they are
// legitimately literal here (they are the probe, not product text).
type fixture struct {
	Name          string
	Text          string
	ExpectedWords []string
}

var fixtures = []fixture{
	{
		Name:          "fox",
		Text:          "the quick brown fox jumps over the lazy dog",
		ExpectedWords: []string{"quick", "brown", "fox", "lazy", "dog"},
	},
	{
		Name:          "helloworld",
		Text:          "hello world one two three",
		ExpectedWords: []string{"hello", "world", "one", "two", "three"},
	},
}

func main() {
	if len(os.Args) < 2 {
		fatal("usage: phase3whisper <subcommand> [args...]")
	}
	switch os.Args[1] {
	case "boot-up":
		cmdBoot(true)
	case "boot-down":
		cmdBoot(false)
	case "boot-status":
		cmdStatus()
	case "synth":
		cmdSynth()
	case "synth-silence":
		cmdSynthSilence()
	case "synth-noise":
		cmdSynthNoise()
	case "transcribe":
		cmdTranscribe()
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
		Services: []string{"helixllm-stt"},
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
		// Detach + build-first (custom image, no pre-built upstream tag) +
		// host-side readiness poll (the proven Phase-3 path on this host's
		// podman-compose shim, where `up --wait` is unsupported). Freshness
		// is guaranteed by a UNIQUE per-run project name + a pre-clean
		// boot-down (§11.4.108/§11.4.139), so plain `up -d --build` always
		// starts a fresh, correctly-rendered container.
		if err := orch.Up(ctx, p,
			compose.WithUpDetach(true),
			compose.WithRemoveOrphans(true),
			compose.WithBuildFirst(true),
		); err != nil {
			fatal("compose up: %v", err)
		}
		fmt.Printf("UP-OK: %s helixllm-stt via containers submodule orchestrator\n", p.Name)
		return
	}
	if err := orch.Down(ctx, p,
		compose.WithDownRemoveVolumes(true),
		compose.WithDownRemoveOrphans(true),
	); err != nil {
		fatal("compose down: %v", err)
	}
	fmt.Printf("DOWN-OK: %s helixllm-stt (volumes removed) via containers submodule orchestrator\n", p.Name)
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

// ---- deterministic KNOWN-audio synthesis (§11.4.107 unfakeable proof) ----

func cmdSynth() {
	if len(os.Args) < 4 {
		fatal("usage: synth <fixture-idx> <out.wav>")
	}
	idx, err := strconv.Atoi(os.Args[2])
	if err != nil || idx < 0 || idx >= len(fixtures) {
		fatal("fixture-idx out of range 0..%d", len(fixtures)-1)
	}
	out := os.Args[3]
	fx := fixtures[idx]
	// espeak-ng: deterministic TTS render of the literal known text -> WAV.
	// -s 150 fixes the speaking rate so renders are stable across hosts.
	cmd := exec.Command("espeak-ng", "-v", "en", "-s", "150", "-w", out, fx.Text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fatal("espeak-ng synth %q: %v: %s", fx.Name, err, stderr.String())
	}
	fmt.Printf("SYNTH-OK: fixture=%s text=%q -> %s\n", fx.Name, fx.Text, out)
}

func cmdSynthSilence() {
	if len(os.Args) < 4 {
		fatal("usage: synth-silence <seconds> <out.wav>")
	}
	secs := os.Args[2]
	out := os.Args[3]
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "anullsrc=r=16000:cl=mono",
		"-t", secs, out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fatal("ffmpeg synth-silence: %v: %s", err, stderr.String())
	}
	fmt.Printf("SYNTH-SILENCE-OK: %ss digital silence -> %s\n", secs, out)
}

func cmdSynthNoise() {
	if len(os.Args) < 4 {
		fatal("usage: synth-noise <seconds> <out.wav>")
	}
	secs := os.Args[2]
	out := os.Args[3]
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "anoisesrc=d="+secs+":c=white:r=16000:a=0.3",
		out)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		fatal("ffmpeg synth-noise: %v: %s", err, stderr.String())
	}
	fmt.Printf("SYNTH-NOISE-OK: %ss white noise -> %s\n", secs, out)
}

// ---- OpenAI /v1/audio/transcriptions shape ----

type sttResponse struct {
	Text                string          `json:"text"`
	RawText             string          `json:"raw_text"`
	Language            string          `json:"language"`
	LanguageProbability float64         `json:"language_probability"`
	Duration            float64         `json:"duration"`
	Segments            json.RawMessage `json:"segments"`
	MaxNoSpeechProb     float64         `json:"max_no_speech_prob"`
	SilenceGuard        struct {
		Triggered bool    `json:"triggered"`
		Threshold float64 `json:"threshold"`
		Reason    string  `json:"reason"`
	} `json:"silence_guard"`
}

func cmdTranscribe() {
	if len(os.Args) < 5 {
		fatal("usage: transcribe <base-url> <wav> <out-record.json>")
	}
	base := os.Args[2]
	wavPath := os.Args[3]
	out := os.Args[4]

	wavBytes, err := os.ReadFile(wavPath)
	if err != nil {
		fatal("read %s: %v", wavPath, err)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, err := mw.CreateFormFile("file", "audio.wav")
	if err != nil {
		fatal("create form file: %v", err)
	}
	if _, err := fw.Write(wavBytes); err != nil {
		fatal("write form file: %v", err)
	}
	_ = mw.WriteField("model", "helixllm-stt")
	_ = mw.WriteField("response_format", "json")
	if err := mw.Close(); err != nil {
		fatal("close multipart: %v", err)
	}

	httpc := &http.Client{Timeout: 120 * time.Second}
	resp, err := httpc.Post(base+"/v1/audio/transcriptions", mw.FormDataContentType(), &body)
	if err != nil {
		fatal("POST /v1/audio/transcriptions: %v", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		fatal("POST /v1/audio/transcriptions status=%d body=%s", resp.StatusCode, string(respBody))
	}
	if err := os.WriteFile(out, respBody, 0o644); err != nil {
		fatal("write %s: %v", out, err)
	}
	var parsed sttResponse
	_ = json.Unmarshal(respBody, &parsed)
	fmt.Printf("TRANSCRIBE-OK: status=200 wrote %s (%d bytes) text=%q language=%s max_no_speech_prob=%.4f\n",
		out, len(respBody), parsed.Text, parsed.Language, parsed.MaxNoSpeechProb)
}

// ---- analyzer (pure, deterministic) ----

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// normalize lower-cases and strips punctuation, collapsing whitespace, per
// the task's runtime-signature spec ("normalized: lowercase, strip
// punctuation").
func normalize(s string) string {
	s = strings.ToLower(s)
	s = nonAlnum.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// numberWordToDigit is the documented Whisper text-normalization behavior
// (also observed and allowed in the prior proven p3_whisper_stt artifact:
// "forty two" -> "42") — the decoder renders spoken small numbers as digits,
// not literal number words. canonToken maps a digit token back to its word
// form so "1"/"one" compare equal; this is an honest equivalence for a real,
// documented engine behavior, not a weakening of the assertion (§11.4.6/
// §11.4.120: reconciling the check with a real, evidenced engine behavior,
// never fake-passing it).
var numberWordToDigit = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4",
	"five": "5", "six": "6", "seven": "7", "eight": "8", "nine": "9", "ten": "10",
}
var digitToNumberWord = func() map[string]string {
	m := map[string]string{}
	for w, d := range numberWordToDigit {
		m[d] = w
	}
	return m
}()

// canonToken maps a digit form ("1") to its number-word form ("one") so a
// transcript token and an expected word compare equal regardless of which
// form the decoder emitted.
func canonToken(tok string) string {
	if w, ok := digitToNumberWord[tok]; ok {
		return w
	}
	return tok
}

func parseResponse(path string) sttResponse {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var r sttResponse
	if err := json.Unmarshal(b, &r); err != nil {
		fatal("parse %s: %v", path, err)
	}
	return r
}

type analyzeResult struct {
	pass       bool
	reasons    []string
	transcript string
	normalized string
	missing    []string
}

// analyze applies the runtime signature: the normalized transcript MUST
// contain every expected key content word for the fixture.
func analyze(transcript string, expectedWords []string) analyzeResult {
	res := analyzeResult{pass: true, transcript: transcript}
	res.normalized = normalize(transcript)
	if res.normalized == "" {
		res.pass = false
		res.reasons = append(res.reasons, "empty/blank transcript")
	}
	tokens := map[string]bool{}
	for _, t := range strings.Fields(res.normalized) {
		tokens[canonToken(t)] = true
	}
	for _, w := range expectedWords {
		if !tokens[canonToken(w)] {
			res.missing = append(res.missing, w)
		}
	}
	if len(res.missing) > 0 {
		res.pass = false
		res.reasons = append(res.reasons, fmt.Sprintf("missing expected words: %v", res.missing))
	}
	return res
}

func printResult(tag string, res analyzeResult) {
	verdict := "PASS"
	if !res.pass {
		verdict = "FAIL"
	}
	fmt.Printf("[%s] %s transcript=%q normalized=%q missing=%v\n",
		tag, verdict, res.transcript, res.normalized, res.missing)
	for _, r := range res.reasons {
		fmt.Printf("    reason: %s\n", r)
	}
}

func cmdAnalyze() {
	if len(os.Args) < 4 {
		fatal("usage: analyze <record.json> <fixture-idx>")
	}
	idx, err := strconv.Atoi(os.Args[3])
	if err != nil || idx < 0 || idx >= len(fixtures) {
		fatal("fixture-idx out of range 0..%d", len(fixtures)-1)
	}
	rec := parseResponse(os.Args[2])
	res := analyze(rec.Text, fixtures[idx].ExpectedWords)
	printResult("RUNTIME-SIGNATURE("+fixtures[idx].Name+")", res)
	if !res.pass {
		os.Exit(1)
	}
}

// cmdAnalyzeStub is invoked via `analyze` too but the RED baseline uses a
// hand-built stub JSON file (run_proof.sh writes {"text":""} or a canned
// wrong string) fed through the same `analyze` subcommand — no separate
// code path, so the RED baseline exercises the IDENTICAL analyzer the GREEN
// lane uses (§11.4.115).

func cmdDeterminism() {
	if len(os.Args) < 4 {
		fatal("usage: determinism <recA.json> <recB.json>")
	}
	a := parseResponse(os.Args[2])
	b := parseResponse(os.Args[3])
	na, nb := normalize(a.Text), normalize(b.Text)
	if na != nb {
		fmt.Printf("[DETERMINISM] FAIL: normalized transcript differs:\n  A=%q\n  B=%q\n", na, nb)
		os.Exit(1)
	}
	fmt.Printf("[DETERMINISM] PASS: normalized transcript identical across two identical requests: %q\n", na)
}

// cmdSelfValidate is the §11.4.107(10) analyzer mutation-proofing AND the
// §11.4.115 RED polarity companion: BOTH real golden-good transcripts (the
// two distinct utterances) MUST PASS against their own fixture, and every
// golden-bad variant (real silence, real white-noise, in-memory wrong-content,
// in-memory empty) MUST FAIL. If any bad variant PASSes, the analyzer is a
// bluff gate and this command exits non-zero.
func cmdSelfValidate() {
	if len(os.Args) < 6 {
		fatal("usage: selfvalidate <good0.json> <good1.json> <silence.json> <noise.json>")
	}
	good0 := parseResponse(os.Args[2])
	good1 := parseResponse(os.Args[3])
	silence := parseResponse(os.Args[4])
	noise := parseResponse(os.Args[5])

	ok := true

	// golden-good #0 ("fox") MUST PASS against its own fixture.
	r0 := analyze(good0.Text, fixtures[0].ExpectedWords)
	printResult("GOLDEN-GOOD-0("+fixtures[0].Name+", expect PASS)", r0)
	if !r0.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good-0 did not PASS")
	}

	// golden-good #1 ("helloworld") MUST PASS against its own fixture — a
	// second distinct utterance so a one-clip fluke cannot pass unnoticed.
	r1 := analyze(good1.Text, fixtures[1].ExpectedWords)
	printResult("GOLDEN-GOOD-1("+fixtures[1].Name+", expect PASS)", r1)
	if !r1.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: golden-good-1 did not PASS")
	}

	// golden-bad: REAL HTTP transcription of digital silence, analyzed
	// against fixture 0's expected words. MUST FAIL (transcript empty/nonsense
	// never contains "quick brown fox lazy dog").
	rs := analyze(silence.Text, fixtures[0].ExpectedWords)
	printResult("GOLDEN-BAD-SILENCE(expect FAIL)", rs)
	if rs.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: silence PASSed the analyzer")
	}

	// golden-bad: REAL HTTP transcription of white noise, analyzed against
	// fixture 0's expected words. MUST FAIL.
	rn := analyze(noise.Text, fixtures[0].ExpectedWords)
	printResult("GOLDEN-BAD-NOISE(expect FAIL)", rn)
	if rn.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: white-noise PASSed the analyzer")
	}

	// golden-bad: wrong-content — the REAL "helloworld" transcript analyzed
	// against fixture 0's ("fox") expected words. Proves the analyzer rejects
	// a transcript of DIFFERENT-but-real, well-formed, non-empty speech.
	rw := analyze(good1.Text, fixtures[0].ExpectedWords)
	printResult("GOLDEN-BAD-WRONGCONTENT(expect FAIL)", rw)
	if rw.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: wrong-content (other fixture's real transcript) PASSed the analyzer")
	}

	// golden-bad: empty transcript. MUST FAIL.
	re := analyze("", fixtures[0].ExpectedWords)
	printResult("GOLDEN-BAD-EMPTY(expect FAIL)", re)
	if re.pass {
		ok = false
		fmt.Println("    SELF-VALIDATION VIOLATION: empty transcript PASSed the analyzer")
	}

	if !ok {
		fmt.Println("[SELF-VALIDATION] FAIL")
		os.Exit(1)
	}
	fmt.Println("[SELF-VALIDATION] PASS: analyzer PASSes both golden-good fixtures and FAILs every golden-bad variant")
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "ERROR: "+format+"\n", a...)
	os.Exit(2)
}
