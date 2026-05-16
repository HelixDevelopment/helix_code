package session

import (
	"os"
	"os/exec"
	"strings"
)

// ComputeProjectIdentity returns the Git toplevel for the cwd, or the cwd
// itself when not in a Git repo. Surfaces error only if both fail.
func ComputeProjectIdentity() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	return os.Getwd()
}
