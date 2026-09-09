package server

import (
	"context"
	"log/slog"
	"net/http"
)

type Server struct {
	http   *http.Server
	logger *slog.Logger
}

func New(handler http.Handler, addr string, logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
		http: &http.Server{
			Addr:    addr,
			Handler: handler,
		},
	}
}

func (s *Server) Start() error {
	s.logger.Info("api сервер запущен", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	s.logger.Info("api сервер остановлен")
	return s.http.Shutdown(ctx)
}
