package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// CustomHandler implements slog.Handler to maintain your existing log format
// Output format: "2025/09/18 11:55:11 [INFO] message"
type CustomHandler struct {
	output io.Writer
	level  slog.Level
}

// Enabled returns whether the handler should log at the given level
func (h *CustomHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level
}

// Handle formats and writes log records
func (h *CustomHandler) Handle(_ context.Context, record slog.Record) error {
	// Format timestamp like the existing logs: "2025/09/18 11:55:11"
	timestamp := record.Time.Format("2006/01/02 15:04:05")

	// Convert slog level to your existing format
	var levelStr string
	switch record.Level {
	case slog.LevelDebug:
		levelStr = "DEBUG"
	case slog.LevelInfo:
		levelStr = "INFO"
	case slog.LevelWarn:
		levelStr = "WARN"
	case slog.LevelError:
		levelStr = "ERROR"
	default:
		levelStr = "INFO"
	}

	// Build the formatted message: "2025/09/18 11:55:11 [INFO] message"
	message := fmt.Sprintf("%s [%s] %s\n", timestamp, levelStr, record.Message)

	// Add structured attributes if any (optional enhancement)
	if record.NumAttrs() > 0 {
		attrs := make([]string, 0, record.NumAttrs())
		record.Attrs(func(attr slog.Attr) bool {
			attrs = append(attrs, fmt.Sprintf("%s=%v", attr.Key, attr.Value))
			return true
		})
		if len(attrs) > 0 {
			// For now, we'll keep it simple and not include structured data
			// But this could be enhanced later for more detailed logging
		}
	}

	_, err := h.output.Write([]byte(message))
	return err
}

// WithAttrs returns a new handler with additional attributes
func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For simplicity, we'll return the same handler
	// In a more complex implementation, you might want to track attributes
	return h
}

// WithGroup returns a new handler with a group prefix
func (h *CustomHandler) WithGroup(name string) slog.Handler {
	// For simplicity, we'll return the same handler
	// In a more complex implementation, you might want to track groups
	return h
}
