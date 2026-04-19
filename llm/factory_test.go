package llm

import (
	"testing"

	"smith/config"
	"smith/tools"
)

func TestNewProvider_fields(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "http://localhost:8080/v1",
		APIKey:  "sk-test",
		Model:   "test-model",
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil, exec.Definitions())
	hp, ok := p.(*HTTPProvider)
	if !ok {
		t.Fatalf("expected *HTTPProvider, got %T", p)
	}
	if hp.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want %q", hp.BaseURL, "http://localhost:8080/v1")
	}
	if hp.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", hp.APIKey, "sk-test")
	}
	if hp.Model != "test-model" {
		t.Errorf("Model = %q, want %q", hp.Model, "test-model")
	}
	if len(hp.Tools) != 4 {
		t.Errorf("expected 4 tools, got %d", len(hp.Tools))
	}
}

func TestNewProvider_emptyAPIKey(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "http://localhost:8080/v1",
		APIKey:  "",
		Model:   "local",
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil)
	hp, ok := p.(*HTTPProvider)
	if !ok {
		t.Fatalf("expected *HTTPProvider, got %T", p)
	}
	if hp.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", hp.APIKey)
	}
}
