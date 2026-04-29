package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"library/internal/middleware"
)

func (h *Handler) ListBooks(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	genre := r.URL.Query().Get("genre")
	books, err := h.store.ListBooks(search, genre)
	if err != nil {
		jsonErr(w, 500, "failed to list books")
		return
	}
	genres, _ := h.store.ListGenres()
	jsonOK(w, map[string]interface{}{"books": books, "genres": genres})
}

func (h *Handler) RequestBook(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	bookID := chi.URLParam(r, "id")
	var body struct {
		Notes string `json:"notes"`
	}
	json.NewDecoder(r.Body).Decode(&body)

	if _, err := h.store.GetBookByID(bookID); err != nil {
		jsonErr(w, 404, "book not found")
		return
	}
	if err := h.store.CreateRequest(uuid.NewString(), claims.UserID, bookID, body.Notes); err != nil {
		jsonErr(w, 409, err.Error())
		return
	}
	jsonOK(w, map[string]string{"message": "request submitted"})
}

func (h *Handler) GetUserRequests(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	reqs, err := h.store.GetUserRequests(claims.UserID)
	if err != nil {
		jsonErr(w, 500, "failed to load requests")
		return
	}
	jsonOK(w, reqs)
}
