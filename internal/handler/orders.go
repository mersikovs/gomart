package handler

import (
	"encoding/json"
	"io"
	"net/http"
)

func (h *Api) RegisterOrder(w http.ResponseWriter, r *http.Request) {

	orderNumber, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read order number", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(string(orderNumber))
}
