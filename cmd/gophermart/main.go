package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/handler"
	"github.com/mersikovs/gomart/internal/logger"
	"github.com/mersikovs/gomart/internal/router"
	"github.com/mersikovs/gomart/internal/server"
)

func main() {
	logger := logger.InitLogger("development", "debug")

	fs := flag.NewFlagSet("agent", flag.ContinueOnError)
	cfg, err := config.Load(fs, os.Args[1:], config.OSenv{})
	if err != nil {
		logger.Error("ошибка получения конфиругации сервиса", "error", err)
		return
	}

	logger.Info("приложение запускается", "config", cfg.Safe())

	h := handler.New(logger)

	router := router.Setup(h, logger)

	srv := server.New(router, cfg.RunAddress, logger)

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			logger.Error("ошибка запуска сервера", "error", err)
			return
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	logger.Info("сигнал завершения работы", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		logger.Error("ошибка остановки сервера", "error", err)
	}

	logger.Info("приложение остановлено")
}
