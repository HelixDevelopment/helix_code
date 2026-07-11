# S4 docs task — evidence

## Task
Create two missing docs under `docs/guides/`:
- `docs/guides/MULTITRACK_WORKTREE_RUNBOOK.md` (F-DOC-RUNBOOK — referenced by
  `config/multitrack/the-factory.yaml` + `constitution/scripts/multitrack/multitrack.sh`
  + `multitrack_bootstrap.sh`, but missing from the checkout).
- `docs/guides/GUI_BUILD_ENV.md` (F-BUILD-GUI — plain `go build ./...` fails on
  X11/GL system libs for the Fyne/go-gl-dependent packages; sanctioned path is
  `make verify-compile` = `go build -tags=nogui ./...`).

## Scripts/config cross-checked for MULTITRACK_WORKTREE_RUNBOOK.md
Full file reads (not just grep) of:
- constitution/scripts/multitrack/multitrack.sh (full, 234 lines)
- constitution/scripts/multitrack/multitrack-up (full, 242 lines)
- constitution/scripts/multitrack/multitrack_bootstrap.sh (header block, lines 1-130)
- constitution/scripts/multitrack/multitrack_claim.sh (header + top of body, lines 1-120)
- constitution/scripts/multitrack/multitrack_work_binding.sh (header + arg parsing, lines 1-120)
- constitution/scripts/multitrack/multitrack_device_lock.sh (header + config load start, lines 1-120)
- constitution/scripts/multitrack/multitrack_supervisor.sh (header block, lines 1-120)
- constitution/scripts/multitrack/multitrack_resolve_worktree.sh (header + resolution logic, lines 1-140)
- constitution/scripts/multitrack/multitrack_host_budget.sh (full header block, lines 1-120)
- config/multitrack/the-factory.yaml (full, 107 lines)

Confirmed via `ls`:
- docs/guides/MULTITRACK_ACTIVATION.md does NOT exist in this checkout (a companion
  doc multitrack.sh's own header/help references) — flagged as a known gap in the
  runbook rather than fabricated or silently ignored.
- No docs/scripts/multitrack*.md companion files exist either (pre-existing gap the
  supervisor/host-budget scripts' own headers already note).

Runbook content is a factual description of what these scripts do (topology, status/up
exit codes incl. 20=OPERATOR-BLOCKED, the four claim/work-binding/device-lock/supervisor
sub-systems, the host-budget guard, and the §11.4.133 never-mount/format boundary) — no
capability was invented that isn't in the read source.

## Verification steps for GUI_BUILD_ENV.md (commands actually run this session)

```
$ cd helix_code && go build ./... 2>&1 | head -60
# github.com/go-gl/gl/v2.1/gl
# [pkg-config --cflags  -- gl gl]
Package gl was not found in the pkg-config search path.
...
No package 'gl' found
# github.com/go-gl/glfw/v3.3/glfw
In file included from ./glfw/src/internal.h:188, ...
./glfw/src/x11_platform.h:33:10: fatal error: X11/Xlib.h: No such file or directory
   33 | #include <X11/Xlib.h>
compilation terminated.
```
(real command run, real output — reproduced verbatim in the doc)

```
$ cd helix_code && make verify-compile
🔍 Verifying code compilation (nogui — no X11 system libs required)...
✅ All packages compile successfully
```
(real command run and GREEN — `make verify-compile` was actually executed and passed
on this host; a full `make desktop` / real-GUI build was NOT run and the doc says so
explicitly — no claim of a full-GUI-build PASS is made anywhere in GUI_BUILD_ENV.md)

Host facts checked live (not assumed):
- `pkg-config --exists gl` → fails (not found)
- `/usr/include/X11/Xlib.h` → does not exist
- `cat /etc/os-release` → ALT Workstation 11.1 (Prometheus), altlinux, apt-rpm-based
- `rpm -q pkg-config` → pkg-config-0.29.2-alt3.x86_64 (already installed)
- `apt-cache search --names-only libGL` → confirms package `libGL-devel` exists
  ("Development files for Mesa Library")
- `apt-cache search --names-only libX11` → confirms package `libX11-devel` exists
  ("X11 Libraries and Header Files")
- `apt-cache search --names-only libXrandr/libXcursor/libXinerama/libXi-devel/libXxf86vm`
  → confirms all five sibling `-devel` package names exist in the ALT repo index

The doc explicitly marks the five sibling X11-extension packages (Xrandr/Xcursor/
Xinerama/Xi/Xxf86vm) as included on the strength of GLFW's documented standard
Linux dependency set, NOT individually re-proven by triggering their specific
missing-header compile errors (only libGL-devel's `gl.pc` and libX11-devel's
`Xlib.h` were the two errors actually observed) — marked "UNCONFIRMED" in-doc per
the anti-bluff/no-guessing discipline (do not claim more verification than was done).

Packages-affected claim cross-checked via:
```
$ grep -rln "fyne.io\|go-gl" tests/ui
tests/ui/desktop_widget_test.go
tests/ui/ui_harness.go
```
and Makefile grep confirming the `nogui` build-tag targets
(verify-compile / desktop-nogui / desktop / desktop-linux etc.) plus the
existing `*_nogui.go` files already present in applications/{desktop,aurora_os,
harmony_os} that implement the accommodation.

## No git operations performed. No edits outside docs/. No --force used.
