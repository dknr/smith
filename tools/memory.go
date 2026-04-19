package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"smith/memory"
	"smith/types"
)

// MemoryTool provides read, add, update, and delete operations for the agent's memory store.
type MemoryTool struct {
	store *memory.Store
}

// NewMemoryTool creates a MemoryTool backed by the given store.
func NewMemoryTool(store *memory.Store) *MemoryTool {
	return &MemoryTool{store: store}
}

// ToolDef returns the tool definition for the LLM.
var MemoryToolDef = types.ToolDef{
	Name:        "memory",
	Description: "Manage the agent's long-term memory store. Actions: read (list recent memories, optionally filter by category), add (store a new memory with content, optional category and tags), update (modify an existing memory by ID), delete (remove a memory by ID).",
	Parameters: map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action to perform: 'read', 'add', 'update', or 'delete'",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Memory content (required for 'add' and 'update')",
			},
			"category": map[string]interface{}{
				"type":        "string",
				"description": "Memory category: lesson, pattern, preference, fact, mistake, context",
			},
			"tags": map[string]interface{}{
				"type":        "string",
				"description": "Comma-separated tags",
			},
			"id": map[string]interface{}{
				"type":        "integer",
				"description": "Memory ID (required for 'update' and 'delete')",
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Number of memories to return for 'read' (default: 30)",
			},
		},
		"required": []string{"action"},
	},
}

// Execute handles memory tool actions.
func (t *MemoryTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Action   string `json:"action"`
		Content  string `json:"content"`
		Category string `json:"category"`
		Tags     string `json:"tags"`
		ID       int64  `json:"id"`
		Limit    int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	switch args.Action {
	case "read":
		if args.Category != "" {
			limit := args.Limit
			if limit <= 0 {
				limit = 30
			}
			mems, err := t.store.RecentByCategory(args.Category, limit)
			if err != nil {
				return "", fmt.Errorf("failed to read memories: %w", err)
			}
			return formatMemories(mems), nil
		}
		limit := args.Limit
		if limit <= 0 {
			limit = 30
		}
		mems, err := t.store.LoadAll(limit)
		if err != nil {
			return "", fmt.Errorf("failed to read memories: %w", err)
		}
		return formatMemories(mems), nil

	case "add":
		if args.Content == "" {
			return "", fmt.Errorf("content is required for add action")
		}
		cat := args.Category
		if cat == "" {
			cat = "context"
		}
		id, err := t.store.Add(args.Content, cat, args.Tags)
		if err != nil {
			return "", fmt.Errorf("failed to add memory: %w", err)
		}
		return fmt.Sprintf("Memory added (id=%d, category=%q).", id, cat), nil

	case "update":
		if args.ID <= 0 {
			return "", fmt.Errorf("id is required for update action")
		}
		if args.Content == "" {
			return "", fmt.Errorf("content is required for update action")
		}
		cat := args.Category
		if cat == "" {
			cat = "context"
		}
		if err := t.store.Update(args.ID, args.Content, cat, args.Tags); err != nil {
			return "", fmt.Errorf("failed to update memory: %w", err)
		}
		return fmt.Sprintf("Memory %d updated.", args.ID), nil

	case "delete":
		if args.ID <= 0 {
			return "", fmt.Errorf("id is required for delete action")
		}
		if err := t.store.Delete(args.ID); err != nil {
			return "", fmt.Errorf("failed to delete memory: %w", err)
		}
		return fmt.Sprintf("Memory %d deleted.", args.ID), nil

	default:
		return "", fmt.Errorf("unknown action: %s (use 'read', 'add', 'update', or 'delete')", args.Action)
	}
}

func formatMemories(mems []memory.Memory) string {
	if len(mems) == 0 {
		return "(no memories)"
	}
	var parts []string
	for _, m := range mems {
		parts = append(parts, fmt.Sprintf("[%s] %s (id=%d, tags=%q)", m.Category, m.Content, m.ID, m.Tags))
	}
	return fmt.Sprintf("%d memories:\n%s", len(mems), joinStrings(parts, "\n"))
}

func joinStrings(s []string, sep string) string {
	if len(s) == 0 {
		return ""
	}
	result := s[0]
	for _, v := range s[1:] {
		result += sep + v
	}
	return result
}
