package client

import (
	"bufio"
	"io"
	"os"

	"github.com/chzyer/readline"
)

// Terminal wraps a readline.Instance and provides methods for all
// terminal I/O in the client. Every write goes through the readline
// library, which handles raw mode, line editing, history, and
// concurrent output during streaming.
type Terminal struct {
	rl      *readline.Instance
	scanner *bufio.Scanner // only used when readline isn't available (e.g. non-interactive)
}

// NewTerminal creates a new Terminal with the given prompt.
func NewTerminal(prompt string) (*Terminal, error) {
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistorySearchFold: true,
		HistoryLimit:      500,
		AutoComplete:    buildCompleter(),
	})
	if err != nil {
		return nil, err
	}
	return &Terminal{rl: rl}, nil
}

// NewTerminalNonInteractive creates a Terminal for non-interactive use.
// All output goes to os.Stdout/os.Stderr and input comes from os.Stdin via scanner.
func NewTerminalNonInteractive() *Terminal {
	return &Terminal{
		scanner: bufio.NewScanner(os.Stdin),
	}
}

// Stdout returns the readline-aware stdout writer.
func (t *Terminal) Stdout() io.Writer {
	if t.rl != nil {
		return t.rl.Stdout()
	}
	return os.Stdout
}

// Stderr returns the readline-aware stderr writer.
func (t *Terminal) Stderr() io.Writer {
	if t.rl != nil {
		return t.rl.Stderr()
	}
	return os.Stderr
}

// Readline reads a line from the user via readline.
func (t *Terminal) Readline() (string, error) {
	if t.rl != nil {
		return t.rl.Readline()
	}
	// Fallback for non-interactive: read from scanner
	if t.scanner.Scan() {
		return t.scanner.Text(), nil
	}
	return "", io.EOF
}

// SetPrompt updates the prompt.
func (t *Terminal) SetPrompt(p string) {
	if t.rl != nil {
		t.rl.SetPrompt(p)
	}
}

// Refresh forces a redraw.
func (t *Terminal) Refresh() {
	if t.rl != nil {
		t.rl.Refresh()
	}
}

// Clean clears the editing line.
func (t *Terminal) Clean() {
	if t.rl != nil {
		t.rl.Clean()
	}
}

// Close shuts down the readline instance.
func (t *Terminal) Close() error {
	if t.rl != nil {
		return t.rl.Close()
	}
	return nil
}

// Line returns the current line content (for non-interactive mode).
func (t *Terminal) Line() *readline.Result {
	if t.rl != nil {
		return t.rl.Line()
	}
	return nil
}

// modePrompt returns the colored prompt prefix based on the current mode.
func modePrompt(mode string) string {
	if mode == "" {
		return "> "
	}
	var color string
	switch mode {
	case "safe":
		color = "\033[32m" // green foreground
	case "edit":
		color = "\033[36m" // cyan foreground
	case "full":
		color = "\033[31m" // red foreground
	default:
		color = "\033[90m" // grey foreground
	}
	return color + "[" + mode + "]" + "\033[0m > "
}

// isInteractive returns true if this Terminal uses readline (interactive mode).
func (t *Terminal) isInteractive() bool {
	return t.rl != nil
}