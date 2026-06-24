# Constitution-submodule advance investigation (§11.4.71 / §11.4.37 / §11.4.55)

- **Date:** 2026-06-24
- **Mode:** READ-ONLY investigation. No pull, no gitlink bump, no cascade. Submodule HEAD left at `1d9e3ce`.
- **Recorded gitlink (meta-repo):** `1d9e3ce` (`1d9e3cecf209315273b06beb4cfb7d1013f0f32b`)
- **Upstream `github/main` (fetched):** `e1bb125` (`e1bb12502d297ccef376698fc2cadd6a92d2b112`)
- **Submodule path:** `./constitution` (repo root — NOT `submodules/constitution`)

## 1. New commits `1d9e3ce..e1bb125` (oldest→newest)

| SHA | Subject | One-line effect |
|-----|---------|-----------------|
| `bbe84e3` | test(anti-bluff): scanner-triage — real observable assertions + honored skip annotations (§11.4) | Test-hardening INSIDE the constitution submodule's own `scripts/workable-items/*_test.go` (drove anti-bluff-scan survivors 140→13). No mandate change, no thresholds weakened. **No consumer action.** |
| `c670154` | feat(§11.4.167,§11.4.168): feature-work-stream lifecycle + visual-doc-validation mandates + §11.4.157 carrier back-fill | **Adds TWO new universal mandates** (§11.4.167, §11.4.168) to `Constitution.md` + mirrors them into the submodule's own CLAUDE/AGENTS/QWEN/GEMINI carriers. This is the only commit that introduces new rules. |
| `b2a7948` | feat(§11.4.73): author default-md.css + default-pdf.css project styling stylesheets | Authors `styles/default-md.css` + `styles/default-pdf.css` (referenced by every consumer's export pipeline via `--css/--stylesheet` but previously missing → unstyled exports + weasyprint `FileNotFoundError`). Lives in the constitution submodule; consumed by reference (§11.4.80-style). |
| `e1bb125` | docs(§11.4.65/§11.4.73): regenerate constitution carrier siblings (html/pdf/docx) with default CSS | Pure regen of the submodule's OWN `*.html/*.pdf/*.docx` siblings with the new CSS. Doc-export hygiene inside the submodule. **No consumer action.** |

## 2. New mandates + required consumer (HelixCode) actions

### §11.4.167 — Big-work-item feature work-stream lifecycle (User mandate 2026-06-23)
Every BIG work item (new feature OR drastic/large fix) MUST be its own isolated feature work-stream: own full project copy in a sibling dir (`<project>_<slug>`), own `feature/<slug>` branch, own per-feature builds + feature tags, kept SEPARATE from trunk until operator manually approves after testing, with trunk merged INTO every stream regularly. Sub-clauses (A)–(H): (A) one big item = one sibling copy, small changes stay on trunk; (B) disk-feasible CoW/reflink clone MANDATORY (`cp -a --reflink=auto` on btrfs/XFS/ZFS/APFS, else `git worktree` + externalized build output), naive deep copies / `git clone` per stream FORBIDDEN, ~0-disk verified by captured evidence before relied on; (C) own branch + greppable tags (`<base>-feat-<slug>[-<iter>]` per §11.4.151), NEVER merged to trunk until §11.4.40 full retest GREEN on clean target → §11.4.41 merge-first → §11.4.113 ff-only; (D) trunk merged INTO every live stream per-trunk-tag/daily (`git merge`, never rebase a shared/tagged branch); (E) feature branch + mirrored feature tag cascades to EVERY touched submodule the moment it is modified, untouched submodules stay trunk-pinned; (F) single-builder FIFO build queue + finite-device test queue (§11.4.119 per-device exclusive ownership), idle `out/` GC'd; (G) every change crosses the full review/impact/anti-bluff gauntlet — a stream is NOT a quality carve-out; (H) resumable, data-safe lifecycle via a stream registry + atomic `.partial`→rename create + §9.2 retire-backup. Universal (§11.4.17); consumer fills CoW mechanism, naming, queue counts, big-item threshold per §11.4.35. Recommended gates: `CM-FEATURE-WORKSTREAM-COW-CLONE` / `-NO-MERGE-UNTIL-APPROVED` / `-TRUNK-SYNC-CADENCE` / `-SUBMODULE-CASCADE`.

**HelixCode required action:** (1) cascade the compact `11.4.167` mirror into root `CLAUDE.md`/`AGENTS.md`/`GEMINI.md`/`QWEN.md` (§11.4.157/§11.4.26 — they currently top out at `11.4.165`); (2) eventually wire the four recommended gates + paired §1.1 mutations (separate gate-code work item). Note: macOS host → APFS supports `cp -c` clonefile / `cp -a --reflink` is not the macOS flag — the consumer §11.4.35 instantiation must pick the APFS CoW mechanism (`cp -c` clonefile or APFS snapshot).

### §11.4.168 — Exported-document independent content + textual + full-visual validation (User mandate 2026-06-23)
Every generated/exported document (HTML/PDF/DOCX/any §11.4.65 export-scope format) MUST pass INDEPENDENT validation by a reviewer structurally separate from the generator (NEVER author self-check), on BOTH source AND every exported artifact, ALWAYS, across THREE layers: (1) CONTENT — export faithfully carries source intent+data, nothing dropped/truncated/garbled; (2) TEXTUAL — human-readable, NO raw markup / diagram-source (Mermaid `gantt`/`graph`/`flowchart`/`sequenceDiagram`/…)/unrendered code-fence leaking as body text; (3) FULL VISUAL — diagrams render as IMAGES not source, layout intact, verified by RENDERING the export (`pdftotext` catches raw-source-as-text + `pdfimages` confirms rendered images + `pdftoppm`→OCR confirms human-readable content) with captured evidence (§11.4.5/.69/.107). Analyzer self-validated golden-good/golden-bad (§11.4.107(10)); iterate to clean GO (§11.4.134). Forensic anchor: 2026-06-23 raw-gantt-in-PDF leak the green suite missed (§11.4.138 operator-escape) — `class="mermaid"` HTML grep is a SOURCE check, NOT an exported-artifact check. Honest boundary: confirms exported artifact is faithful/readable/rendered — does NOT prove source content correct nor replace the §11.4.65 mtime-parity freshness gate. Universal (§11.4.17). Recommended gate: `CM-EXPORTED-DOC-VISUALLY-VALIDATED`.

**HelixCode required action:** (1) cascade the compact `11.4.168` mirror into the four root carriers (§11.4.157/§11.4.26); (2) eventually wire `CM-EXPORTED-DOC-VISUALLY-VALIDATED` + paired §1.1 mutation, and apply the three-layer validation to HelixCode's own doc-export pipeline (Status/Status_Summary/feature-status HTML/PDF/DOCX). HelixCode does have a doc-export/QA surface, so this binds — but as wiring, not as a blocker on the current code/test cycle.

### §11.4.73 CSS authoring (`styles/default-md.css` + `styles/default-pdf.css`)
Not a new mandate — fills a long-standing gap. Consumed by reference at `constitution/styles/*.css` (NOT copied, §11.4.80-style). Affects HelixCode only if/when HelixCode runs the constitution-driven doc-export pipeline pointing `--css`/`--stylesheet` at these files.

## 3. Classification of the advance

- **(a) Anchors needing only consumer-fleet propagation-literal added (cascade):** §11.4.167 and §11.4.168 — add the compact mirror (literal `11.4.167` / `11.4.168`) to root `CLAUDE.md`/`AGENTS.md`/`GEMINI.md`/`QWEN.md`. Currently ALL FOUR carriers top out at `11.4.165` → uniform 2-anchor gap (confirmed: `167=0 168=0` in every carrier).
- **(b) New enforceable gates the consumer must wire:** `CM-COVENANT-114-167-PROPAGATION`, `CM-COVENANT-114-168-PROPAGATION` (propagation literals), plus recommended functional gates `CM-FEATURE-WORKSTREAM-COW-CLONE` / `-NO-MERGE-UNTIL-APPROVED` / `-TRUNK-SYNC-CADENCE` / `-SUBMODULE-CASCADE` (§11.4.167) and `CM-EXPORTED-DOC-VISUALLY-VALIDATED` (§11.4.168). Each is its own gate-code work item with paired §1.1 mutation.
- **(c) Informational / no consumer action:** `bbe84e3` (submodule-internal test triage), `e1bb125` (submodule-internal sibling regen). `b2a7948` is consumed-by-reference infra (action only if HelixCode invokes the constitution export pipeline).
- **(d) Anything that changes how the CURRENT build/test cycle should run:** **NONE.** Neither §11.4.167 nor §11.4.168 alters HelixCode's build/boot/runtime expectations or the current code/test cycle's pass criteria. §11.4.167 governs how FUTURE big work items are organised (branch/clone/queue discipline) — it does not retroactively invalidate the in-flight cycle. §11.4.168 governs doc-export validation — orthogonal to the code/test cycle (it would add a doc-export gate, not change test outcomes).

## 4. Recommendation

**None of this must be done BEFORE the current test cycle.** The advance contains:
- 2 new universal governance mandates requiring a **cascade** into HelixCode's four root carriers (§11.4.157/§11.4.26) + future gate-wiring,
- 2 submodule-internal hygiene commits (no consumer action),
- 1 consumed-by-reference CSS-infra commit.

Per §11.4.121 (no mid-cycle change to avoid racing the in-flight cycle) and §11.4.92 multi-pass, the gitlink-bump + cascade should be a TRACKED FOLLOW-UP cycle the conductor sequences AFTER the current test cycle completes — NOT interleaved now. Flag check: no new rule changes test/boot expectations, so deferring is safe.

The follow-up cycle (conductor-sequenced) would: fetch+investigate (done here) → §11.4.113 ff-only update of the submodule to `e1bb125` (no force) → bump the meta-repo `.gitmodules`/gitlink pointer in the SAME commit as the cascade (§11.4.49 step 7) → add the `11.4.167`/`11.4.168` compact mirrors to `CLAUDE.md`/`AGENTS.md`/`GEMINI.md`/`QWEN.md` → run `scripts/verify-all-constitution-rules.sh` (§11.4.55) → wire the recommended gates as separate gate-code items.

**Submodule HEAD verified untouched at `1d9e3ce` (fetch-only; no checkout).**
