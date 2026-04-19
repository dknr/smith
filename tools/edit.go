package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// toolEdit performs exact-match find-and-replace edits on a file, or creates
// the file if old_string is empty. The path is relative to the working
// directory and is checked for path traversal.
func toolEdit(ctx context.Context, argsJSON string) (string, error) {
	var p struct {
		FilePath   string `json:"file_path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll *bool  `json:"replace_all,omitempty"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &p); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if p.FilePath == "" {
		return "", fmt.Errorf("file_path is required")
	}

	// Resolve path relative to working directory and check for traversal.
	safePath, err := safePath(p.FilePath)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(safePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	existing := len(data) > 0
	creating := !existing && p.OldString == ""

	if existing && p.OldString == "" {
		return "", fmt.Errorf("cannot create: file already exists at %s (set old_string to replace content)", safePath)
	}

	content := string(data)

	if p.OldString != "" {
		// Count occurrences for validation.
		count := strings.Count(content, p.OldString)
		if count == 0 {
			return "", fmt.Errorf("old_string not found in file")
		}
		if !*p.ReplaceAll && count > 1 {
			return "", fmt.Errorf("old_string matches %d locations; set replace_all=true to replace all, or provide more context for a unique match", count)
		}

		if *p.ReplaceAll {
			content = strings.ReplaceAll(content, p.OldString, p.NewString)
		} else {
			content = strings.Replace(content, p.OldString, p.NewString, 1)
		}
	} else if creating {
		content = p.NewString
	}

	// Ensure parent directories exist.
	if err := os.MkdirAll(filepath.Dir(safePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directories: %w", err)
	}

	if err := os.WriteFile(safePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	if creating {
		return fmt.Sprintf("Created %s", safePath), nil
	}
	return "OK", nil
}

// safePath resolves a path relative to the working directory and verifies it
// does not escape via path traversal (e.g. ../..).
func safePath(rel string) (string, error) {
	// Reject absolute paths.
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("absolute paths are not allowed: %s", rel)
	}

	// Reject paths containing ".." components.
	cleaned := filepath.Clean(rel)
	if strings.Contains(cleaned, "..") || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("path traversal not allowed: %s", rel)
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	return filepath.Join(wd, cleaned), nil
}
