package tools

import (
	"context"
	"time"
)

func toolTime(ctx context.Context, argsJSON string) (string, error) {
	// Always return RFC3339 (ISO 8601). Args are accepted but ignored for now.
	_ = argsJSON
	return time.Now().UTC().Format(time.RFC3339), nil
}
