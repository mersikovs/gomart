// Package config отвечает за чтение, парсинг и валидацию конфигурационных настроек приложения.
package config

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"
)

const accrualSystemAddress = "ACCRUAL_SYSTEM_ADDRESS"
const databaseURI = "DATABASE_URI"
const runAddress = "RUN_ADDRESS"

// JWTSecret — имя переменной окружения, хранящей секретный ключ для подписи JWT-токенов.
const JWTSecret = "JWTSECRET"

// BcryptCost — имя переменной окружения, задающей стоимость (сложность) хеширования паролей bcrypt.
const BcryptCost = "BCRYPT_COST"

// AppConfig агрегирует все параметры запуска и подключения внешних сервисов.
type AppConfig struct {
	AccrualSystemAddress string // адрес системы расчёта начислений
	DatabaseURI          string // адрес подключения к базе данных
	RunAddress           string // адрес и порт запуска сервиса

	JWTSecret  string // секрет для подписи JWT-токенов
	BcryptCost int    // сложность хеширования паролей (bcrypt.GenerateFromPassword)
}

// EnvSource — интерфейс-адаптер для получения значений переменных окружения.
type EnvSource interface {
	LookupEnv(key string) (string, bool)
}

// OSenv — реализация EnvSource, использующая стандартный пакет os для чтения реальных переменных ОС.
type OSenv struct{}

// LookupEnv реализует интерфейс EnvSource, делегируя вызов стандартной функции os.LookupEnv.
func (e OSenv) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Load выполняет первичную инициализацию конфигурации приложения.
func Load(fs *flag.FlagSet, args []string, env EnvSource) (*AppConfig, error) {
	cfg := &AppConfig{}

	fs.StringVar(&cfg.RunAddress, "a", ":8080", "адрес эндпоинта HTTP-сервера")
	fs.StringVar(&cfg.DatabaseURI, "d", "", "строка подключения к БД (DSN)")
	fs.StringVar(&cfg.AccrualSystemAddress, "r", "", "адрес системы расчёта начислений")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg.AccrualSystemAddress = getEnvOrArg(env, accrualSystemAddress, cfg.AccrualSystemAddress)
	cfg.DatabaseURI = getEnvOrArg(env, databaseURI, cfg.DatabaseURI)
	cfg.RunAddress = getEnvOrArg(env, runAddress, cfg.RunAddress)

	// значения по умолчанию для автотестов
	cfg.JWTSecret = getEnvOrArg(env, JWTSecret, "DefaultSecret")
	bcryptCost, find := env.LookupEnv(BcryptCost)
	if find {
		cost, err := strconv.Atoi(bcryptCost)
		if err == nil {
			cfg.BcryptCost = cost
		}
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return cfg, nil
}

func getEnvOrArg(env EnvSource, envKey, cliArg string) string {
	if envVal, exists := env.LookupEnv(envKey); exists && envVal != "" {
		return envVal
	}
	return cliArg
}

func (c *AppConfig) validate() error {

	if c.DatabaseURI == "" {
		return fmt.Errorf("env %s or flag --d  is required", c.DatabaseURI)
	}
	if c.AccrualSystemAddress == "" {
		return fmt.Errorf("env %s or flag --r is required", accrualSystemAddress)
	}

	return nil
}

// Safe создает копию текущей конфигурации, очищенную от чувствительных данных.
func (c *AppConfig) Safe() AppConfig {
	return AppConfig{
		AccrualSystemAddress: c.AccrualSystemAddress,
		DatabaseURI:          maskDSN(c.DatabaseURI),
		RunAddress:           c.RunAddress,
	}
}

func maskDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "BIGSECRET"
	}

	if u.User != nil {
		u.User = url.UserPassword(u.User.Username(), "BIGSECRET")
	}
	return u.String()
}
