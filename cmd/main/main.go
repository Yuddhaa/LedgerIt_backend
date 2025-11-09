package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers" // Make sure helpers is imported
	"LedgerIt/internal/server"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

type config struct {
	port  string
	dbURL string
}

func main() {
	// 1. Open the log file
	logFile, err := os.OpenFile("log.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		// Can't use helpers yet, so just use slog
		slog.Error("Failed to open log file", "error", err)
		os.Exit(1)
	}
	defer logFile.Close()

	//  2. Create a JSON handler that *only* writes to the file.
	fileHandler := slog.NewJSONHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	// 3. Create the logger
	logger := slog.New(fileHandler)

	// 4. Set the default logger and redirect the standard 'log' package
	slog.SetDefault(logger)

	slogAdapter := slog.NewLogLogger(logger.Handler(), slog.LevelInfo)
	log.SetOutput(slogAdapter.Writer())
	log.SetFlags(0)

	// 5. Inject the file-only logger into your helpers package
	helpers.Logger = logger

	// --- From now on, use helpers.LogInfo and helpers.LogError ---
	if err := godotenv.Load(); err != nil {
		// This is not an error in production, it's expected.
		// We'll just log that we're not using a .env file.
		helpers.LogInfo("main", "could not load .env file, using environment variables")
	}
	// if err := godotenv.Load(); err != nil {
	// 	// CHANGED: Switched to helper and used structured error
	// 	helpers.LogError("main", "error in godotenv.Load()", "error", err)
	// 	os.Exit(1)
	// }

	// CHANGED: Removed logger param, it's now global in helpers
	cfg := loadConfig()

	// db connection
	dbCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	dbConn, err := pgx.Connect(dbCtx, cfg.dbURL)
	if err != nil {
		// CHANGED: Switched to helper and used structured error
		helpers.LogError("main", "error in connecting to db", "error", err)
		os.Exit(1)
	}
	db := db.New(dbConn)
	pool, err := pgxpool.New(dbCtx, cfg.dbURL)
	if err != nil {
		// CHANGED: Switched to helper and used structured error
		helpers.LogError("main", "error in connecting to a pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// new server object
	srvr := server.NewServer(db, pool, logger)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.port),
		Handler: srvr.Router,
	}

	go func() {
		// CHANGED: Switched to helper
		logger.Info(" ------------------------------------------------ ")
		helpers.LogInfo("main", "Starting server", "port", cfg.port)
		logger.Info(" ------------------------------------------------ ")

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			// CHANGED: Switched to helper
			helpers.LogError("main", "Server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Handle Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	// CHANGED: Switched to helper
	helpers.LogInfo("main", "Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		// CHANGED: Swwitched to helper
		helpers.LogError("main", "Server shutdown failed", "error", err)
		os.Exit(1)
	}

	// CHANGED: Switched to helper
	helpers.LogInfo("main", "Server exited gracefully")
}

// CHANGED: Removed logger parameter, now uses helpers package directly
func loadConfig() *config {
	cfg := &config{
		port:  os.Getenv("PORT"),
		dbURL: os.Getenv("DBURL"),
	}

	if cfg.port == "" {
		cfg.port = "3000" // Default port
	}

	if cfg.dbURL == "" {
		// CHANGED: Switched to helper
		helpers.LogError("loadConfig", "DBURL must be set in environment variables")
		// Note: You might want to os.Exit(1) here too
	}

	return cfg
}
