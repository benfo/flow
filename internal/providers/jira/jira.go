// Package jira implements a tasks.Provider backed by the Jira Cloud REST API v3.
// Authentication uses HTTP Basic Auth with an Atlassian API token; the token
// is never stored in config files — it is read from the OS keychain.
package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ben-fourie/flow-cli/internal/config"
	"github.com/ben-fourie/flow-cli/internal/keychain"
	"github.com/ben-fourie/flow-cli/internal/tasks"
)

const (
	keychainProvider = "jira"
	maxResults       = 50
)

// ── Provider ──────────────────────────────────────────────────────────────────

// Provider fetches Jira issues assigned to the authenticated user.
type Provider struct {
	client *client
	cfg    *config.JiraConfig
}

// New creates a Provider using the token from the OS keychain.
// Returns (nil, false, nil) if no Jira config is present so the registry
// can skip it silently.
func New(cfg config.Config, kr *keychain.Keychain) (tasks.Provider, bool, error) {
	if cfg.Providers.Jira == nil {
		return nil, false, nil
	}
	jc := cfg.Providers.Jira

	token, err := kr.Get(keychainProvider, jc.Email)
	if err != nil {
		return nil, false, fmt.Errorf(
			"no Jira token in keychain for %s — run 'flow auth jira' to set up credentials",
			jc.Email,
		)
	}

	return &Provider{
		client: newClient(jc.BaseURL, jc.Email, token),
		cfg:    jc,
	}, true, nil
}

// Name satisfies tasks.Provider.
func (p *Provider) Name() string { return "Jira" }

// GetTasks fetches open issues assigned to the current user via JQL.
func (p *Provider) GetTasks() ([]tasks.Task, error) {
	jql := p.buildJQL()

	resp, err := p.client.search(jql, maxResults)
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}

	return mapIssues(resp.Issues, p.cfg.BaseURL), nil
}

// Update satisfies tasks.Updater. It writes the new Title and Description
// back to Jira using a PUT request. The description is converted from plain
// text to Atlassian Document Format (ADF) before sending.
func (p *Provider) Update(task tasks.Task) error {
	return p.client.updateIssue(task.ID, task.Title, task.Description)
}


func (p *Provider) buildJQL() string {
	base := "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
	if len(p.cfg.Projects) == 0 {
		return base
	}
	quoted := make([]string, len(p.cfg.Projects))
	for i, proj := range p.cfg.Projects {
		quoted[i] = `"` + proj + `"`
	}
	return "project IN (" + strings.Join(quoted, ", ") + ") AND " + base
}

// ── HTTP client ───────────────────────────────────────────────────────────────

type client struct {
	baseURL string
	auth    string // base64(email:token)
	http    *http.Client
}

// NewTestClient creates an HTTP client that can be used to validate credentials
// without constructing a full Provider (e.g. from the auth wizard).
func NewTestClient(baseURL, email, token string) *client {
	return newClient(baseURL, email, token)
}

func newClient(baseURL, email, token string) *client {
	creds := base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		auth:    "Basic " + creds,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *client) get(path string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	return c.http.Do(req)
}

func (c *client) put(path string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPut, c.baseURL+path, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

// updateIssue writes a new summary and description to the given issue key.
func (c *client) updateIssue(key, summary, description string) error {
	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"summary":     summary,
			"description": plainTextToADF(description),
		},
	}
	resp, err := c.put("/rest/api/3/issue/"+key, body)
	if err != nil {
		return fmt.Errorf("jira update %s: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira update %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// plainTextToADF converts a plain-text string to Atlassian Document Format.
// Blank-line-separated blocks become paragraphs; single newlines within a
// block become hardBreak nodes so formatting is preserved.
func plainTextToADF(text string) map[string]interface{} {
	blocks := strings.Split(strings.TrimSpace(text), "\n\n")
	content := make([]interface{}, 0, len(blocks))

	for _, block := range blocks {
		if block == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		nodes := make([]interface{}, 0, len(lines)*2)
		for i, line := range lines {
			nodes = append(nodes, map[string]interface{}{"type": "text", "text": line})
			if i < len(lines)-1 {
				nodes = append(nodes, map[string]interface{}{"type": "hardBreak"})
			}
		}
		content = append(content, map[string]interface{}{
			"type":    "paragraph",
			"content": nodes,
		})
	}

	if len(content) == 0 {
		content = []interface{}{
			map[string]interface{}{"type": "paragraph", "content": []interface{}{}},
		}
	}

	return map[string]interface{}{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
}

// MyselfResponse is the subset of /rest/api/3/myself we care about.
type MyselfResponse struct {
	DisplayName  string `json:"displayName"`
	EmailAddress string `json:"emailAddress"`
}

// Myself calls /rest/api/3/myself to verify credentials.
func (c *client) Myself() (*MyselfResponse, error) {
	resp, err := c.get("/rest/api/3/myself")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira returned HTTP %d — check your credentials", resp.StatusCode)
	}

	var m MyselfResponse
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("decoding /myself response: %w", err)
	}
	return &m, nil
}

// ── Search ────────────────────────────────────────────────────────────────────

type searchResponse struct {
	Issues []issue `json:"issues"`
}

type issue struct {
	Key    string      `json:"key"`
	Fields issueFields `json:"fields"`
}

type issueFields struct {
	Summary     string          `json:"summary"`
	Description *adfNode        `json:"description"`
	Status      issueStatus     `json:"status"`
	Priority    *issuePriority  `json:"priority"`
	Assignee    *issueUser      `json:"assignee"`
	Labels      []string        `json:"labels"`
	Project     issueProject    `json:"project"`
}

type issueStatus struct {
	StatusCategory statusCategory `json:"statusCategory"`
}

type statusCategory struct {
	Key string `json:"key"` // "new", "indeterminate", "done"
}

type issuePriority struct {
	Name string `json:"name"` // "Highest", "High", "Medium", "Low", "Lowest"
}

type issueUser struct {
	DisplayName string `json:"displayName"`
}

type issueProject struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

func (c *client) search(jql string, maxResults int) (*searchResponse, error) {
	path := fmt.Sprintf(
		"/rest/api/3/search/jql?jql=%s&maxResults=%d&fields=summary,description,status,priority,assignee,labels,project",
		url.QueryEscape(jql),
		maxResults,
	)

	resp, err := c.get(path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var sr searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}
	return &sr, nil
}

// ── Field mapping ─────────────────────────────────────────────────────────────

func mapIssues(issues []issue, baseURL string) []tasks.Task {
	result := make([]tasks.Task, len(issues))
	for i, iss := range issues {
		result[i] = mapIssue(iss, baseURL)
	}
	return result
}

func mapIssue(iss issue, baseURL string) tasks.Task {
	f := iss.Fields

	assignee := ""
	if f.Assignee != nil {
		assignee = f.Assignee.DisplayName
	}

	return tasks.Task{
		ID:          iss.Key,
		Title:       f.Summary,
		Description: extractText(f.Description),
		Status:      mapStatus(f.Status.StatusCategory.Key),
		Priority:    mapPriority(f.Priority),
		URL:         strings.TrimRight(baseURL, "/") + "/browse/" + iss.Key,
		Assignee:    assignee,
		Labels:      f.Labels,
		Project:     f.Project.Name,
	}
}

func mapStatus(categoryKey string) tasks.Status {
	switch categoryKey {
	case "indeterminate":
		return tasks.StatusInProgress
	case "done":
		return tasks.StatusDone
	default: // "new" and anything unexpected
		return tasks.StatusTodo
	}
}

func mapPriority(p *issuePriority) tasks.Priority {
	if p == nil {
		return tasks.PriorityMedium
	}
	switch strings.ToLower(p.Name) {
	case "highest", "critical", "blocker":
		return tasks.PriorityCritical
	case "high":
		return tasks.PriorityHigh
	case "low", "lowest":
		return tasks.PriorityLow
	default: // "medium" and anything else
		return tasks.PriorityMedium
	}
}
