// Package git provides helpers for interacting with the local Git repository.
package git

import (
	"fmt"
	"net/url"
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

// CurrentBranch returns the name of the currently checked-out branch.
// Returns an empty string (no error) when not in a repo or in detached HEAD state.
func CurrentBranch() string {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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

// CheckoutBranch switches to an existing branch with the given name.
func CheckoutBranch(name string) error {
	out, err := exec.Command("git", "checkout", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// AllLocalBranches returns the names of all local branches.
func AllLocalBranches() ([]string, error) {
	out, err := exec.Command("git", "branch", "--list", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, err
	}
	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// FindLocalBranch returns the name of the first local branch whose name
// contains taskID (case-insensitive). Returns "" if none found.
func FindLocalBranch(taskID string) string {
	out, err := exec.Command("git", "branch", "--list", "--format=%(refname:short)").Output()
	if err != nil {
		return ""
	}
	upper := strings.ToUpper(taskID)
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(strings.ToUpper(line), upper) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

// BranchSummary holds the last-commit metadata for a branch.
type BranchSummary struct {
	Hash    string // short hash, e.g. "a1b2c3d"
	Subject string // commit subject line
	When    string // human-readable relative time, e.g. "2 hours ago"
}

// LastCommit returns the most recent commit summary on the given branch.
// Returns an empty BranchSummary on error.
func LastCommit(branch string) BranchSummary {
	out, err := exec.Command(
		"git", "log", branch, "-1",
		"--format=%h\x1f%s\x1f%cr",
	).Output()
	if err != nil {
		return BranchSummary{}
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\x1f", 3)
	if len(parts) != 3 {
		return BranchSummary{}
	}
	return BranchSummary{Hash: parts[0], Subject: parts[1], When: parts[2]}
}


func RemoteURL(remote string) (string, error) {
	out, err := exec.Command("git", "remote", "get-url", remote).Output()
	if err != nil {
		return "", fmt.Errorf("could not get URL for remote %q", remote)
	}
	return strings.TrimSpace(string(out)), nil
}

// PRCreateURL builds the browser URL to open a new pull / merge request for
// branch on the repository identified by remoteURL.
// Supports GitHub, GitLab, and Bitbucket (both HTTPS and SSH remotes).
func PRCreateURL(remoteURL, branch string) (string, error) {
	host, repo, err := parseRemote(remoteURL)
	if err != nil {
		return "", err
	}
	escaped := url.PathEscape(branch)
	switch {
	case strings.Contains(host, "github"):
		return fmt.Sprintf("https://%s/%s/compare/%s?expand=1", host, repo, escaped), nil
	case strings.Contains(host, "gitlab"):
		return fmt.Sprintf("https://%s/%s/-/merge_requests/new?merge_request[source_branch]=%s", host, repo, escaped), nil
	case strings.Contains(host, "bitbucket"):
		return fmt.Sprintf("https://%s/%s/pull-requests/new?source=%s", host, repo, escaped), nil
	default:
		return "", fmt.Errorf("unsupported git host %q — open your browser manually", host)
	}
}

// parseRemote extracts the host and owner/repo path from an HTTPS or SSH remote URL.
func parseRemote(raw string) (host, repo string, err error) {
	raw = strings.TrimSpace(raw)
	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(raw, "git@") {
		raw = strings.TrimPrefix(raw, "git@")
		parts := strings.SplitN(raw, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH remote URL")
		}
		return parts[0], strings.TrimSuffix(parts[1], ".git"), nil
	}
	// HTTPS format: https://github.com/owner/repo.git
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", fmt.Errorf("invalid remote URL: %w", err)
	}
	return u.Host, strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git"), nil
}
