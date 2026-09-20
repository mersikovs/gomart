// Package logger инкапсулирует настройку и конфигурацию глобального логгера на базе log/slog.
// Пакет определяет формат вывода (текстовый для разработки, JSON для production)
// и устанавливает экземпляр slog.Logger как стандартный через slog.SetDefault().
package logger

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger создает и настраивает экземпляр *slog.Logger в зависимости от окружения и уровня логирования.
func InitLogger(env string, level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: logLevel}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	l := slog.New(handler)
	slog.SetDefault(l)
	return l
}
