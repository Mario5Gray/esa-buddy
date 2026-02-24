package logging

import (
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/meain/esa/internal/config"
	"github.com/meain/esa/internal/utils"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	defaultLevel      = "info"
	defaultFormat     = "text"
	defaultMaxAgeDays = 30
	defaultMaxSizeMB  = 50
)

// Setup builds a structured logger and configures stdlib log output.
func Setup(cfg config.LoggingConfig) (*slog.Logger, io.Closer, error) {
	level := strings.TrimSpace(cfg.Level)
	if level == "" {
		level = defaultLevel
	}
	format := strings.TrimSpace(cfg.Format)
	if format == "" {
		format = defaultFormat
	}

	writer, closer, err := buildWriter(cfg)
	if err != nil {
		return nil, nil, err
	}
	if writer == nil {
		writer = io.Discard
	}

	log.SetOutput(writer)
	log.SetFlags(log.LstdFlags)

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "text":
		handler = slog.NewTextHandler(writer, &slog.HandlerOptions{Level: parseLevel(level)})
	case "json":
		handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: parseLevel(level)})
	default:
		return nil, closer, errors.New("unsupported log format")
	}

	logger := slog.New(handler)
	return logger, closer, nil
}

func buildWriter(cfg config.LoggingConfig) (io.Writer, io.Closer, error) {
	toFile := cfg.ToFile
	toStdout := cfg.ToStdout
	if !toFile && !toStdout {
		toFile = true
	}

	var writers []io.Writer
	var closer io.Closer

	if toFile {
		filePath, err := resolveLogPath(cfg.File)
		if err != nil {
			return nil, nil, err
		}
		maxAge := cfg.MaxAgeDays
		if maxAge <= 0 {
			maxAge = defaultMaxAgeDays
		}
		maxSize := cfg.MaxSizeMB
		if maxSize <= 0 {
			maxSize = defaultMaxSizeMB
		}

		rotator := &lumberjack.Logger{
			Filename:   filePath,
			MaxAge:     maxAge,
			MaxSize:    maxSize,
			MaxBackups: cfg.MaxBackups,
			Compress:   false,
		}
		writers = append(writers, rotator)
		closer = rotator
	}

	if toStdout {
		writers = append(writers, os.Stderr)
	}

	if len(writers) == 1 {
		return writers[0], closer, nil
	}
	return io.MultiWriter(writers...), closer, nil
}

func resolveLogPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		cacheDir, err := utils.SetupCacheDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(cacheDir, "esa.log"), nil
	}
	path = utils.ExpandHomePath(path)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return path, nil
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
