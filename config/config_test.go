package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigDir_XDG(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := configDir()
	want := filepath.Join(tmp, "smith")
	if got != want {
		t.Errorf("configDir() = %q, want %q", got, want)
	}
}

func TestConfigDir_NoXDG(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	got := configDir()
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config", "smith")
	if got != want {
		t.Errorf("configDir() = %q, want %q", got, want)
	}
}

func TestLoad_valid(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmp, "smith"), 0755); err != nil {
		t.Fatal(err)
	}
	toml := "base_url = \"http://localhost:8080/v1\"\nmodel = \"test-model\"\n"
	if err := os.WriteFile(filepath.Join(tmp, "smith", configName), []byte(toml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.BaseURL != "http://localhost:8080/v1" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "http://localhost:8080/v1")
	}
	if cfg.Model != "test-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "test-model")
	}
}

func TestLoad_missingBaseURL(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmp, "smith"), 0755); err != nil {
		t.Fatal(err)
	}
	toml := "model = \"test\"\n"
	os.WriteFile(filepath.Join(tmp, "smith", configName), []byte(toml), 0644)

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing base_url")
	}
}

func TestLoad_missingModel(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmp, "smith"), 0755); err != nil {
		t.Fatal(err)
	}
	toml := "base_url = \"http://localhost:8080/v1\"\n"
	os.WriteFile(filepath.Join(tmp, "smith", configName), []byte(toml), 0644)

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing model")
	}
}

func TestLoad_notFound(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	_, err := Load()
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestLoad_invalidTOML(t *testing.T) {
	tmp := t.TempDir()
	os.Setenv("XDG_CONFIG_HOME", tmp)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	if err := os.MkdirAll(filepath.Join(tmp, "smith"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(tmp, "smith", configName), []byte("{{invalid"), 0644)

	_, err := Load()
	if err == nil {
		t.Error("expected error for invalid TOML")
	}
}
