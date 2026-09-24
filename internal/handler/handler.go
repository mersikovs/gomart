package handler

import (
	"log/slog"

	"github.com/mersikovs/gomart/internal/accrualclient"
	"github.com/mersikovs/gomart/internal/repository"
	"github.com/mersikovs/gomart/internal/service"
)

// API — основной HTTP-хендлер (контроллер) приложения.
type API struct {
	logger *slog.Logger

	// JWTSecret — секретный ключ для подписи и проверки JWT-токенов авторизации. JWTSecret string
	JWTSecret    string
	userService  service.UserService
	orderService service.OrderService
}

// New — конструктор API. Создает и возвращает сконфигурированный экземпляр API.
func New(repo repository.Storage, client accrualclient.AccrualClient, jwtSecret string, bCost int, logger *slog.Logger) *API {
	return &API{
		logger:       logger,
		JWTSecret:    jwtSecret,
		userService:  service.NewUserService(repo, jwtSecret, bCost),
		orderService: service.NewOrderService(repo, client, logger),
	}
}
