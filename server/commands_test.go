package server

import (
	"strings"
	"testing"
)

func TestCommandRegistry(t *testing.T) {
	// All registered commands should be findable.
	for _, cmd := range commands {
		found := commandFor(cmd.Name)
		if found == nil {
			t.Fatalf("command %s not found via commandFor", cmd.Name)
		}
		if found.Description == "" {
			t.Errorf("command %s has empty description", cmd.Name)
		}
	}

	// Unknown command should return nil.
	if commandFor("/nonexistent") != nil {
		t.Error("commandFor('/nonexistent') should return nil")
	}

	// Names should match registered commands.
	names := commandNames()
	if len(names) != len(commands) {
		t.Fatalf("commandNames() returned %d names, want %d", len(names), len(commands))
	}
	for _, name := range names {
		if !strings.HasPrefix(name, "/") {
			t.Errorf("command name %q should start with /", name)
		}
	}
}

func TestBuildHelpText(t *testing.T) {
	help := buildHelpText(commands)

	// Should contain all registered command names.
	for _, cmd := range commands {
		if !strings.Contains(help, cmd.Name) {
			t.Errorf("help text missing command %s", cmd.Name)
		}
		if cmd.Description != "" && !strings.Contains(help, cmd.Description) {
			t.Errorf("help text missing description for %s", cmd.Name)
		}
	}

	// Should not contain /help (handled specially to avoid init cycle).
	if strings.Contains(help, "/help") {
		t.Error("help text should not contain /help (handled specially)")
	}

	// Should contain client-side commands.
	for _, c := range []string{"/compact", "/quit"} {
		if !strings.Contains(help, c) {
			t.Errorf("help text missing client command %s", c)
		}
	}

	if help == "" {
		t.Error("help text is empty")
	}
}
