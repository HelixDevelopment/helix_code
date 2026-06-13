package workflow

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"dev.helix.code/internal/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGatherProjectContext_PipelineFilterMap proves the dev.helix.pipeline
// FromSlice → Filter(isSourceFile) → Map(toFileInfo) → Collect stage in
// gatherProjectContext produces exactly the expected []FileInfo for a mixed
// source/non-source project tree:
//   - source files (.go/.py/.ts ...) are INCLUDED
//   - non-source files (.md/.txt/.json/.png ...) are EXCLUDED
//   - FileInfo fields (Path, Size, Type, IsEntry) are correct
//
// §11.4.115 RED→GREEN: the §1.1 mutation in the prompt is to break the Filter
// predicate (e.g. invert it / make it always-true). With a broken predicate the
// excluded set (README.md, notes.txt, data.json, logo.png) leaks into ctx.Files
// and/or main.go is dropped — either way the assertions below FAIL. With the
// correct pipeline they PASS. This is a real runtime exercise of the pipeline,
// not a metadata/grep assertion.
func TestGatherProjectContext_PipelineFilterMap(t *testing.T) {
	root := t.TempDir()

	// Mixed tree: source files (must be kept) + non-source files (must be dropped)
	// + a skipped directory (node_modules) whose contents must never appear.
	writeFile(t, root, "main.go", "package main\nfunc main() {}\n")          // source, entry
	writeFile(t, root, "internal/svc.go", "package internal\n")             // source
	writeFile(t, root, "scripts/build.py", "print('x')\n")                  // source
	writeFile(t, root, "web/app.ts", "console.log(1)\n")                    // source
	writeFile(t, root, "README.md", "# docs\n")                            // NON-source
	writeFile(t, root, "notes.txt", "hello\n")                            // NON-source
	writeFile(t, root, "data.json", "{}\n")                               // NON-source
	writeFile(t, root, "assets/logo.png", "\x89PNG\r\n")                  // NON-source
	writeFile(t, root, "node_modules/dep/index.js", "module.exports={}\n") // skipped dir

	proj := &project.Project{Path: root, Type: "go"}
	e := NewExecutor(project.NewManager())

	ctx, err := e.gatherProjectContext(context.Background(), proj)
	require.NoError(t, err)
	require.NotNil(t, ctx)

	// Collect the produced relative paths.
	gotPaths := make([]string, 0, len(ctx.Files))
	byPath := map[string]FileInfo{}
	for _, f := range ctx.Files {
		gotPaths = append(gotPaths, f.Path)
		byPath[f.Path] = f
	}
	sort.Strings(gotPaths)

	wantPaths := []string{
		filepath.Join("internal", "svc.go"),
		"main.go",
		filepath.Join("scripts", "build.py"),
		filepath.Join("web", "app.ts"),
	}
	sort.Strings(wantPaths)

	// Source files INCLUDED, non-source EXCLUDED, skipped dir absent.
	assert.Equal(t, wantPaths, gotPaths, "pipeline must include only source files")

	// Non-source files must NOT leak through the Filter stage.
	for _, excluded := range []string{"README.md", "notes.txt", "data.json", filepath.Join("assets", "logo.png")} {
		_, present := byPath[excluded]
		assert.False(t, present, "non-source file %q must be excluded by the Filter stage", excluded)
	}
	// node_modules content must never be walked.
	for p := range byPath {
		assert.NotContains(t, p, "node_modules", "skipped directory content must not appear")
	}

	// FileInfo field correctness for a representative source file.
	mainGo, ok := byPath["main.go"]
	require.True(t, ok, "main.go must be present")
	assert.Equal(t, ".go", mainGo.Type, "Type must be the file extension")
	assert.True(t, mainGo.IsEntry, "main.go in a go project must be flagged IsEntry")
	wantSize := fileSize(t, filepath.Join(root, "main.go"))
	assert.Equal(t, wantSize, mainGo.Size, "Size must match the real on-disk byte count")

	// A non-entry source file must NOT be flagged as entry.
	svc, ok := byPath[filepath.Join("internal", "svc.go")]
	require.True(t, ok)
	assert.False(t, svc.IsEntry, "internal/svc.go must not be an entry point")
	assert.Equal(t, ".go", svc.Type)
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

func fileSize(t *testing.T, full string) int64 {
	t.Helper()
	info, err := os.Stat(full)
	require.NoError(t, err)
	return info.Size()
}
