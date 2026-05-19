// Package adapters is an umbrella package collecting consumer-side
// adapters that wire HelixCode to owned-by-us submodules without
// violating CONST-051(B) decoupling. Each adapter lives in its own
// sub-package (e.g. internal/adapters/containers, internal/adapters/
// speckit_debate_adapter). This file (doc.go) exists so the umbrella
// directory is itself a buildable Go package — required by the
// CONST-046 round-239 scaffold (translator.go + translator_test.go)
// which lives at this level.
//
// Sub-packages currently present:
//   - containers/                — adapter to digital.vasic.containers
//   - speckit_debate_adapter/    — adapter wiring HelixSpecifier's
//                                  LLMResponder to DebateOrchestrator
//                                  (round-70 §11.4 anti-bluff fix)
//
// Constitutional anchors:
//   - CONST-046 (no-hardcoded-content): the umbrella ships a
//     Translator seam at internal/adapters/i18n/ so any FUTURE
//     user-facing string added at the umbrella level inherits the
//     standard migration pattern.
//   - CONST-051(B) (decoupling): every adapter under this umbrella
//     keeps the underlying submodule project-not-aware. The umbrella
//     itself imports nothing project-specific.
package adapters
