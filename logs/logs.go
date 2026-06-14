package logs

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

type FormatHandlerOptions struct {
	Colors bool
	Level  slog.Level
}

type FormatHandler struct {
	w      io.Writer
	colors bool
	level  slog.Level
}

func NewFormatHandler(writer io.Writer, o *FormatHandlerOptions) *FormatHandler {
	return &FormatHandler{
		w:      writer,
		colors: o.Colors,
		level:  o.Level,
	}
}

func (h FormatHandler) Handle(ctx context.Context, r slog.Record) error {
	var builder strings.Builder

	level := r.Level.String()
	if h.colors {
		switch r.Level {
		case slog.LevelError:
			builder.WriteString(colorRed + level + colorReset + " ")
		case slog.LevelWarn:
			builder.WriteString(colorYellow + level + colorReset + " ")
		case slog.LevelInfo:
			builder.WriteString(colorGreen + level + colorReset + " ")
		case slog.LevelDebug:
			builder.WriteString(colorCyan + level + colorReset + " ")
		default:
			builder.WriteString(level + " ")
		}
	} else {
		builder.WriteString(level + " ")
	}

	builder.WriteString(r.Message + " ")

	r.Attrs(func(attr slog.Attr) bool {
		builder.WriteString(attr.Key + "=")
		if attr.Key == "duration" {
			if dur, ok := attr.Value.Any().(time.Duration); ok {
				builder.WriteString(formatDurationWithColor(dur, h.colors))
			} else {
				builder.WriteString(attr.Value.String())
			}
		} else {
			builder.WriteString(attr.Value.String())
		}

		builder.WriteString(" ")
		return true
	})
	builder.WriteString("\n")
	_, err := h.w.Write([]byte(builder.String()))
	return err
}

func formatDurationWithColor(d time.Duration, useColors bool) string {
	if !useColors {
		return d.String()
	}

	if d > 50*time.Millisecond {
		return colorRed + d.String() + colorReset
	}
	if d > 20*time.Millisecond {
		return colorYellow + d.String() + colorReset
	}

	return d.String()
}

func (h *FormatHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return int(level) >= int(h.level)
}

func (h *FormatHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *FormatHandler) WithGroup(name string) slog.Handler {
	return h
}
