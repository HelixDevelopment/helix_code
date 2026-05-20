package database

import (
	"testing"
)

// BenchmarkResolvePoolConfig_CLI / _Server measure the (negligible) cost of
// profile resolution and, more importantly, let the suite record the
// resolved connection counts as part of the P4-T03 evidence: the CLI
// profile resolves to a strictly smaller pool, which is the speed win
// (fewer idle connections opened at CLI startup — R1 B17).
func BenchmarkResolvePoolConfig_CLI(b *testing.B) {
	cfg := Config{Profile: PoolProfileCLI}
	var rp resolvedPool
	for i := 0; i < b.N; i++ {
		rp = resolvePoolConfig(cfg)
	}
	b.ReportMetric(float64(rp.MaxConns), "max_conns")
	b.ReportMetric(float64(rp.MinConns), "min_idle_conns")
}

func BenchmarkResolvePoolConfig_Server(b *testing.B) {
	cfg := Config{Profile: PoolProfileServer}
	var rp resolvedPool
	for i := 0; i < b.N; i++ {
		rp = resolvePoolConfig(cfg)
	}
	b.ReportMetric(float64(rp.MaxConns), "max_conns")
	b.ReportMetric(float64(rp.MinConns), "min_idle_conns")
}

// BenchmarkStartupIdleConns_CLIvsServer reports the eager-idle-connection
// count each profile would open at process startup. MinConns is the number
// of connections pgxpool opens immediately when the pool is constructed —
// the CLI profile's value of 0 vs the server's 5 is the measured before/
// after of the P4-T03 "fewer idle connections at startup" claim.
func BenchmarkStartupIdleConns_CLIvsServer(b *testing.B) {
	server := resolvePoolConfig(Config{Profile: PoolProfileServer})
	cli := resolvePoolConfig(Config{Profile: PoolProfileCLI})
	for i := 0; i < b.N; i++ {
		_ = resolvePoolConfig(Config{Profile: PoolProfileCLI})
	}
	b.ReportMetric(float64(server.MinConns), "server_startup_idle_conns")
	b.ReportMetric(float64(cli.MinConns), "cli_startup_idle_conns")
	b.ReportMetric(float64(server.MinConns-cli.MinConns), "idle_conns_saved")
}
