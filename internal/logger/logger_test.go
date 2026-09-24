package logger

import (
	"log/slog"
	"testing"

	"github.com/magiconair/properties/assert"
	"github.com/stretchr/testify/require"
)

func TestInitLogger(t *testing.T) {

	tests := []struct {
		name  string
		env   string
		level string
	}{
		{"production_debug", "production", "debug"},
		{"production_info", "production", "info"},
		{"production_warn", "production", "warn"},
		{"production_error", "production", "error"},
		{"production_невалидный_уровень", "production", "trace"},
		{"development_debug", "development", "debug"},
		{"пустой_env_по_умолчанию_text", "", "warn"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			oldDefault := slog.Default()
			t.Cleanup(func() {
				slog.SetDefault(oldDefault)
			})

			logger := InitLogger(tt.env, tt.level)

			require.NotNil(t, logger, "логгер не должен быть nil")

			assert.Equal(t, logger, slog.Default(), "логгер должен быть установлен как default")
		})
	}
}
