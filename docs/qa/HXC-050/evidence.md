# HXC-050 — event_bus NATS env-gated skips lacked SKIP-OK markers (§11.4.98 no-silent-skips)
Found by owned-submodule health sweep (D-3). pkg/nats/integration_test.go:23,120 env-gated t.Skip (legitimate — runs
vs real NATS when NATS_URL set) lacked literal SKIP-OK markers the no-silent-skips gate requires. FIX: added
"SKIP-OK: #HXC-050 ..." to both. build OK; go test ./pkg/nats/ → ok (clean skip). event_bus commit 1cae683.
