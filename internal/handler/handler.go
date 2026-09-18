package handler

import (
	"log/slog"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/repository"
	"github.com/mersikovs/gomart/internal/service"
)

type Api struct {
	logger       *slog.Logger
	JWTSecret    string
	userService  service.UserService
	orderService service.OrderService
}

func New(repo repository.Storage, client *accrualclient.DynHTTPClient, jwtSecret string, bCost int, logger *slog.Logger) *Api {
	return &Api{
		logger:       logger,
		JWTSecret:    jwtSecret,
		userService:  service.NewUserService(repo, jwtSecret, bCost),
		orderService: service.NewOrderService(repo, client, logger),
	}
}
