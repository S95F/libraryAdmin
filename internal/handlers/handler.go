package handlers

import (
	"encoding/json"
	"net/http"

	"library/internal/models"
)

type Handler struct {
	store *models.Store
}

func New(store *models.Store) *Handler {
	return &Handler{store: store}
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "data": data})
}

func jsonErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": msg})
}
