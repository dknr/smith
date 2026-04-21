package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
)

func toolList(ctx context.Context, argsJSON string) (string, error) {
	path := "."
	all := false
	if argsJSON != "" && argsJSON != "{}" {
		var p struct {
			Path string `json:"path"`
			All  bool   `json:"all"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &p); err == nil {
			if p.Path != "" {
				path = p.Path
			}
			all = p.All
		}
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	var names []string
	for _, e := range entries {
		name := e.Name()
		if !all && len(name) > 0 && name[0] == '.' {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		suffix := ""
		if info.IsDir() {
			suffix = "/"
		}
		names = append(names, name+suffix)
	}

	if len(names) == 0 {
		return "(empty)", nil
	}
	return strings.Join(names, "\n")+"\n", nil
}
