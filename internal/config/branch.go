package config

import (
	"regexp"
	"strings"

	"github.com/ben-fourie/flow-cli/internal/tasks"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// previewTask is the fixed example task used to render live previews in the
// setup wizard, chosen to demonstrate all template placeholders clearly.
var previewTask = tasks.Task{
	ID:     "PROJ-42",
	Title:  "Implement OAuth login",
	Labels: []string{"feature"},
}

// Apply generates a Git branch name from t using this BranchConfig.
// Placeholders in Template are substituted in order: {type}, {id}, {title}.
func (c BranchConfig) Apply(t tasks.Task) string {
	sep := c.Separator
	if sep == "" {
		sep = "-"
	}

	result := c.Template
	result = strings.ReplaceAll(result, "{type}", c.branchType(t.Labels))
	result = strings.ReplaceAll(result, "{id}", slugify(t.ID, sep))
	result = strings.ReplaceAll(result, "{title}", slugify(t.Title, sep))

	if c.MaxLength > 0 && len(result) > c.MaxLength {
		result = result[:c.MaxLength]
		// Trim trailing separator/slash so the name doesn't end mid-word.
		result = strings.TrimRight(result, sep+"/-_")
	}

	return result
}

// Preview returns an example branch name using a fixed example task.
// Useful for showing the user what their configuration will produce.
func (c BranchConfig) Preview() string {
	return c.Apply(previewTask)
}

// branchType resolves the branch type prefix for the given labels using TypeMap.
// Falls back to the "default" key, then to "feature" if no match is found.
func (c BranchConfig) branchType(labels []string) string {
	for _, l := range labels {
		if t, ok := c.TypeMap[strings.ToLower(l)]; ok {
			return t
		}
	}
	if def, ok := c.TypeMap["default"]; ok {
		return def
	}
	return "feature"
}

// slugify converts s to a lowercase, separator-delimited slug safe for use
// in a Git branch name.
func slugify(s, sep string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, sep)
	s = strings.Trim(s, sep)
	return s
}
