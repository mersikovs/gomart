package main

import (
	"flag"
	"os"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/logger"
)

func main() {
	log := logger.InitLogger("development", "info")

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	cfg, err := config.Load(fs, os.Args[1:], config.OSenv{})
	if err != nil {
		log.Error("ошибка получения конфиругации сервиса ", "error", err)
	}

	log.Info("приложение запушенно ", "config", cfg.Safe())
}
