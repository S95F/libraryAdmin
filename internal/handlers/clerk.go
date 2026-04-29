package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"library/internal/middleware"
)

func (h *Handler) LookupUser(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("uuid")
	if uid == "" {
		jsonErr(w, 400, "uuid required")
		return
	}
	user, err := h.store.GetUserByID(uid)
	if err != nil {
		jsonErr(w, 404, "user not found")
		return
	}
	checkouts, _ := h.store.GetActiveCheckoutsForUser(user.ID)
	requests, _ := h.store.GetApprovedRequestsForUser(user.ID)

	overdue := 0
	for _, c := range checkouts {
		if c.IsOverdue {
			overdue++
		}
	}

	jsonOK(w, map[string]interface{}{
		"user":              user,
		"active_checkouts":  checkouts,
		"approved_requests": requests,
		"overdue_count":     overdue,
	})
}

func (h *Handler) Checkout(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	var body struct {
		UserID string `json:"user_id"`
		BookID string `json:"book_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.UserID == "" || body.BookID == "" {
		jsonErr(w, 400, "user_id and book_id required")
		return
	}
	due := time.Now().Add(14 * 24 * time.Hour)
	if err := h.store.CreateCheckout(uuid.NewString(), body.UserID, body.BookID, claims.UserID, due); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOK(w, map[string]string{"message": "checkout successful", "due_date": due.Format("2006-01-02")})
}

func (h *Handler) ReturnBook(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CheckoutID string `json:"checkout_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CheckoutID == "" {
		jsonErr(w, 400, "checkout_id required")
		return
	}
	if err := h.store.ReturnBook(body.CheckoutID); err != nil {
		jsonErr(w, 400, err.Error())
		return
	}
	jsonOK(w, map[string]string{"message": "book returned successfully"})
}
