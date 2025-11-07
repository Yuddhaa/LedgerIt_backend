package server

import (
	"log/slog"
	"net/http"
	"os"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"
	"LedgerIt/internal/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Server struct {
	Router *chi.Mux
	db     *db.Queries
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewServer(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Server {
	s := &Server{
		db:     db,
		pool:   pool,
		logger: logger,
	}
	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Apply CORS globally to all handlers
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	authHandler, err := auth.NewHandler(s.db, s.pool, s.logger)
	if err != nil {
		s.logger.Error("error in intialising authHandler, err:" + err.Error())
		os.Exit(1)
	}
	userHandler := users.NewHandler(s.db, s.pool, s.logger)

	// ----- public routes -----
	// Test route
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Handler()
		w.Write([]byte("hello!! Server is up and running"))
	})
	r.Mount("/api/v1/auth/", authHandler.Routes())

	// ----- Protected Routes -----
	r.Group(func(r chi.Router) {
		r.Use(authHandler.JwtAuthMiddleware)

		r.Mount("/api/v1/users/", userHandler.Routes())
	})

	s.Router = r
}
