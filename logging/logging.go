package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// logDir is the directory where all log files are written.
const logDir = "log"

// SetupServer configures two loggers for the server:
//   - serverLogger:  Info/Error level, writes to both log/smith-serve.log and stdout
//   - networkLogger: Debug level, writes to log/smith-serve.log only (provider + WebSocket traffic)
//
// If debug is false, networkLogger is nil (traffic is silently dropped).
// Returns both loggers and a cleanup function.
func SetupServer(debug bool) (serverLogger, networkLogger *slog.Logger, cleanup func()) {
	os.MkdirAll(logDir, 0755)

	filePath := filepath.Join(logDir, "smith-serve.log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("failed to open server log file", "error", err)
		file = nil
	}

	// Server logger: writes to both file and stdout at Info+ level.
	var serverWriters []io.Writer
	if file != nil {
		serverWriters = append(serverWriters, file)
	}
	serverWriters = append(serverWriters, os.Stdout)

	var serverWriter io.Writer
	switch len(serverWriters) {
	case 0:
		serverWriter = os.Stdout
	case 1:
		serverWriter = serverWriters[0]
	default:
		serverWriter = io.MultiWriter(serverWriters...)
	}

	opts := &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: false,
	}
	serverLogger = slog.New(slog.NewTextHandler(serverWriter, opts)).With("component", "server")

	// Network logger: debug-level, file-only.
	if debug {
		var netWriters []io.Writer
		if file != nil {
			netWriters = append(netWriters, file)
		}

		var netWriter io.Writer
		switch len(netWriters) {
		case 0:
			// Fallback if file open failed — still create a no-op logger.
			netWriter = io.Discard
		case 1:
			netWriter = netWriters[0]
		default:
			netWriter = io.MultiWriter(netWriters...)
		}

		netOpts := &slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: false,
		}
		networkLogger = slog.New(slog.NewTextHandler(netWriter, netOpts)).With("component", "network")
	}

	cleanup = func() {
		if file != nil {
			file.Close()
		}
	}

	return serverLogger, networkLogger, cleanup
}

// SetupClient configures a logger for client commands (send, chat):
//   - If debug is false: returns a no-op logger (silences everything, never touches stdout)
//   - If debug is true: returns a Debug-level logger that writes to log/smith-<name>.log only
//
// Name should be "send" or "chat" to produce the appropriate log file.
func SetupClient(name string, debug bool) (logger *slog.Logger, cleanup func()) {
	if !debug {
		// No-op logger that discards everything.
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}
	}

	os.MkdirAll(logDir, 0755)

	filePath := filepath.Join(logDir, "smith-"+name+".log")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// If we can't open the file, fall back to no-op.
		return slog.New(slog.NewTextHandler(io.Discard, nil)), func() {}
	}

	opts := &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false,
	}
	logger = slog.New(slog.NewTextHandler(file, opts)).With("component", name)

	cleanup = func() {
		file.Close()
	}

	return logger, cleanup
}
