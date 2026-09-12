package handler

import (
	"log/slog"

	"github.com/mersikovs/gomart/internal/repository"
	"github.com/mersikovs/gomart/internal/service"
)

type Api struct {
	logger      *slog.Logger
	userService service.UserService
}

func New(repo repository.Storage, jwtSecret string, bCost int, logger *slog.Logger) *Api {

	return &Api{
		logger:      logger,
		userService: service.NewUserService(repo, jwtSecret, bCost),
	}
}
