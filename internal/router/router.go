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
	mux.Handle("GET /api/user/balance", authMW(http.HandlerFunc(h.GetBalance)))
	mux.Handle("GET /api/user/withdrawals", authMW(http.HandlerFunc(h.ListWithdraws)))

	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)
	mux.Handle("POST /api/user/orders", authMW(http.HandlerFunc(h.RegisterOrder)))
	mux.Handle("POST /api/user/balance/withdraw", authMW(http.HandlerFunc(h.RegisterWithdraw)))

	return mux
}
