// Package tasks defines the core Task domain model and the Provider interface
// that all task integrations must satisfy.
package tasks

// Status represents the workflow state of a task.
type Status int

const (
	StatusTodo       Status = iota
	StatusInProgress Status = iota
	StatusInReview   Status = iota
	StatusDone       Status = iota
)

// String returns the human-readable label for a Status.
func (s Status) String() string {
	switch s {
	case StatusTodo:
		return "To Do"
	case StatusInProgress:
		return "In Progress"
	case StatusInReview:
		return "In Review"
	case StatusDone:
		return "Done"
	default:
		return "Unknown"
	}
}

// Priority represents the urgency of a task.
type Priority int

const (
	PriorityLow      Priority = iota
	PriorityMedium   Priority = iota
	PriorityHigh     Priority = iota
	PriorityCritical Priority = iota
)

// String returns the human-readable label for a Priority.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "Low"
	case PriorityMedium:
		return "Medium"
	case PriorityHigh:
		return "High"
	case PriorityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}

// Task is the canonical representation of a unit of work, normalised from
// any upstream provider (Jira, Linear, GitHub Issues, etc.).
type Task struct {
	ID          string
	Title       string
	Description string
	Status      Status
	Priority    Priority
	URL         string
	Assignee    string
	Labels      []string
	Project     string
	ParentID    string // empty if top-level task
	HasChildren bool   // hint: true if this task has subtasks (avoids N+1 on list)
}

// CreateInput is the provider-agnostic description of a new task or subtask.
// ParentID is empty for top-level tasks; non-empty signals a subtask request.
// Providers that do not support subtask linking may silently ignore ParentID.
type CreateInput struct {
	Title       string
	Description string
	Priority    Priority
	ParentID    string
}

// Provider is the abstraction over any task management system.
// New integrations (Jira, Linear, GitHub Issues) implement this interface.
type Provider interface {
	// Name returns a short human-readable identifier for the provider.
	Name() string
	// GetTasks fetches the current user's relevant tasks.
	GetTasks() ([]Task, error)
}

// Updater is an optional capability a Provider may implement to support
// editing tasks in-place. Providers that are read-only do not need to
// implement this interface.
type Updater interface {
	// Update writes a modified task back to the upstream provider.
	// Only Title and Description are expected to be updated.
	Update(task Task) error
}

// Creator is an optional capability a Provider may implement to support
// creating new tasks and subtasks. Providers that are read-only do not need
// to implement this interface.
type Creator interface {
	// Create sends a new task to the upstream provider and returns the
	// canonical Task populated with the provider-assigned ID, URL, etc.
	Create(input CreateInput) (Task, error)
}

// SubtaskFetcher is an optional capability a Provider may implement to
// retrieve the children of a task. Used to populate the subtask section in
// the detail view. Providers without native subtask support do not need to
// implement this interface.
type SubtaskFetcher interface {
	// GetSubtasks returns the immediate children of the given task ID.
	GetSubtasks(parentID string) ([]Task, error)
}
