package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *Handler) AdminListBooks(w http.ResponseWriter, r *http.Request) {
	books, err := h.store.ListBooks("", "")
	if err != nil {
		jsonErr(w, 500, "failed to list books")
		return
	}
	jsonOK(w, books)
}

func (h *Handler) AddBook(w http.ResponseWriter, r *http.Request) {
	var b struct {
		ISBN        string `json:"isbn"`
		Title       string `json:"title"`
		Author      string `json:"author"`
		Genre       string `json:"genre"`
		Description string `json:"description"`
		Year        int    `json:"published_year"`
		Copies      int    `json:"total_copies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Title == "" || b.Author == "" {
		jsonErr(w, 400, "title and author required")
		return
	}
	if b.Copies < 1 {
		b.Copies = 1
	}
	if err := h.store.CreateBook(uuid.NewString(), b.ISBN, b.Title, b.Author, b.Genre, b.Description, b.Year, b.Copies); err != nil {
		jsonErr(w, 500, "failed to create book")
		return
	}
	jsonOK(w, map[string]string{"message": "book added"})
}

func (h *Handler) UpdateBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var b struct {
		ISBN        string `json:"isbn"`
		Title       string `json:"title"`
		Author      string `json:"author"`
		Genre       string `json:"genre"`
		Description string `json:"description"`
		Year        int    `json:"published_year"`
		Copies      int    `json:"total_copies"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Title == "" || b.Author == "" {
		jsonErr(w, 400, "title and author required")
		return
	}
	if b.Copies < 1 {
		b.Copies = 1
	}
	if err := h.store.UpdateBook(id, b.ISBN, b.Title, b.Author, b.Genre, b.Description, b.Year, b.Copies); err != nil {
		jsonErr(w, 500, "failed to update book")
		return
	}
	jsonOK(w, map[string]string{"message": "book updated"})
}

func (h *Handler) DeleteBook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteBook(id); err != nil {
		jsonErr(w, 500, "failed to delete book")
		return
	}
	jsonOK(w, map[string]string{"message": "book deleted"})
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers()
	if err != nil {
		jsonErr(w, 500, "failed to list users")
		return
	}
	jsonOK(w, users)
}

func (h *Handler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid request")
		return
	}
	valid := map[string]bool{"user": true, "clerk": true, "admin": true}
	if !valid[body.Role] {
		jsonErr(w, 400, "role must be user, clerk, or admin")
		return
	}
	if err := h.store.UpdateUserRole(id, body.Role); err != nil {
		jsonErr(w, 500, "failed to update role")
		return
	}
	jsonOK(w, map[string]string{"message": "role updated"})
}

func (h *Handler) AdminListRequests(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	reqs, err := h.store.ListAllRequests(status)
	if err != nil {
		jsonErr(w, 500, "failed to list requests")
		return
	}
	jsonOK(w, reqs)
}

func (h *Handler) UpdateRequest(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid request")
		return
	}
	valid := map[string]bool{"approved": true, "rejected": true, "pending": true}
	if !valid[body.Status] {
		jsonErr(w, 400, "invalid status")
		return
	}
	if err := h.store.UpdateRequestStatus(id, body.Status); err != nil {
		jsonErr(w, 500, "failed to update request")
		return
	}
	jsonOK(w, map[string]string{"message": "request updated"})
}

func (h *Handler) ListCheckouts(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	checkouts, err := h.store.ListAllCheckouts(activeOnly)
	if err != nil {
		jsonErr(w, 500, "failed to list checkouts")
		return
	}
	jsonOK(w, checkouts)
}

func (h *Handler) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics, err := h.store.GetAnalytics()
	if err != nil {
		jsonErr(w, 500, "failed to compute analytics")
		return
	}
	jsonOK(w, analytics)
}
