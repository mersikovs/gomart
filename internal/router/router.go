package router

import (
	"log/slog"
	"net/http"

	"github.com/mersikovs/gomart/internal/handler"
)

func Setup(h *handler.Api, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", h.Ping)
	mux.HandleFunc("GET /api/user/orders", h.Ping)
	mux.HandleFunc("GET /api/user/balance", h.Ping)
	mux.HandleFunc("GET /api/user/withdrawals", h.Ping)

	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Ping)
	mux.HandleFunc("POST /api/user/orders", h.Ping)
	mux.HandleFunc("POST /api/user/balance/withdraw", h.Ping)

	return mux
}
