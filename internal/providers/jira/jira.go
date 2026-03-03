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
	client           *client
	cfg              *config.JiraConfig
	accountID        string            // cached from /myself on first create
	subtaskTypeCache map[string]string // project key → subtask issue type name
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


// GetSubtasks satisfies tasks.SubtaskFetcher. It fetches children of the given
// issue key using JQL parent = KEY.
func (p *Provider) GetSubtasks(parentID string) ([]tasks.Task, error) {
	jql := fmt.Sprintf("parent = %s ORDER BY created ASC", parentID)
	resp, err := p.client.search(jql, 50)
	if err != nil {
		return nil, fmt.Errorf("jira subtasks: %w", err)
	}
	return mapIssues(resp.Issues, p.cfg.BaseURL), nil
}

// GetTransitions satisfies tasks.StatusUpdater. It fetches the workflow
// transitions currently available for the given issue key.
func (p *Provider) GetTransitions(taskID string) ([]tasks.StatusTransition, error) {
	resp, err := p.client.getTransitions(taskID)
	if err != nil {
		return nil, err
	}
	out := make([]tasks.StatusTransition, len(resp.Transitions))
	for i, tr := range resp.Transitions {
		out[i] = tasks.StatusTransition{
			ID:   tr.ID,
			Name: tr.Name,
			To:   mapStatus(tr.To.StatusCategory.Key),
		}
	}
	return out, nil
}

// TransitionTask satisfies tasks.StatusUpdater. It fires the given transition
// and returns the updated Task.
func (p *Provider) TransitionTask(taskID string, transitionID string) (tasks.Task, error) {
	if err := p.client.transitionIssue(taskID, transitionID); err != nil {
		return tasks.Task{}, err
	}
	iss, err := p.client.getIssue(taskID)
	if err != nil {
		return tasks.Task{}, err
	}
	return mapIssue(*iss, p.cfg.BaseURL), nil
}

// Search satisfies tasks.Searcher. It runs a full-text JQL search so the user
// can find issues not assigned to them.
func (p *Provider) Search(query string) ([]tasks.Task, error) {
	jql := fmt.Sprintf(`text ~ %q ORDER BY updated DESC`, query)
	if len(p.cfg.Projects) > 0 {
		quoted := make([]string, len(p.cfg.Projects))
		for i, proj := range p.cfg.Projects {
			quoted[i] = `"` + proj + `"`
		}
		jql = "project IN (" + strings.Join(quoted, ", ") + ") AND " + jql
	}
	resp, err := p.client.search(jql, maxResults)
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	return mapIssues(resp.Issues, p.cfg.BaseURL), nil
}

// AssignToSelf satisfies tasks.SelfAssigner. It assigns the issue to the
// currently authenticated user and returns the updated Task.
func (p *Provider) AssignToSelf(taskID string) (tasks.Task, error) {
	if p.accountID == "" {
		me, err := p.client.Myself()
		if err != nil {
			return tasks.Task{}, fmt.Errorf("could not determine current user: %w", err)
		}
		p.accountID = me.AccountId
	}
	if err := p.client.assignIssue(taskID, p.accountID); err != nil {
		return tasks.Task{}, err
	}
	iss, err := p.client.getIssue(taskID)
	if err != nil {
		return tasks.Task{}, err
	}
	return mapIssue(*iss, p.cfg.BaseURL), nil
}

// Create satisfies tasks.Creator. It creates a new issue (or subtask when
// input.ParentID is non-empty) via POST /rest/api/3/issue and returns the
// canonical Task populated from the resulting issue key.
func (p *Provider) Create(input tasks.CreateInput) (tasks.Task, error) {
	if input.Title == "" {
		return tasks.Task{}, fmt.Errorf("title is required")
	}

	projectKey := p.projectKeyFor(input)
	if projectKey == "" {
		return tasks.Task{}, fmt.Errorf("no project configured — run 'flow setup jira' to set a default project")
	}

	fields := map[string]interface{}{
		"summary":     input.Title,
		"description": plainTextToADF(input.Description),
		"project":     map[string]string{"key": projectKey},
	}

	// Assign to the current user only when explicitly requested.
	if input.AssignToSelf {
		if p.accountID == "" {
			if me, err := p.client.Myself(); err == nil {
				p.accountID = me.AccountId
			}
		}
		if p.accountID != "" {
			fields["assignee"] = map[string]string{"accountId": p.accountID}
		}
	}

	if input.ParentID != "" {
		fields["issuetype"] = map[string]string{"name": p.subtaskTypeName(projectKey)}
		fields["parent"] = map[string]string{"key": input.ParentID}
	} else {
		fields["issuetype"] = map[string]string{"name": "Task"}
	}

	createResp, err := p.client.createIssue(fields)
	if err != nil {
		return tasks.Task{}, err
	}

	// Fetch the full issue to return a complete Task.
	iss, err := p.client.getIssue(createResp.Key)
	if err != nil {
		return tasks.Task{}, err
	}
	return mapIssue(*iss, p.cfg.BaseURL), nil
}

// projectKeyFor resolves the Jira project key for a new issue.
// For subtasks the project is derived from the parent key; for top-level tasks
// it falls back to the first configured project.
func (p *Provider) projectKeyFor(input tasks.CreateInput) string {
	if input.ParentID != "" {
		if parts := strings.SplitN(input.ParentID, "-", 2); len(parts) == 2 {
			return parts[0]
		}
	}
	if len(p.cfg.Projects) > 0 {
		return p.cfg.Projects[0]
	}
	return ""
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

func (c *client) post(path string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, strings.NewReader(string(data)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

type createIssueResponse struct {
	Key string `json:"key"`
}

// createIssue POSTs to /rest/api/3/issue and returns the key of the new issue.
func (c *client) createIssue(fields map[string]interface{}) (*createIssueResponse, error) {
	resp, err := c.post("/rest/api/3/issue", map[string]interface{}{"fields": fields})
	if err != nil {
		return nil, fmt.Errorf("jira create issue: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira create issue returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var cr createIssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decoding create response: %w", err)
	}
	return &cr, nil
}

// getIssue fetches a single issue by key and returns the raw issue struct.
func (c *client) getIssue(key string) (*issue, error) {
	path := fmt.Sprintf("/rest/api/3/issue/%s?fields=summary,description,status,priority,assignee,labels,project,parent,subtasks", key)
	resp, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("jira get issue %s: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira get issue %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var iss issue
	if err := json.NewDecoder(resp.Body).Decode(&iss); err != nil {
		return nil, fmt.Errorf("decoding issue response: %w", err)
	}
	return &iss, nil
}

type issueTypesResponse struct {
	IssueTypes []issueTypeItem `json:"issueTypes"`
}

type issueTypeItem struct {
	Name    string `json:"name"`
	Subtask bool   `json:"subtask"`
}

func (c *client) getProjectIssueTypes(projectKey string) (*issueTypesResponse, error) {
	path := fmt.Sprintf("/rest/api/3/issue/createmeta/%s/issuetypes", projectKey)
	resp, err := c.get(path)
	if err != nil {
		return nil, fmt.Errorf("jira get issue types: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira get issue types returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var itr issueTypesResponse
	if err := json.NewDecoder(resp.Body).Decode(&itr); err != nil {
		return nil, fmt.Errorf("decoding issue types response: %w", err)
	}
	return &itr, nil
}

// subtaskTypeName returns the name of the subtask issue type for projectKey.
// It queries the project's issue types once and caches the result.
// Falls back to "Subtask" if the lookup fails.
func (p *Provider) subtaskTypeName(projectKey string) string {
	if p.subtaskTypeCache == nil {
		p.subtaskTypeCache = make(map[string]string)
	}
	if name, ok := p.subtaskTypeCache[projectKey]; ok {
		return name
	}
	itr, err := p.client.getProjectIssueTypes(projectKey)
	if err == nil {
		for _, it := range itr.IssueTypes {
			if it.Subtask {
				p.subtaskTypeCache[projectKey] = it.Name
				return it.Name
			}
		}
	}
	// Fallback: common names to try
	p.subtaskTypeCache[projectKey] = "Subtask"
	return "Subtask"
}

type transitionsResponse struct {
	Transitions []transitionItem `json:"transitions"`
}

type transitionItem struct {
	ID   string              `json:"id"`
	Name string              `json:"name"`
	To   transitionTargetStatus `json:"to"`
}

type transitionTargetStatus struct {
	StatusCategory statusCategory `json:"statusCategory"`
}

func (c *client) getTransitions(key string) (*transitionsResponse, error) {
	resp, err := c.get("/rest/api/3/issue/" + key + "/transitions")
	if err != nil {
		return nil, fmt.Errorf("jira get transitions %s: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira get transitions %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var tr transitionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return nil, fmt.Errorf("decoding transitions response: %w", err)
	}
	return &tr, nil
}

func (c *client) transitionIssue(key, transitionID string) error {
	body := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	resp, err := c.post("/rest/api/3/issue/"+key+"/transitions", body)
	if err != nil {
		return fmt.Errorf("jira transition %s: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira transition %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}


func (c *client) assignIssue(key, accountID string) error {
	body := map[string]interface{}{
		"fields": map[string]interface{}{
			"assignee": map[string]string{"accountId": accountID},
		},
	}
	resp, err := c.put("/rest/api/3/issue/"+key, body)
	if err != nil {
		return fmt.Errorf("jira assign %s: %w", key, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira assign %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// ── Comments ──────────────────────────────────────────────────────────────────

// GetComments satisfies tasks.CommentLister.
func (p *Provider) GetComments(taskID string) ([]tasks.Comment, error) {
	list, err := p.client.listComments(taskID)
	if err != nil {
		return nil, err
	}
	out := make([]tasks.Comment, 0, len(list.Comments))
	for _, c := range list.Comments {
		out = append(out, tasks.Comment{
			ID:        c.ID,
			Author:    c.Author.DisplayName,
			Body:      extractText(c.Body),
			CreatedAt: formatJiraTime(c.Created),
			UpdatedAt: formatJiraTime(c.Updated),
		})
	}
	return out, nil
}

// AddComment satisfies tasks.CommentAdder.
func (p *Provider) AddComment(taskID, body string) (tasks.Comment, error) {
	c, err := p.client.addComment(taskID, body)
	if err != nil {
		return tasks.Comment{}, err
	}
	return tasks.Comment{
		ID:        c.ID,
		Author:    c.Author.DisplayName,
		Body:      extractText(c.Body),
		CreatedAt: formatJiraTime(c.Created),
		UpdatedAt: formatJiraTime(c.Updated),
	}, nil
}

// EditComment satisfies tasks.CommentEditor.
func (p *Provider) EditComment(taskID, commentID, body string) (tasks.Comment, error) {
	c, err := p.client.editComment(taskID, commentID, body)
	if err != nil {
		return tasks.Comment{}, err
	}
	return tasks.Comment{
		ID:        c.ID,
		Author:    c.Author.DisplayName,
		Body:      extractText(c.Body),
		CreatedAt: formatJiraTime(c.Created),
		UpdatedAt: formatJiraTime(c.Updated),
	}, nil
}

// DeleteComment satisfies tasks.CommentDeleter.
func (p *Provider) DeleteComment(taskID, commentID string) error {
	return p.client.deleteComment(taskID, commentID)
}

// DeleteTask satisfies TaskDeleter. It permanently deletes the Jira issue
// (and any subtasks) from the board.
func (p *Provider) DeleteTask(taskID string) error {
	return p.client.deleteIssue(taskID)
}

// GetTask satisfies ParentFetcher.
func (p *Provider) GetTask(taskID string) (tasks.Task, error) {
	iss, err := p.client.getIssue(taskID)
	if err != nil {
		return tasks.Task{}, err
	}
	return mapIssue(*iss, p.cfg.BaseURL), nil
}

// formatJiraTime converts a Jira ISO timestamp to a short display string.
func formatJiraTime(s string) string {
	// Jira returns times like "2024-01-15T09:30:00.000+0000". We trim to date+time.
	if len(s) >= 16 {
		return s[:10] + " " + s[11:16]
	}
	return s
}

// ── Comment API types ─────────────────────────────────────────────────────────

type commentListResponse struct {
	Comments []jiraComment `json:"comments"`
}

type jiraComment struct {
	ID      string     `json:"id"`
	Author  issueUser  `json:"author"`
	Body    *adfNode   `json:"body"`
	Created string     `json:"created"`
	Updated string     `json:"updated"`
}

func (c *client) listComments(key string) (*commentListResponse, error) {
	resp, err := c.get("/rest/api/3/issue/" + key + "/comment?maxResults=50&orderBy=created")
	if err != nil {
		return nil, fmt.Errorf("jira list comments %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira list comments %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var r commentListResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decoding comments response: %w", err)
	}
	return &r, nil
}

func (c *client) addComment(key, body string) (*jiraComment, error) {
	payload := map[string]interface{}{"body": plainTextToADF(body)}
	resp, err := c.post("/rest/api/3/issue/"+key+"/comment", payload)
	if err != nil {
		return nil, fmt.Errorf("jira add comment %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira add comment %s returned HTTP %d: %s", key, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var r jiraComment
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decoding add comment response: %w", err)
	}
	return &r, nil
}

func (c *client) editComment(key, commentID, body string) (*jiraComment, error) {
	payload := map[string]interface{}{"body": plainTextToADF(body)}
	resp, err := c.put("/rest/api/3/issue/"+key+"/comment/"+commentID, payload)
	if err != nil {
		return nil, fmt.Errorf("jira edit comment %s/%s: %w", key, commentID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jira edit comment returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var r jiraComment
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decoding edit comment response: %w", err)
	}
	return &r, nil
}

func (c *client) deleteComment(key, commentID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/rest/api/3/issue/"+key+"/comment/"+commentID, nil)
	if err != nil {
		return fmt.Errorf("jira delete comment %s/%s: %w", key, commentID, err)
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira delete comment %s/%s: %w", key, commentID, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira delete comment returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func (c *client) deleteIssue(key string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/rest/api/3/issue/"+key+"?deleteSubtasks=true", nil)
	if err != nil {
		return fmt.Errorf("jira delete issue %s: %w", key, err)
	}
	req.Header.Set("Authorization", c.auth)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira delete issue %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira delete issue returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

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
	AccountId    string `json:"accountId"`
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
	Summary     string         `json:"summary"`
	Description *adfNode       `json:"description"`
	Status      issueStatus    `json:"status"`
	Priority    *issuePriority `json:"priority"`
	Assignee    *issueUser     `json:"assignee"`
	Labels      []string       `json:"labels"`
	Project     issueProject   `json:"project"`
	Parent      *issueParent   `json:"parent"`
	Subtasks    []issueRef     `json:"subtasks"`
}

type issueParent struct {
	Key string `json:"key"`
}

// issueRef is a lightweight reference used for the subtasks array.
type issueRef struct {
	Key string `json:"key"`
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
		"/rest/api/3/search/jql?jql=%s&maxResults=%d&fields=summary,description,status,priority,assignee,labels,project,parent,subtasks",
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

	parentID := ""
	if f.Parent != nil {
		parentID = f.Parent.Key
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
		ParentID:    parentID,
		HasChildren: len(f.Subtasks) > 0,
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
