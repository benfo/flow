// Package config manages global and per-repository configuration for flow-cli.
// Config is stored as YAML and loaded in layers: defaults → global → repo,
// where each layer overrides only the fields it explicitly sets.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ── Types ─────────────────────────────────────────────────────────────────────

// Config is the top-level application configuration.
type Config struct {
	Branch    BranchConfig    `yaml:"branch"`
	Providers ProvidersConfig `yaml:"providers,omitempty"`
}

// ProvidersConfig lists which task providers are active and holds their settings.
// The API token for each provider is stored separately in the OS keychain,
// never in this file.
type ProvidersConfig struct {
	// Active is the ordered list of provider names to load.
	// Built-in values: "mock". Add "jira" after running 'flow auth jira'.
	Active []string    `yaml:"active,omitempty"`
	Jira   *JiraConfig `yaml:"jira,omitempty"`
}

// JiraConfig holds the connection settings for a Jira Cloud instance.
type JiraConfig struct {
	// BaseURL is the root URL of the Jira instance, e.g. https://yourco.atlassian.net
	BaseURL string `yaml:"base_url"`
	// Email is the Atlassian account email used for API authentication.
	Email string `yaml:"email"`
	// Projects optionally filters results to specific project keys.
	// Leave empty to show all issues assigned to the current user.
	Projects []string `yaml:"projects,omitempty"`
}

// BranchConfig controls how Git branch names are generated from tasks.
type BranchConfig struct {
	// Template uses {type}, {id}, and {title} as placeholders.
	// Example: "{type}/{id}-{title}" → "feature/FLOW-001-implement-oauth"
	Template string `yaml:"template"`

	// Separator is placed between words when slugifying {id} and {title}.
	// Common values: "-" (default) or "_".
	Separator string `yaml:"separator"`

	// MaxLength caps the final branch name length. 0 means no limit.
	MaxLength int `yaml:"max_length,omitempty"`

	// TypeMap maps task labels to branch type prefixes.
	// A "default" key is used when no label matches.
	TypeMap map[string]string `yaml:"type_map,omitempty"`
}

// BranchPreset is a named, selectable branch naming template shown in the
// setup wizard.
type BranchPreset struct {
	Name        string // human-readable display name
	Template    string // template string with placeholders
	PatternDesc string // short description of the pattern
}

// Presets is the ordered list of branch naming presets offered during setup.
var Presets = []BranchPreset{
	{"type/ID-title", "{type}/{id}-{title}", "e.g. feature/FLOW-001-implement-oauth"},
	{"type/title", "{type}/{title}", "e.g. feature/implement-oauth"},
	{"ID-title", "{id}-{title}", "e.g. FLOW-001-implement-oauth"},
	{"ID/title", "{id}/{title}", "e.g. FLOW-001/implement-oauth"},
	{"type/ID/title", "{type}/{id}/{title}", "e.g. feature/FLOW-001/implement-oauth"},
	{"title", "{title}", "e.g. implement-oauth"},
}

// DefaultTypeMap is the built-in label → branch-type mapping.
var DefaultTypeMap = map[string]string{
	"bug":     "fix",
	"fix":     "fix",
	"hotfix":  "hotfix",
	"default": "feature",
}

// Default is the configuration used when no config file is present.
var Default = Config{
	Branch: BranchConfig{
		Template:  "{type}/{id}-{title}",
		Separator: "-",
		MaxLength: 0,
		TypeMap:   DefaultTypeMap,
	},
	Providers: ProvidersConfig{
		Active: []string{"mock"},
	},
}

// ── Paths ─────────────────────────────────────────────────────────────────────

// GlobalPath returns the path to the global config file,
// using the OS-appropriate user config directory.
func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locating user config directory: %w", err)
	}
	return filepath.Join(dir, "flow-cli", "config.yaml"), nil
}

// RepoPath returns the path to the per-repository config file.
func RepoPath(repoRoot string) string {
	return filepath.Join(repoRoot, ".flow.yaml")
}

// GlobalExists reports whether the global config file is present on disk.
func GlobalExists() bool {
	p, err := GlobalPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// ── Load ──────────────────────────────────────────────────────────────────────

// Load builds the effective config by merging layers:
//
//	Default → global config → per-repo config (if repoRoot is non-empty)
//
// A missing config file is silently skipped; only parse errors are returned.
func Load(repoRoot string) (Config, error) {
	cfg := Default

	globalPath, err := GlobalPath()
	if err == nil {
		if global, err := readFile(globalPath); err == nil {
			cfg = merge(cfg, global)
		} else if !errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("reading global config: %w", err)
		}
	}

	if repoRoot != "" {
		if repo, err := readFile(RepoPath(repoRoot)); err == nil {
			cfg = merge(cfg, repo)
		} else if !errors.Is(err, os.ErrNotExist) {
			return cfg, fmt.Errorf("reading repo config: %w", err)
		}
	}

	return cfg, nil
}

// ── Save ──────────────────────────────────────────────────────────────────────

// SaveGlobal writes cfg to the global config path, creating parent dirs as needed.
func SaveGlobal(cfg Config) (string, error) {
	p, err := GlobalPath()
	if err != nil {
		return "", err
	}
	return p, writeFile(p, cfg)
}

// SaveRepo writes cfg to <repoRoot>/.flow.yaml.
func SaveRepo(cfg Config, repoRoot string) (string, error) {
	p := RepoPath(repoRoot)
	return p, writeFile(p, cfg)
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func readFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cfg, nil
}

func writeFile(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("serialising config: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// merge returns base with any non-zero fields from override applied on top.
// This implements the "sparse override" behaviour — only fields explicitly
// set in a config file affect the result.
func merge(base, override Config) Config {
	result := base
	b := override.Branch
	if b.Template != "" {
		result.Branch.Template = b.Template
	}
	if b.Separator != "" {
		result.Branch.Separator = b.Separator
	}
	if b.MaxLength != 0 {
		result.Branch.MaxLength = b.MaxLength
	}
	if b.TypeMap != nil {
		result.Branch.TypeMap = b.TypeMap
	}
	p := override.Providers
	if len(p.Active) > 0 {
		result.Providers.Active = p.Active
	}
	if p.Jira != nil {
		if result.Providers.Jira == nil {
			result.Providers.Jira = p.Jira
		} else {
			// Deep-merge so per-repo config can override individual fields
			// (e.g. just Projects) without wiping credentials.
			merged := *result.Providers.Jira
			if p.Jira.BaseURL != "" {
				merged.BaseURL = p.Jira.BaseURL
			}
			if p.Jira.Email != "" {
				merged.Email = p.Jira.Email
			}
			if len(p.Jira.Projects) > 0 {
				merged.Projects = p.Jira.Projects
			}
			result.Providers.Jira = &merged
		}
	}
	return result
}
