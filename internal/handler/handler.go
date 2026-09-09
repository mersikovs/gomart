package handler

import "log/slog"

type Api struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Api {
	return &Api{logger: logger}
}
