package jira

import (
	"strconv"
	"strings"
)

// adfNode represents a node in Atlassian Document Format (ADF), the JSON
// structure Jira API v3 uses for rich-text fields like issue descriptions.
type adfNode struct {
	Type    string     `json:"type"`
	Text    string     `json:"text,omitempty"`
	Content []*adfNode `json:"content,omitempty"`
}

// extractText walks an ADF node tree and returns the plain-text content,
// preserving paragraph breaks and list item markers.
func extractText(node *adfNode) string {
	if node == nil {
		return ""
	}
	var sb strings.Builder
	walkADF(node, &sb)
	return strings.TrimSpace(sb.String())
}

func walkADF(node *adfNode, sb *strings.Builder) {
	switch node.Type {
	case "text":
		sb.WriteString(node.Text)

	case "hardBreak":
		sb.WriteString("\n")

	case "paragraph":
		for _, child := range node.Content {
			walkADF(child, sb)
		}
		sb.WriteString("\n")

	case "heading":
		for _, child := range node.Content {
			walkADF(child, sb)
		}
		sb.WriteString("\n")

	case "bulletList":
		for _, item := range node.Content {
			sb.WriteString("  • ")
			walkChildren(item.Content, sb)
		}

	case "orderedList":
		for i, item := range node.Content {
			sb.WriteString("  ")
			sb.WriteString(strconv.Itoa(i + 1))
			sb.WriteString(". ")
			walkChildren(item.Content, sb)
		}

	case "codeBlock":
		sb.WriteString("\n")
		for _, child := range node.Content {
			walkADF(child, sb)
		}
		sb.WriteString("\n")

	default:
		// For any unrecognised node type, recurse into children.
		for _, child := range node.Content {
			walkADF(child, sb)
		}
	}
}

func walkChildren(nodes []*adfNode, sb *strings.Builder) {
	for _, n := range nodes {
		walkADF(n, sb)
	}
	sb.WriteString("\n")
}
