package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/config"
	"github.com/mersikovs/gomart/internal/database"
	"github.com/mersikovs/gomart/internal/handler"
	"github.com/mersikovs/gomart/internal/logger"
	"github.com/mersikovs/gomart/internal/repository"
	"github.com/mersikovs/gomart/internal/router"
	"github.com/mersikovs/gomart/internal/server"
	"github.com/mersikovs/gomart/internal/worker"
)

const (
	exitOK        = 0
	exitConfig    = 1
	exitMigration = 2
	exitStorage   = 3
	exitServer    = 4
	exitShutdown  = 5
)

func main() {
	os.Exit(run())
}

func run() int {
	log := logger.InitLogger("development", "debug")

	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	cfg, err := config.Load(fs, os.Args[1:], config.OSenv{})
	if err != nil {
		log.Error("ошибка получения конфиругации сервиса", "error", err)
		return exitConfig
	}

	if err := database.MigrateUp(cfg.DatabaseURI); err != nil {
		log.Error("ошибка миграции базы данных", "error", err)
		return exitMigration
	}

	log.Info("приложение запускается", "config", cfg.Safe())

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()

	storage, err := repository.NewStorage(appCtx, cfg, log)
	if err != nil {
		log.Error("ошибка создания объекта хранилища", "error", err)
		return exitStorage
	}

	if closer, ok := storage.(io.Closer); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Info("Ошибка закрытия хранилища", "error", err)
			}
		}()
	}

	accrualClient := accrualclient.NewHTTPClient(
		cfg.AccrualSystemAddress,
		5*time.Second,
		1,
	)

	orderPool := worker.NewPool(
		"order-processing",
		5,
		10,
	)
	orderPool.Start()

	orderProcessor := worker.NewOrderProcessor(
		storage,
		accrualClient,
		orderPool,
		1*time.Second,
	)
	go orderProcessor.Run(appCtx)

	h := handler.New(storage, accrualClient, cfg.JWTSecret, cfg.BcryptCost, log)
	router := router.Setup(h, log)
	srv := server.New(router, cfg.RunAddress, log)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	errCh := make(chan error, 1)
	go func() {
		err := srv.Start()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	code := 0
	var runErr error

	select {
	case err := <-errCh:
		if err != nil {
			log.Error("ошибка запуска сервера", "error", err)
			code = exitServer
			runErr = err
		} else {
			log.Info("сервер завершился штатно")
		}
	case sig := <-quit:
		log.Info("сигнал завершения работы", "signal", sig.String())
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	orderPool.Stop()
	if err := srv.Stop(shutdownCtx); err != nil {
		log.Error("ошибка остановки сервера", "error", err)
		if runErr == nil {
			code = exitShutdown
			runErr = err
		}
	}

	log.Info("приложение остановлено")

	return code
}
