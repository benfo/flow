package git

import (
	"regexp"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/tasks"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateBranchName produces a branch name from a task following the convention:
//
//	<type>/<id>-<slugified-title>
//
// The type prefix is derived from the task's labels:
//   - any label equal to "bug" or "fix" → "fix"
//   - otherwise → "feature"
//
// Example: feature/FLOW-001-implement-oauth-flow-for-jira
func GenerateBranchName(t tasks.Task) string {
	prefix := branchPrefix(t.Labels)
	slug := slugify(t.ID + "-" + t.Title)
	return prefix + "/" + slug
}

// branchPrefix returns "fix" when the task has a bug/fix label, else "feature".
func branchPrefix(labels []string) string {
	for _, l := range labels {
		switch strings.ToLower(l) {
		case "bug", "fix":
			return "fix"
		}
	}
	return "feature"
}

// slugify converts a string to a lowercase, hyphen-separated slug safe for
// use in a Git branch name.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}
