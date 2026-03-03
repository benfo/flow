// Package git provides helpers for interacting with the local Git repository.
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// IsRepo reports whether the current working directory is inside a Git repository.
func IsRepo() bool {
	return exec.Command("git", "rev-parse", "--git-dir").Run() == nil
}

// RepoRoot returns the absolute path to the root of the current Git repository.
// Returns an error if the working directory is not inside a repo.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not inside a git repository")
	}
	return strings.TrimSpace(string(out)), nil
}

// CreateBranch creates and checks out a new branch with the given name.
// Returns an error if the branch already exists or git is unavailable.
func CreateBranch(name string) error {
	out, err := exec.Command("git", "checkout", "-b", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
