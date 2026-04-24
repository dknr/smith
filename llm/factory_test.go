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
		ProviderType: "llamacpp",
		ReasoningEffort: "medium",
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil, nil, exec.Definitions())
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
	if hp.ProviderType != "llamacpp" {
		t.Errorf("ProviderType = %q, want %q", hp.ProviderType, "llamacpp")
	}
	if hp.ReasoningEffort != "medium" {
		t.Errorf("ReasoningEffort = %q, want %q", hp.ReasoningEffort, "medium")
	}
}

func TestNewProvider_emptyAPIKey(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "http://localhost:8080/v1",
		APIKey:  "",
		Model:   "local",
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil, nil)
	hp, ok := p.(*HTTPProvider)
	if !ok {
		t.Fatalf("expected *HTTPProvider, got %T", p)
	}
	if hp.APIKey != "" {
		t.Errorf("APIKey = %q, want empty", hp.APIKey)
	}
}

// TestNewProvider_trtllmProvider tests that TRT-LLM provider type is set correctly
func TestNewProvider_trtllmProvider(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "http://localhost:8080/v1",
		APIKey:  "sk-test",
		Model:   "test-model",
		ProviderType: "trtllm",
		ReasoningEffort: "high",
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil, nil)
	hp, ok := p.(*HTTPProvider)
	if !ok {
		t.Fatalf("expected *HTTPProvider, got %T", p)
	}
	if hp.ProviderType != "trtllm" {
		t.Errorf("ProviderType = %q, want %q", hp.ProviderType, "trtllm")
	}
	if hp.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want %q", hp.ReasoningEffort, "high")
	}
}

// TestNewProvider_defaults tests that default values are set when config fields are empty
func TestNewProvider_defaults(t *testing.T) {
	cfg := &config.Config{
		BaseURL: "http://localhost:8080/v1",
		APIKey:  "sk-test",
		Model:   "test-model",
		// ProviderType and ReasoningEffort left empty
	}
	exec := tools.NewRegistry()

	p := NewProvider(cfg, exec, nil, nil)
	hp, ok := p.(*HTTPProvider)
	if !ok {
		t.Fatalf("expected *HTTPProvider, got %T", p)
	}
	// Should default to llamacpp provider type
	if hp.ProviderType != "llamacpp" {
		t.Errorf("ProviderType = %q, want %q", hp.ProviderType, "llamacpp")
	}
	// Should default to low reasoning effort
	if hp.ReasoningEffort != "low" {
		t.Errorf("ReasoningEffort = %q, want %q", hp.ReasoningEffort, "low")
	}
}
