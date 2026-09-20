// Package server содержит настройку и управление жизненным циклом HTTP-сервера.
package server

import (
	"context"
	"log/slog"
	"net/http"
)

// Server структура со сылками на стандартный HTTP сервер и логгер для работы с ними
type Server struct {
	http   *http.Server // стандартный HTTP-сервер
	logger *slog.Logger // структурированный логгер (никогда не nil)
}

// New конструктор Server, который возвращает сконфигурированный указатель на структуру
func New(handler http.Handler, addr string, logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
		http: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

// Start инициирует запуск работы HTTP-сервера.
// Логгирует запуск.
//
// error — возвращает nil при успешной запуске или или ошибку запуска.
func (s *Server) Start() error {
	s.logger.Info("starting api server", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

// Stop инициирует корректное завершение работы HTTP-сервера.
// Логгирует остановку.
// ctx — контекст для контроля времени ожидания завершения.
//
// error — возвращает nil при успешной остановке всех соединений или ошибку оставновки.
func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("stopping api server")
	return s.http.Shutdown(ctx)
}
