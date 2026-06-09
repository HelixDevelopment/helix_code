# HXC-043 — auth Login nil-DB panic → HTTP 500 (real product bug, found by HXC-041 live run)
Case HXC-AUTH-003 (login, expect 401) got HTTP 500 empty body. ROOT CAUSE (FACT from server stacktrace):
with db=nil (server's graceful "continuing without database" path), helix_code/internal/auth/auth.go:156
(*AuthService).Login calls s.db.GetUserByUsername on nil s.db → nil-pointer panic → Gin Recovery → HTTP 500.
Reached via internal/server/handlers.go:162. Raw curl confirmed HTTP/1.1 500. FIX: guard nil s.db in Login +
sibling db-touching auth paths → clean 401/503. RED test = helixcode-auth.yaml HXC-AUTH-003 via `helixqa http`.
[Fix in progress under subagent — RED→GREEN evidence to be appended.]
