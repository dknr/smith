package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Setup configures a dual-output slog.Logger that writes to both a file and stdout.
// Returns the configured logger and a cleanup function.
func Setup(programName string) (*slog.Logger, func()) {
	var writers []io.Writer

	// File output
	logDir := "."
	logFile, err := os.OpenFile(filepath.Join(logDir, fmt.Sprintf("%s.log", programName)), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		writers = append(writers, logFile)
	}

	// Stdout output
	writers = append(writers, os.Stdout)

	var writer io.Writer
	switch len(writers) {
	case 0:
		writer = os.Stdout
	case 1:
		writer = writers[0]
	default:
		writer = io.MultiWriter(writers...)
	}

	logger := slog.New(slog.NewTextHandler(writer, nil))

	cleanup := func() {
		if logFile != nil {
			logFile.Close()
		}
	}

	return logger, cleanup
}
