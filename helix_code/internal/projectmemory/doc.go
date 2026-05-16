// Package projectmemory implements the codex-style project-memory subsystem
// for the HelixCode CLI agent. A project-root Markdown file (helixcode.md /
// codex.md / AGENTS.md, first-found wins) plus a per-user overlay at
// $XDG_CONFIG_HOME/helixcode/memory.md is loaded into every LLM call's
// system prompt. The blob is hot-reloaded mid-session via fsnotify when the
// underlying file changes.
//
// Components:
//
//   - Memory          — immutable value type holding the loaded content +
//                        paths + truncation flags + load timestamp.
//   - MemoryLoader    — parent-walk discovery (cwd → parent → … → git root)
//                        + user overlay reader. Missing files are NOT errors.
//   - MemoryRegistry  — atomic.Pointer[Memory] wrapper. Snapshot is lock-free;
//                        Reload is mu-serialised. Implements MemorySnapshotter.
//   - MemoryWatcher   — fsnotify wrapper with 200 ms debounce. Watches the
//                        parent directories of the project + user paths so
//                        atomic-write editors (vim, emacs) survive renames.
//                        Graceful degrade when fsnotify is unavailable.
//
// Constitutional anchors:
//   - CONST-035 (anti-bluff): every PASS in F24 carries positive runtime
//     evidence — real tempdirs + real fsnotify + real file I/O + sentinel
//     byte equality + truncation flag verification.
//   - CONST-042 (no secret leak): memory contents NEVER logged at INFO. The
//     loader logs only the path + byte counts.
//
// Per Feature 24 (codex project memory port), P2-F24-T02..T08.
package projectmemory
