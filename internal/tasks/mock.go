package tasks

import (
	"fmt"
	"strings"
)

// MockProvider returns a realistic set of tasks for development and UI testing.
// It holds mutable state so edits made via Update() are reflected in subsequent
// GetTasks() calls within the same session.
type MockProvider struct {
	tasks    []Task
	comments map[string][]Comment // taskID → comments
	nextID   int                  // counter for generated task IDs
	nextCID  int                  // counter for generated comment IDs
}

// NewMockProvider creates a MockProvider seeded with realistic sample tasks.
func NewMockProvider() *MockProvider {
	return &MockProvider{
		tasks:    seedTasks(),
		comments: seedComments(),
		nextID:   20,
		nextCID:  10,
	}
}

// Name satisfies Provider.
func (m *MockProvider) Name() string { return "Mock" }

// GetTasks satisfies Provider.
func (m *MockProvider) GetTasks() ([]Task, error) {
	// Return only top-level tasks in the list view (subtasks are fetched on demand).
	var out []Task
	for _, t := range m.tasks {
		if t.ParentID == "" {
			out = append(out, t)
		}
	}
	return out, nil
}

// Update satisfies Updater. It applies the new Title and Description to the
// matching task in the in-memory slice.
func (m *MockProvider) Update(task Task) error {
	for i, t := range m.tasks {
		if t.ID == task.ID {
			m.tasks[i].Title = task.Title
			m.tasks[i].Description = task.Description
			return nil
		}
	}
	return fmt.Errorf("task %s not found", task.ID)
}

// Create satisfies Creator. It appends a new task to the in-memory slice and
// returns the created task with a generated ID.
func (m *MockProvider) Create(input CreateInput) (Task, error) {
	if input.Title == "" {
		return Task{}, fmt.Errorf("title is required")
	}

	id := fmt.Sprintf("MOCK-%03d", m.nextID)
	m.nextID++

	task := Task{
		ID:          id,
		Title:       input.Title,
		Description: input.Description,
		Status:      StatusTodo,
		Priority:    input.Priority,
		URL:         "https://example.atlassian.net/browse/" + id,
		Project:     "Flow CLI",
		ParentID:    input.ParentID,
	}
	if input.AssignToSelf {
		task.Assignee = "you"
	}
	m.tasks = append(m.tasks, task)

	// Mark parent as having children if a ParentID is set.
	if input.ParentID != "" {
		for i, t := range m.tasks {
			if t.ID == input.ParentID {
				m.tasks[i].HasChildren = true
				break
			}
		}
	}

	return task, nil
}

// GetSubtasks satisfies SubtaskFetcher. It returns all tasks whose ParentID
// matches the given parent ID.
func (m *MockProvider) GetSubtasks(parentID string) ([]Task, error) {
	var out []Task
	for _, t := range m.tasks {
		if t.ParentID == parentID {
			out = append(out, t)
		}
	}
	return out, nil
}

// Search satisfies Searcher. It returns all tasks whose ID, Title, or
// Description contains the query string (case-insensitive).
func (m *MockProvider) Search(query string) ([]Task, error) {
	q := strings.ToLower(query)
	var out []Task
	for _, t := range m.tasks {
		if strings.Contains(strings.ToLower(t.ID), q) ||
			strings.Contains(strings.ToLower(t.Title), q) ||
			strings.Contains(strings.ToLower(t.Description), q) {
			out = append(out, t)
		}
	}
	return out, nil
}

// AssignToSelf satisfies SelfAssigner. It sets the task's assignee to "you".
func (m *MockProvider) AssignToSelf(taskID string) (Task, error) {
	for i, t := range m.tasks {
		if t.ID == taskID {
			m.tasks[i].Assignee = "you"
			return m.tasks[i], nil
		}
	}
	return Task{}, fmt.Errorf("task %s not found", taskID)
}

// GetTransitions satisfies StatusUpdater. It returns all statuses except the
// task's current one, since the mock has no workflow constraints.
func (m *MockProvider) GetTransitions(taskID string) ([]StatusTransition, error) {
	current := StatusTodo
	for _, t := range m.tasks {
		if t.ID == taskID {
			current = t.Status
			break
		}
	}
	all := []Status{StatusTodo, StatusInProgress, StatusInReview, StatusDone}
	var out []StatusTransition
	for _, s := range all {
		if s == current {
			continue
		}
		out = append(out, StatusTransition{
			ID:   fmt.Sprintf("%d", int(s)),
			Name: s.String(),
			To:   s,
		})
	}
	return out, nil
}

// TransitionTask satisfies StatusUpdater. It updates the task's status
// in-memory and returns the updated Task.
func (m *MockProvider) TransitionTask(taskID string, transitionID string) (Task, error) {
	var newStatus Status
	switch transitionID {
	case "0":
		newStatus = StatusTodo
	case "1":
		newStatus = StatusInProgress
	case "2":
		newStatus = StatusInReview
	case "3":
		newStatus = StatusDone
	default:
		return Task{}, fmt.Errorf("unknown transition ID %q", transitionID)
	}
	for i, t := range m.tasks {
		if t.ID == taskID {
			m.tasks[i].Status = newStatus
			return m.tasks[i], nil
		}
	}
	return Task{}, fmt.Errorf("task %s not found", taskID)
}

// seedTasks returns the initial set of mock tasks
// under FLOW-001 to demonstrate the subtask feature in the UI.
func seedTasks() []Task {
	return []Task{
		{
			ID:    "FLOW-001",
			Title: "Implement OAuth flow for Jira integration",
			Description: `Set up the OAuth 2.0 authorisation code flow so users can authenticate against their Jira instance.

Steps:
  • Register an OAuth app in the Atlassian developer console
  • Implement the redirect + callback handlers
  • Store access/refresh tokens in ~/.config/flow-cli/credentials
  • Refresh tokens automatically before expiry

Acceptance criteria:
  • 'flow auth jira' opens the browser and redirects back to the CLI
  • Credentials are persisted and reused on subsequent runs
  • Expired tokens refresh transparently without user intervention`,
			Status:      StatusInProgress,
			Priority:    PriorityHigh,
			URL:         "https://example.atlassian.net/browse/FLOW-001",
			Assignee:    "you",
			Labels:      []string{"backend", "auth"},
			Project:     "Flow CLI",
			HasChildren: true,
		},
		// Subtasks of FLOW-001
		{
			ID:          "FLOW-011",
			Title:       "Register OAuth app in Atlassian developer console",
			Description: "Create the OAuth 2.0 application entry in https://developer.atlassian.com and capture the client ID and secret.",
			Status:      StatusDone,
			Priority:    PriorityHigh,
			URL:         "https://example.atlassian.net/browse/FLOW-011",
			Assignee:    "you",
			Project:     "Flow CLI",
			ParentID:    "FLOW-001",
		},
		{
			ID:          "FLOW-012",
			Title:       "Implement redirect and callback handlers",
			Description: "Set up a local HTTP server to receive the OAuth callback and exchange the auth code for tokens.",
			Status:      StatusInProgress,
			Priority:    PriorityHigh,
			URL:         "https://example.atlassian.net/browse/FLOW-012",
			Assignee:    "you",
			Project:     "Flow CLI",
			ParentID:    "FLOW-001",
		},
		{
			ID:          "FLOW-013",
			Title:       "Store and refresh access tokens",
			Description: "Persist access/refresh tokens in the OS keychain and refresh automatically before expiry.",
			Status:      StatusTodo,
			Priority:    PriorityMedium,
			URL:         "https://example.atlassian.net/browse/FLOW-013",
			Assignee:    "you",
			Project:     "Flow CLI",
			ParentID:    "FLOW-001",
		},
		{
			ID:    "FLOW-002",
			Title: "Open task URL in default browser",
			Description: `When viewing a task in the detail pane, pressing 'o' should open the task URL in the system's default browser.

Platform commands:
  • macOS  – open <url>
  • Linux  – xdg-open <url>
  • Windows – rundll32 url.dll,FileProtocolHandler <url>

Acceptance criteria:
  • Pressing 'o' opens the correct URL on all three platforms
  • An inline notice is shown if no URL is available for the task`,
			Status:   StatusTodo,
			Priority: PriorityMedium,
			URL:      "https://example.atlassian.net/browse/FLOW-002",
			Assignee: "you",
			Labels:   []string{"ui", "cross-platform"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-003",
			Title: "Design configuration file schema",
			Description: `Define the YAML schema for ~/.config/flow-cli/config.yaml.

The schema should accommodate:
  • Multiple task providers with per-provider settings
  • UI preferences (colour theme, date format, items per page)
  • Calendar provider settings (future)

Acceptance criteria:
  • Schema documented inline with comments
  • A default config is generated on first run if none exists
  • Invalid config produces a descriptive error on startup`,
			Status:   StatusInReview,
			Priority: PriorityMedium,
			URL:      "https://example.atlassian.net/browse/FLOW-003",
			Assignee: "you",
			Labels:   []string{"config", "dx"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-004",
			Title: "Fix crash on terminal resize in detail view",
			Description: `The app panics with an index-out-of-bounds error when the terminal is resized while the task detail view is open.

Root cause: tea.WindowSizeMsg is not handled in the detail view branch, so the viewport's width/height are never updated.

Steps to reproduce:
  1. Run 'flow'
  2. Select any task and press Enter
  3. Resize the terminal window → panic

Fix: handle tea.WindowSizeMsg in updateDetailView and update viewport dimensions.`,
			Status:   StatusTodo,
			Priority: PriorityCritical,
			URL:      "https://example.atlassian.net/browse/FLOW-004",
			Assignee: "you",
			Labels:   []string{"bug", "ui"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-005",
			Title: "Add Linear task provider",
			Description: `Implement the Provider interface targeting the Linear GraphQL API.

Linear uses API keys (not OAuth), which simplifies auth.

GraphQL queries needed:
  • List issues assigned to the authenticated user
  • Filter by team / project
  • Fetch a single issue by ID

The API key is read from config.yaml under providers.linear.api_key.`,
			Status:   StatusTodo,
			Priority: PriorityMedium,
			URL:      "https://example.atlassian.net/browse/FLOW-005",
			Assignee: "you",
			Labels:   []string{"integration", "linear"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-006",
			Title: "Filter task list by status",
			Description: `Allow users to narrow the task list to a specific status using number keys.

Shortcuts:
  1 – All tasks
  2 – In Progress
  3 – To Do
  4 – In Review
  5 – Done (hidden by default to reduce noise)

The active filter label should be shown in the header bar.`,
			Status:   StatusTodo,
			Priority: PriorityLow,
			URL:      "https://example.atlassian.net/browse/FLOW-006",
			Assignee: "you",
			Labels:   []string{"ui", "filtering"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-007",
			Title: "Create Git branch from active task",
			Description: `When viewing a task, pressing 'b' should create and check out a new branch named after the task.

Naming convention:
  <type>/<id>-<slugified-title>
  e.g. feature/FLOW-007-create-git-branch-from-active-task

Type is derived from labels:
  'bug' label  → fix/
  otherwise    → feature/

Requirements:
  • Must be run inside a Git repository – show an error if not
  • Display the generated branch name and ask for confirmation before creating`,
			Status:   StatusTodo,
			Priority: PriorityMedium,
			URL:      "https://example.atlassian.net/browse/FLOW-007",
			Assignee: "you",
			Labels:   []string{"git", "workflow"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-008",
			Title: "Write unit tests for task model",
			Description: `Add a test suite covering the tasks package.

Cases to cover:
  • Status.String() returns the correct label for every constant
  • Priority.String() returns the correct label for every constant
  • MockProvider.GetTasks() returns a non-empty slice
  • All mock tasks have non-empty ID, Title, and a valid Status
  • Task URL, if non-empty, starts with "https://"

Use table-driven tests throughout.`,
			Status:   StatusDone,
			Priority: PriorityLow,
			URL:      "https://example.atlassian.net/browse/FLOW-008",
			Assignee: "you",
			Labels:   []string{"testing"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-009",
			Title: "Integrate Google Calendar",
			Description: `Show upcoming calendar events alongside tasks in the dashboard.

Auth: OAuth 2.0 against Google APIs (scope: calendar.readonly)

Display:
  • Next 5 events for today
  • Event title, start time, and Google Meet link if present
  • Colour coding: upcoming → yellow, ongoing → green, past → dim

Storage: store OAuth tokens alongside task provider tokens in credentials file.`,
			Status:   StatusTodo,
			Priority: PriorityLow,
			URL:      "https://example.atlassian.net/browse/FLOW-009",
			Assignee: "you",
			Labels:   []string{"calendar", "integration"},
			Project:  "Flow CLI",
		},
		{
			ID:    "FLOW-010",
			Title: "Publish v0.1.0 release",
			Description: `Tag and publish the first public release.

Checklist:
  • Update README with installation instructions and a screenshot
  • Write CHANGELOG.md with v0.1.0 entries
  • Build cross-platform binaries (macOS arm64/amd64, Linux amd64, Windows amd64)
  • Create GitHub Release and attach binaries
  • Draft Homebrew formula for easy installation`,
			Status:   StatusTodo,
			Priority: PriorityLow,
			URL:      "https://example.atlassian.net/browse/FLOW-010",
			Assignee: "you",
			Labels:   []string{"release", "devops"},
			Project:  "Flow CLI",
		},
	}
}

// DeleteTask satisfies TaskDeleter. It removes the task and any of its
// subtasks from the in-memory slice, updates the parent's HasChildren flag,
// and purges associated comments.
func (m *MockProvider) DeleteTask(taskID string) error {
	var found bool
	var parentID string
	var updated []Task
	for _, t := range m.tasks {
		if t.ID == taskID {
			found = true
			parentID = t.ParentID
			continue
		}
		if t.ParentID == taskID {
			// cascade-delete children
			delete(m.comments, t.ID)
			continue
		}
		updated = append(updated, t)
	}
	if !found {
		return fmt.Errorf("task %s not found", taskID)
	}
	// Clear parent HasChildren if no other children remain.
	if parentID != "" {
		hasChild := false
		for _, t := range updated {
			if t.ParentID == parentID {
				hasChild = true
				break
			}
		}
		if !hasChild {
			for i := range updated {
				if updated[i].ID == parentID {
					updated[i].HasChildren = false
					break
				}
			}
		}
	}
	m.tasks = updated
	delete(m.comments, taskID)
	return nil
}

// GetTask satisfies ParentFetcher.
func (m *MockProvider) GetTask(taskID string) (Task, error) {
	for _, t := range m.tasks {
		if t.ID == taskID {
			return t, nil
		}
	}
	return Task{}, fmt.Errorf("task %s not found", taskID)
}

// GetComments satisfies CommentLister.
func (m *MockProvider) GetComments(taskID string) ([]Comment, error) {
	return m.comments[taskID], nil
}

// AddComment satisfies CommentAdder.
func (m *MockProvider) AddComment(taskID, body string) (Comment, error) {
	c := Comment{
		ID:        fmt.Sprintf("CMT-%03d", m.nextCID),
		Author:    "you",
		Body:      body,
		CreatedAt: "just now",
		UpdatedAt: "just now",
	}
	m.nextCID++
	m.comments[taskID] = append(m.comments[taskID], c)
	return c, nil
}

// EditComment satisfies CommentEditor.
func (m *MockProvider) EditComment(taskID, commentID, body string) (Comment, error) {
	for i, c := range m.comments[taskID] {
		if c.ID == commentID {
			m.comments[taskID][i].Body = body
			m.comments[taskID][i].UpdatedAt = "just now"
			return m.comments[taskID][i], nil
		}
	}
	return Comment{}, fmt.Errorf("comment %s not found", commentID)
}

// DeleteComment satisfies CommentDeleter.
func (m *MockProvider) DeleteComment(taskID, commentID string) error {
	list := m.comments[taskID]
	for i, c := range list {
		if c.ID == commentID {
			m.comments[taskID] = append(list[:i], list[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("comment %s not found", commentID)
}

func seedComments() map[string][]Comment {
	return map[string][]Comment{
		"FLOW-001": {
			{ID: "CMT-001", Author: "alice", Body: "I've started looking into the Atlassian OAuth docs. Looks like we need to use the 3LO flow.", CreatedAt: "2 days ago", UpdatedAt: "2 days ago"},
			{ID: "CMT-002", Author: "bob", Body: "Heads up — the token expiry is 1 hour by default. Make sure we handle refresh proactively.", CreatedAt: "1 day ago", UpdatedAt: "1 day ago"},
			{ID: "CMT-003", Author: "you", Body: "Good point. I'll add a 5-minute buffer before expiry to trigger the refresh.", CreatedAt: "3 hours ago", UpdatedAt: "3 hours ago"},
		},
		"FLOW-003": {
			{ID: "CMT-004", Author: "you", Body: "Draft schema is in the PR. Would love a review on the provider nesting structure.", CreatedAt: "5 hours ago", UpdatedAt: "5 hours ago"},
		},
	}
}

