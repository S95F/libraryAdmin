package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"library/internal/db"
	"library/internal/handlers"
	"library/internal/middleware"
	"library/internal/models"
)

func main() {
	// Load .env before reading any PG* vars
	db.LoadDotEnv(".env")

	// ── --migrate flag ───────────────────────────────────────────────────────
	if len(os.Args) > 1 && os.Args[1] == "--migrate" {
		database := db.Open()
		defer database.Close()
		if err := db.Migrate(database); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("✓ Migration complete.")
		return
	}

	// ── Normal server start ──────────────────────────────────────────────────
	database := db.Open()
	defer database.Close()

	store := models.NewStore(database)
	h := handlers.New(store)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Post("/auth/register", h.Register)
		r.Post("/auth/login", h.Login)
		r.Post("/auth/logout", h.Logout)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Get("/auth/me", h.Me)
			r.Get("/user/qr", h.GetQRCode)
			r.Get("/user/barcode", h.GetBarcode)
			r.Get("/books", h.ListBooks)
			r.Post("/books/{id}/request", h.RequestBook)
			r.Get("/user/requests", h.GetUserRequests)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Use(middleware.RequireRoles("clerk", "admin"))
			r.Get("/clerk/user", h.LookupUser)
			r.Post("/clerk/checkout", h.Checkout)
			r.Post("/clerk/return", h.ReturnBook)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth)
			r.Use(middleware.RequireRoles("admin"))
			r.Get("/admin/analytics", h.GetAnalytics)
			r.Get("/admin/books", h.AdminListBooks)
			r.Post("/admin/books", h.AddBook)
			r.Put("/admin/books/{id}", h.UpdateBook)
			r.Delete("/admin/books/{id}", h.DeleteBook)
			r.Get("/admin/users", h.ListUsers)
			r.Put("/admin/users/{id}/role", h.UpdateUserRole)
			r.Get("/admin/requests", h.AdminListRequests)
			r.Put("/admin/requests/{id}", h.UpdateRequest)
			r.Get("/admin/checkouts", h.ListCheckouts)
		})
	})

	r.Handle("/*", http.FileServer(http.Dir("static")))

	addr := ":" + envOr("PORT", "8080")
	log.Println("╔══════════════════════════════════════════╗")
	log.Println("║      Library Management System           ║")
	log.Printf ("║      http://localhost%s%s║\n", addr, pad(addr))
	log.Println("╚══════════════════════════════════════════╝")
	log.Fatal(http.ListenAndServe(addr, r))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func pad(addr string) string {
	spaces := 26 - len(addr)
	out := ""
	for i := 0; i < spaces; i++ {
		out += " "
	}
	return out
}
