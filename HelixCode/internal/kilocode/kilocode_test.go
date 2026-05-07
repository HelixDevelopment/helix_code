package kilocode

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraph_BuildAndQuery(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
func helper() string { return "ok" }
func main() { println(helper()) }
`), 0644)

	cg, err := BuildCallGraph(dir)
	require.NoError(t, err)

	assert.Greater(t, cg.NodeCount(), 0)
	assert.Greater(t, cg.EdgeCount(), 0)
	t.Logf("nodes=%d edges=%d", cg.NodeCount(), cg.EdgeCount())
}

func TestRenameEngine(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
func oldHelper() string { return "ok" }
func main() { oldHelper() }
`), 0644)

	engine := NewRenameEngine(dir)
	result, err := engine.Rename(context.Background(), "oldHelper", "newHelper")
	require.NoError(t, err)

	assert.Equal(t, "newHelper", result.NewName)
	assert.Greater(t, result.Occurrences, 0)
	assert.Equal(t, "oldHelper", result.Symbol.Name)

	src, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	assert.NotContains(t, string(src), "oldHelper")
	assert.Contains(t, string(src), "newHelper")
}

func TestRenameEngine_NotFound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	engine := NewRenameEngine(dir)
	_, err := engine.Rename(context.Background(), "nonexistent", "other")
	assert.ErrorIs(t, err, ErrSymbolNotFound)
}

func TestImpactAnalyzer(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main
func helper() string { return "ok" }
func main() { println(helper()) }
func caller() { helper() }
`), 0644)

	ia, err := NewImpactAnalyzer(dir)
	require.NoError(t, err)

	result, err := ia.Analyze("main.helper")
	require.NoError(t, err)

	t.Logf("impact result: callers=%d callees=%d blast=%d risk=%.2f",
		len(result.Callers), len(result.Callees), result.BlastRadius, result.RiskScore)

	if len(result.Callers) > 0 {
		assert.Greater(t, result.BlastRadius, 0)
		assert.Greater(t, result.RiskScore, 0.0)
	}
}

func TestImpactAnalyzer_UnknownSymbol(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	ia, _ := NewImpactAnalyzer(dir)
	result, err := ia.Analyze("unknown.func")
	require.NoError(t, err)
	assert.Equal(t, 0, result.BlastRadius)
}

func TestRefactorer_ExtractMethod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")

	os.WriteFile(path, []byte(`package main
func main() {
	println("hello")
	println("world")
}
`), 0644)

	r := NewRefactorer(dir)
	err := r.ExtractMethod(path, "sayHello", 3, 4)
	require.NoError(t, err)

	src, _ := os.ReadFile(path)
	assert.Contains(t, string(src), "func sayHello()")
	assert.Contains(t, string(src), "sayHello()")
}

func TestRefactorer_InlineCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "main.go")

	os.WriteFile(path, []byte(`package main
func helper() { println("help") }
func main() { helper() }
`), 0644)

	r := NewRefactorer(dir)
	err := r.InlineCall(path, "helper")
	require.NoError(t, err)

	src, _ := os.ReadFile(path)
	assert.NotContains(t, string(src), "helper()")
	assert.Contains(t, string(src), "/* inlined")
}

func TestSentinelErrors(t *testing.T) {
	assert.Error(t, ErrSymbolNotFound)
	assert.Error(t, ErrRenameConflict)
	assert.Error(t, ErrRefactorNotSafe)
	assert.Error(t, ErrNoCallGraph)
}
