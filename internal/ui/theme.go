package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds the complete colour palette for the application.
// All styles are rebuilt from the active theme whenever SetTheme is called.
type Theme struct {
	// Base palette
	Surface lipgloss.Color
	Text    lipgloss.Color
	Subtle  lipgloss.Color
	Primary lipgloss.Color
	Border  lipgloss.Color

	// Status colours
	StatusTodo       lipgloss.Color
	StatusInProgress lipgloss.Color
	StatusInReview   lipgloss.Color
	StatusDone       lipgloss.Color

	// Priority colours
	PriorityLow      lipgloss.Color
	PriorityMedium   lipgloss.Color
	PriorityHigh     lipgloss.Color
	PriorityCritical lipgloss.Color
}

// activeTheme is the theme currently in use. All style functions read from it.
var activeTheme Theme

// builtinThemes is the registry of shipped themes, keyed by lowercase name.
var builtinThemes = map[string]Theme{
	"tokyonight": {
		Surface: "#24283b", Text: "#c0caf5", Subtle: "#565f89",
		Primary: "#7aa2f7", Border: "#3b4261",
		StatusTodo: "#565f89", StatusInProgress: "#7aa2f7",
		StatusInReview: "#e0af68", StatusDone: "#9ece6a",
		PriorityLow: "#565f89", PriorityMedium: "#e0af68",
		PriorityHigh: "#ff9e64", PriorityCritical: "#f7768e",
	},
	"dracula": {
		Surface: "#282a36", Text: "#f8f8f2", Subtle: "#6272a4",
		Primary: "#bd93f9", Border: "#44475a",
		StatusTodo: "#6272a4", StatusInProgress: "#8be9fd",
		StatusInReview: "#ffb86c", StatusDone: "#50fa7b",
		PriorityLow: "#6272a4", PriorityMedium: "#ffb86c",
		PriorityHigh: "#ff79c6", PriorityCritical: "#ff5555",
	},
	"catppuccin": {
		Surface: "#1e1e2e", Text: "#cdd6f4", Subtle: "#585b70",
		Primary: "#89b4fa", Border: "#45475a",
		StatusTodo: "#585b70", StatusInProgress: "#89b4fa",
		StatusInReview: "#f9e2af", StatusDone: "#a6e3a1",
		PriorityLow: "#585b70", PriorityMedium: "#f9e2af",
		PriorityHigh: "#fab387", PriorityCritical: "#f38ba8",
	},
	"gruvbox": {
		Surface: "#282828", Text: "#ebdbb2", Subtle: "#928374",
		Primary: "#83a598", Border: "#504945",
		StatusTodo: "#928374", StatusInProgress: "#83a598",
		StatusInReview: "#fabd2f", StatusDone: "#b8bb26",
		PriorityLow: "#928374", PriorityMedium: "#fabd2f",
		PriorityHigh: "#fe8019", PriorityCritical: "#fb4934",
	},
	"nord": {
		Surface: "#2e3440", Text: "#eceff4", Subtle: "#4c566a",
		Primary: "#88c0d0", Border: "#3b4252",
		StatusTodo: "#4c566a", StatusInProgress: "#88c0d0",
		StatusInReview: "#ebcb8b", StatusDone: "#a3be8c",
		PriorityLow: "#4c566a", PriorityMedium: "#ebcb8b",
		PriorityHigh: "#d08770", PriorityCritical: "#bf616a",
	},
	"onedark": {
		Surface: "#282c34", Text: "#abb2bf", Subtle: "#5c6370",
		Primary: "#61afef", Border: "#3e4451",
		StatusTodo: "#5c6370", StatusInProgress: "#61afef",
		StatusInReview: "#e5c07b", StatusDone: "#98c379",
		PriorityLow: "#5c6370", PriorityMedium: "#e5c07b",
		PriorityHigh: "#d19a66", PriorityCritical: "#e06c75",
	},
	"light": {
		Surface: "#f8f8f8", Text: "#383a42", Subtle: "#a0a1a7",
		Primary: "#4078f2", Border: "#e2e2e2",
		StatusTodo: "#a0a1a7", StatusInProgress: "#4078f2",
		StatusInReview: "#c18401", StatusDone: "#50a14f",
		PriorityLow: "#a0a1a7", PriorityMedium: "#c18401",
		PriorityHigh: "#d75f00", PriorityCritical: "#e45649",
	},
}

// noColorTheme uses empty color values so the terminal's own defaults show through.
var noColorTheme = Theme{}

// SetTheme activates a built-in theme by name. If the NO_COLOR environment
// variable is set, all colours are suppressed regardless of the requested theme.
// Unknown names fall back to the default (Tokyo Night).
func SetTheme(name string) {
	// Respect the NO_COLOR convention (https://no-color.org).
	if _, set := os.LookupEnv("NO_COLOR"); set {
		activeTheme = noColorTheme
		return
	}

	key := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	if t, ok := builtinThemes[key]; ok {
		activeTheme = t
		return
	}
	// Unknown or empty name — use Tokyo Night as the default.
	activeTheme = builtinThemes["tokyonight"]
}

// AvailableThemes returns the names of all built-in themes in display order.
func AvailableThemes() []string {
	return []string{
		"tokyonight",
		"dracula",
		"catppuccin",
		"gruvbox",
		"nord",
		"onedark",
		"light",
	}
}
