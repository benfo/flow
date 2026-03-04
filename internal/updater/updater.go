// Package updater checks for newer flow releases on GitHub and caches the result.
package updater

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	cacheTTL = 24 * time.Hour
	apiURL   = "https://api.github.com/repos/benfo/flow/releases/latest"
)

type cacheEntry struct {
	LatestVersion string    `json:"latest_version"`
	CheckedAt     time.Time `json:"checked_at"`
}

// Check returns the latest release tag (e.g. "v0.2.0") when it is newer than
// currentVersion, or an empty string when the installation is up to date.
// Results are cached for 24 hours to avoid hitting the API on every startup.
// Any network or parsing error is silently ignored and returns ("", nil).
func Check(currentVersion string) (latestVersion string, err error) {
	if currentVersion == "dev" || currentVersion == "" {
		return "", nil // dev builds never prompt to update
	}
	latest, err := resolveLatest()
	if err != nil || latest == "" {
		return "", err
	}
	if isNewer(latest, currentVersion) {
		return latest, nil
	}
	return "", nil
}

// resolveLatest returns the latest version from cache if fresh, else hits GitHub.
func resolveLatest() (string, error) {
	if c, ok := loadCache(); ok {
		return c.LatestVersion, nil
	}
	tag, err := fetchLatestTag()
	if err != nil {
		return "", err
	}
	saveCache(cacheEntry{LatestVersion: tag, CheckedAt: time.Now()})
	return tag, nil
}

func loadCache() (cacheEntry, bool) {
	data, err := os.ReadFile(cachePath())
	if err != nil {
		return cacheEntry{}, false
	}
	var c cacheEntry
	if err := json.Unmarshal(data, &c); err != nil {
		return cacheEntry{}, false
	}
	if time.Since(c.CheckedAt) > cacheTTL {
		return cacheEntry{}, false
	}
	return c, true
}

func saveCache(c cacheEntry) {
	p := cachePath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	if data, err := json.Marshal(c); err == nil {
		_ = os.WriteFile(p, data, 0o644)
	}
}

func cachePath() string {
	dir, _ := os.UserConfigDir()
	return filepath.Join(dir, "flow-cli", "update-check.json")
}

type release struct {
	TagName string `json:"tag_name"`
}

func fetchLatestTag() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "flow-cli/updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github API: %s", resp.Status)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// isNewer returns true when latest is a higher semver than current.
func isNewer(latest, current string) bool {
	return cmpSemver(stripV(latest), stripV(current)) > 0
}

// cmpSemver compares two semver strings without a leading "v".
// Returns 1 if a > b, -1 if a < b, 0 if equal.
// Pre-release suffixes (e.g. "-beta.4") sort below the equivalent release.
func cmpSemver(a, b string) int {
	aCore, aPre, _ := strings.Cut(a, "-")
	bCore, bPre, _ := strings.Cut(b, "-")

	aParts := strings.SplitN(aCore, ".", 3)
	bParts := strings.SplitN(bCore, ".", 3)

	for i := 0; i < 3; i++ {
		av, bv := 0, 0
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}

	// Same core version: release (no pre-release) beats pre-release.
	switch {
	case aPre == "" && bPre != "":
		return 1 // a is release, b is pre-release → a > b
	case aPre != "" && bPre == "":
		return -1 // a is pre-release, b is release → a < b
	case aPre == bPre:
		return 0
	default:
		if aPre > bPre {
			return 1
		}
		return -1
	}
}

func stripV(v string) string {
	return strings.TrimPrefix(v, "v")
}
