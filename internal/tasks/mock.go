package tasks

// MockProvider returns a static set of realistic tasks.
// It is used during development and for UI testing before real integrations exist.
type MockProvider struct{}

// NewMockProvider creates a new MockProvider.
func NewMockProvider() *MockProvider {
	return &MockProvider{}
}

// Name satisfies Provider.
func (m *MockProvider) Name() string { return "Mock" }

// GetTasks returns a curated set of mock tasks that cover a variety of
// statuses, priorities, and label combinations.
func (m *MockProvider) GetTasks() ([]Task, error) {
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
			Status:   StatusInProgress,
			Priority: PriorityHigh,
			URL:      "https://example.atlassian.net/browse/FLOW-001",
			Assignee: "you",
			Labels:   []string{"backend", "auth"},
			Project:  "Flow CLI",
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
	}, nil
}
