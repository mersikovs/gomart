// Package database предоставляет утилиты для управления схемой PostgreSQL через SQL-файлы.
package database

import (
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// Драйвер базы данных Postgres для golang-migrate.
	// Регистрирует драйвер в init() через side-effect, чтобы библиотека
	// могла распознавать URL вида "postgres://...".
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations
var migrationFS embed.FS

// MigrateUp применяет все ожидающие SQL-миграции к базе данных, поднимая схему до последней версии.
func MigrateUp(dsn string) (err error) {
	src, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	defer func() {
		srcErr, dbErr := m.Close()
		if err == nil {
			err = errors.Join(srcErr, dbErr)
		}
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil

}
