package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/server"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

type config struct {
	port  string
	dbURL string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := godotenv.Load(); err != nil {
		logger.Error("error in godotenv.Load(),err:" + err.Error())
		os.Exit(1)
	}
	cfg := loadConfig(logger)

	// db connection
	dbCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	dbConn, err := pgx.Connect(dbCtx, cfg.dbURL)
	if err != nil {
		logger.Error("error in connecting to db, err:" + err.Error())
	}
	db := db.New(dbConn)

	// new server object
	srvr := server.NewServer(db)

	// We run this in a goroutine so it doesn't block the graceful shutdown logic
	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.port),
		Handler: srvr.Router, // srv.Router is the main chi.Mux
	}

	go func() {
		logger.Info("Starting server", "port", cfg.port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Handle Graceful Shutdown ---
	// Wait for an interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Create a context with a timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Server exited gracefully")
}

func loadConfig(logger *slog.Logger) *config {
	cfg := &config{
		port:  os.Getenv("PORT"),
		dbURL: os.Getenv("DBURL"),
	}

	if cfg.port == "" {
		cfg.port = "8080" // Default port
	}

	if cfg.dbURL == "" {
		logger.Error("DBURL must be set in environment variables")
	}

	return cfg
}
