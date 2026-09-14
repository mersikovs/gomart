package router

import (
	"log/slog"
	"net/http"

	"github.com/mersikovs/gomart/internal/handler"
	"github.com/mersikovs/gomart/internal/middleware"
)

func Setup(h *handler.Api, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	authMW := middleware.Auth(middleware.AuthConfig{
		SecretKey: []byte(h.JWTSecret),
	})

	mux.Handle("GET /api/user/orders", authMW(http.HandlerFunc(h.ListOrders)))
	mux.HandleFunc("GET /api/user/balance", h.Ping)
	mux.HandleFunc("GET /api/user/withdrawals", h.Ping)

	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)
	mux.Handle("POST /api/user/orders", authMW(http.HandlerFunc(h.RegisterOrder)))
	mux.HandleFunc("POST /api/user/balance/withdraw", h.Ping)

	return mux
}
