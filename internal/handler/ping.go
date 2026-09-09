package handler

import (
	"net/http"
)

func (h *Api) Ping(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`pong`))
}
