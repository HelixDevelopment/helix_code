package filesystem

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFileReader_ListDir is the RED→GREEN guard for the graceful-directory
// behaviour of fs_read. Before the fix, the reader had no way to list a
// directory — Read() returned an ErrorIsDirectory error and the agent
// tool-loop surfaced a NEGATIVE "is_directory: path is a directory" result
// to the model. ListDir returns a readable *DirListing instead.
func TestFileReader_ListDir(t *testing.T) {
	tmpDir := t.TempDir()

	// A few files + a subdirectory.
	if err := os.WriteFile(filepath.Join(tmpDir, "alpha.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write alpha.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "beta.go"), []byte("package x"), 0o644); err != nil {
		t.Fatalf("write beta.go: %v", err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "subdir"), 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	config := DefaultConfig()
	config.WorkspaceRoot = tmpDir
	fs, err := NewFileSystemTools(config)
	if err != nil {
		t.Fatalf("NewFileSystemTools: %v", err)
	}

	listing, err := fs.Reader().ListDir(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ListDir returned a NEGATIVE error (must be graceful): %v", err)
	}
	if listing == nil {
		t.Fatal("ListDir returned nil listing")
	}

	got := listing.String()
	for _, want := range []string{"alpha.txt", "beta.go", "subdir"} {
		if !strings.Contains(got, want) {
			t.Fatalf("listing must contain entry %q; got:\n%s", want, got)
		}
	}
	// Directory marker for the subdir.
	if !strings.Contains(got, "subdir/") {
		t.Fatalf("listing must mark directories with a trailing slash; got:\n%s", got)
	}
	// Header naming the path + entry count.
	if !strings.Contains(got, tmpDir) {
		t.Fatalf("listing header must name the directory path; got:\n%s", got)
	}
	if !strings.Contains(got, "3 entries") {
		t.Fatalf("listing header must report the entry count; got:\n%s", got)
	}
}

// TestFileReader_ListDir_Truncation verifies the bounded-listing cap so a huge
// directory cannot flood the model context.
func TestFileReader_ListDir_Truncation(t *testing.T) {
	tmpDir := t.TempDir()
	for i := 0; i < maxDirListingEntries+50; i++ {
		name := filepath.Join(tmpDir, "f"+strings.Repeat("0", 3)+itoa(i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatalf("write entry %d: %v", i, err)
		}
	}

	config := DefaultConfig()
	config.WorkspaceRoot = tmpDir
	fs, err := NewFileSystemTools(config)
	if err != nil {
		t.Fatalf("NewFileSystemTools: %v", err)
	}

	listing, err := fs.Reader().ListDir(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ListDir error: %v", err)
	}
	if listing.Truncated != true {
		t.Fatalf("expected Truncated=true for an over-cap directory")
	}
	if !strings.Contains(listing.String(), "truncated") {
		t.Fatalf("over-cap listing must note truncation; got:\n%s", listing.String())
	}
	if len(listing.Entries) > maxDirListingEntries {
		t.Fatalf("listing entries (%d) must be capped at %d", len(listing.Entries), maxDirListingEntries)
	}
}

// itoa is a tiny strconv.Itoa avoiding an import in this test file.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
