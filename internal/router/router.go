// Package router настраивает HTTP-маршруты и связывает их с обработчиками.
package router

import (
	"log/slog"
	"net/http"

	"github.com/mersikovs/gomart/internal/handler"
	"github.com/mersikovs/gomart/internal/middleware"
)

// Setup инициализирует HTTP-маршруты приложения и возвращает handler.
func Setup(h *handler.API, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	authMW := middleware.Auth(middleware.AuthConfig{
		SecretKey: []byte(h.JWTSecret),
	})

	loggerMW := middleware.Logger(logger)

	mux.Handle("GET /api/user/orders", loggerMW(authMW(middleware.GzipResponseMiddleware(http.HandlerFunc(h.ListOrders)))))
	mux.Handle("GET /api/user/balance", loggerMW(authMW(http.HandlerFunc(h.GetBalance))))
	mux.Handle("GET /api/user/withdrawals", loggerMW(authMW(middleware.GzipResponseMiddleware(http.HandlerFunc(h.ListWithdraws)))))

	mux.Handle("POST /api/user/register", loggerMW(http.HandlerFunc(h.Register)))
	mux.Handle("POST /api/user/login", loggerMW(http.HandlerFunc(h.Login)))
	mux.Handle("POST /api/user/orders", authMW(http.HandlerFunc(h.RegisterOrder)))
	mux.Handle("POST /api/user/balance/withdraw", authMW(http.HandlerFunc(h.RegisterWithdraw)))

	return mux
}
