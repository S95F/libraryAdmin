package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"library/internal/auth"
	"library/internal/middleware"
)

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid request")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Email == "" || len(body.Password) < 6 {
		jsonErr(w, 400, "username, email, and password (min 6 chars) required")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		jsonErr(w, 500, "server error")
		return
	}
	id := uuid.NewString()
	if err := h.store.CreateUser(id, body.Username, body.Email, string(hash), "user"); err != nil {
		jsonErr(w, 409, "username or email already taken")
		return
	}
	token, _ := auth.GenerateToken(id, body.Email, "user")
	setTokenCookie(w, token)
	jsonOK(w, map[string]interface{}{"id": id, "username": body.Username, "email": body.Email, "role": "user"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, 400, "invalid request")
		return
	}
	user, err := h.store.GetUserByEmail(strings.ToLower(strings.TrimSpace(body.Email)))
	if err != nil {
		jsonErr(w, 401, "invalid email or password")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		jsonErr(w, 401, "invalid email or password")
		return
	}
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Role)
	setTokenCookie(w, token)
	jsonOK(w, map[string]interface{}{"id": user.ID, "username": user.Username, "email": user.Email, "role": user.Role})
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "token", Value: "", MaxAge: -1, Path: "/"})
	jsonOK(w, nil)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		jsonErr(w, 401, "unauthorized")
		return
	}
	user, err := h.store.GetUserByID(claims.UserID)
	if err != nil {
		jsonErr(w, 404, "user not found")
		return
	}
	jsonOK(w, user)
}

func setTokenCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int((24 * time.Hour).Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}
