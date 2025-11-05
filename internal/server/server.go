package server

import (
	"log/slog"
	"net/http"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/db"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	Router *chi.Mux
	db     *db.Queries
	logger *slog.Logger
}

func NewServer(db *db.Queries, logger *slog.Logger) *Server {
	s := &Server{
		db:     db,
		logger: logger,
	}
	s.setupRouter()
	return s
}

func (s *Server) setupRouter() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	authHandler := auth.NewHandler(s.logger, s.db)

	// Test route
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		s.logger.Handler()
		w.Write([]byte("hello!! Server is up and running"))
	})

	r.Mount("/api/auth/", authHandler.Routes())

	s.Router = r
}
