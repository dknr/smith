package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"smith/memory"
	"smith/session"
	"smith/types"
)

// SearchTool provides full-text search across memory and conversation history.
type SearchTool struct {
	memStore *memory.Store
	sess     *session.Session
}

// NewSearchTool creates a SearchTool backed by the given memory store and session.
func NewSearchTool(memStore *memory.Store, sess *session.Session) *SearchTool {
	return &SearchTool{
		memStore: memStore,
		sess:     sess,
	}
}

// SearchToolDef is the tool definition for the LLM.
var SearchToolDef = types.ToolDef{
	Name:        "search",
	Description: "Search memory and conversation history for relevant information. Returns results from both the agent's long-term memory store and archived session history.",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query text",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of results to return (default: 10)",
			},
		},
		"required": []string{"query"},
	},
}

// Execute handles search tool actions.
func (t *SearchTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Query == "" {
		return "", fmt.Errorf("query is required")
	}

	args.Query = strings.TrimSpace(args.Query)
	if args.Query == "" {
		return "", fmt.Errorf("query cannot be empty or whitespace-only")
	}

	// Wrap query in quotes to handle FTS5 special characters (*, AND, OR, etc.)
	// Escape any existing double quotes by doubling them
	escaped := strings.ReplaceAll(args.Query, `"`, `""`)
	args.Query = `"` + escaped + `"`

	if args.Limit <= 0 {
		args.Limit = 10
	}

	var results []string

	// Search memory store
	if t.memStore != nil {
		memResults, err := t.memStore.Search(args.Query, args.Limit/2)
		if err != nil {
			// Log error but continue with session search
			results = append(results, fmt.Sprintf("[Memory search error: %v]", err))
		} else if len(memResults) > 0 {
			results = append(results, "=== Memory Results ===")
			for _, m := range memResults {
				results = append(results, fmt.Sprintf("[%s] %s (id=%d, tags=%q, created=%s)",
					m.Category, m.Content, m.ID, m.Tags, m.CreatedAt))
			}
		}
	}

	// Search session history
	if t.sess != nil {
		sessResults, err := t.sess.Search(args.Query, args.Limit/2)
		if err != nil {
			results = append(results, fmt.Sprintf("[Session search error: %v]", err))
		} else if len(sessResults) > 0 {
			results = append(results, "=== Conversation History ===")
			for _, msg := range sessResults {
				content := msg.Content
				if len(content) > 200 {
					content = content[:200] + "..."
				}
				results = append(results, fmt.Sprintf("[%s] %s (session_id=%d)",
					msg.Role, content, msg.SessionID))
			}
		}
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for query: %q", args.Query), nil
	}

	return fmt.Sprintf("Search results for %q:\n%s", args.Query, strings.Join(results, "\n")), nil
}