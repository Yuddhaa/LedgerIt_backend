package server

import (
	"log/slog"
	"net/http"
	"os"

	"LedgerIt/internal/auth"
	"LedgerIt/internal/business"
	"LedgerIt/internal/categories"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
	"LedgerIt/internal/parties"
	"LedgerIt/internal/transactions"
	"LedgerIt/internal/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Server holds all dependencies for the HTTP server.
type Server struct {
	Router *chi.Mux
	db     *db.Queries
	pool   *pgxpool.Pool
	logger *slog.Logger // The base file logger
}

// NewServer creates a new instance of the Server and sets up its router.
func NewServer(db *db.Queries, pool *pgxpool.Pool, logger *slog.Logger) *Server {
	s := &Server{
		db:     db,
		pool:   pool,
		logger: logger, // CHANGED: Added this line to fix a nil pointer bug.
	}
	s.setupRouter()
	return s
}

// setupRouter initializes all middleware and routes for the server.
func (s *Server) setupRouter() {
	r := chi.NewRouter()

	// --- Core Middleware ---
	// ADDED: RequestID is essential for tracing requests through logs.
	// It MUST come before the logger.
	r.Use(middleware.RequestID)

	// CHANGED: Use your custom helper-based logger
	r.Use(s.SlogLoggerMiddleware)

	r.Use(middleware.Recoverer) // Recovers from panics

	// Apply CORS globally to all handlers
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:8000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// --- Handlers Initialization ---
	// All handlers are initialized with the base slog.Logger.
	// Their internal logging logic will determine what to do.
	authHandler, err := auth.NewHandler(s.db, s.pool, s.logger)
	if err != nil {
		// This is a fatal error during startup, so we log and exit.
		helpers.LogError("setupRouter", "error in intialising authHandler", "err", err.Error())
		os.Exit(1)
	}
	userHandler := users.NewHandler(s.db, s.pool, s.logger)
	businessHandler := business.NewHandler(s.db, s.pool, s.logger)
	partiesHandler := parties.NewHandler(s.db, s.pool)
	categoriesHandler := categories.NewHandler(s.db, s.pool)

	// for transactionshandler
	dbStore := db.NewDBStore(s.pool)
	transactionsHandler := transactions.NewHandler(dbStore)

	// --- Public Routes ---
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		helpers.LogInfo("setupRouter", "server is up and running", "route", "/")
		w.Write([]byte("hello!! Server is up and running"))
	})

	r.Mount("/api/v1/auth/", authHandler.Routes())

	// --- Protected Routes (under JWT Auth) ---
	r.Group(func(r chi.Router) {
		// This middleware protects all routes in this group
		r.Use(authHandler.JwtAuthMiddleware)

		r.Get("/protectedTest", func(w http.ResponseWriter, r *http.Request) {
			helpers.LogInfo("setupRouter", "protected route test successful", "route", "/protectedTest")
			w.Write([]byte("hello!! accessToken is still valid and Server is up and running"))
		})

		r.Mount("/api/v1/users/", userHandler.Routes())
		r.Mount("/api/v1/business/", businessHandler.Routes())
		r.Mount("/api/v1/business/{id}/transactions", transactionsHandler.Routes())
		r.Mount("/api/v1/business/{id}/parties", partiesHandler.Routes())
		r.Mount("/api/v1/business/{id}/categories", categoriesHandler.Routes())
	})

	s.Router = r
}

// SlogLoggerMiddleware creates a basic request logger that uses your
// helpers.LogInfo function to log to both file and console.
func (s *Server) SlogLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the RequestID from the context (set by middleware.RequestID)
		reqID := middleware.GetReqID(r.Context())

		// CHANGED: Use helpers.LogInfo to get both file and console logging.
		helpers.LogInfo("SlogLoggerMiddleware", "incoming request",
			"method", r.Method,
			"path", r.URL.Path,
			"request_id", reqID, // ADDED: Tracing the request ID
		)

		// Pass the request to the next handler
		next.ServeHTTP(w, r)
	})
}

// DELETED: This function is now replaced by SlogLoggerMiddleware
// func (s *Server) SimpleSlogLogger(next http.Handler) http.Handler { ... }
