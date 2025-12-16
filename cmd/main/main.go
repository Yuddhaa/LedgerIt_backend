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

	"LedgerIt/internal/admin"
	"LedgerIt/internal/db"
	"LedgerIt/internal/helpers"
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

	if err := godotenv.Load(); err != nil {
		helpers.LogInfo("main", "could not load .env file, using environment variables")
	}

	cfg := loadConfig()

	// --- DATABASE CONNECTION LOGIC (FIXED) ---

	// 1. Create a context for the connection attempt
	dbCtx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// 2. Parse the config first so we can modify it
	dbConfig, err := pgxpool.ParseConfig(cfg.dbURL)
	if err != nil {
		helpers.LogError("main", "error parsing db config", "error", err)
		os.Exit(1)
	}

	// --- THE FIX FOR "conn busy" & RACE CONDITIONS ---
	// Disable the implicit statement cache. This prevents the driver from trying
	// to clean up prepared statements on a connection that was just cancelled.
	dbConfig.ConnConfig.StatementCacheCapacity = 0
	// -------------------------------------------------
	dbConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	// 3. Create the Pool (Thread-safe, handles concurrency)
	pool, err := pgxpool.NewWithConfig(dbCtx, dbConfig)
	if err != nil {
		helpers.LogError("main", "error connecting to db pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// 4. Verify connection
	if err := pool.Ping(dbCtx); err != nil {
		helpers.LogError("main", "error pinging db", "error", err)
		os.Exit(1)
	}

	// 5. Initialize sqlc with the POOL
	// Your db.New() accepts DBTX interface, which pgxpool.Pool satisfies.
	// This ensures every query gets its own connection from the pool.
	dbQueries := db.New(pool)

	// --- END DATABASE LOGIC ---

	// new server object
	// Pass dbQueries (backed by pool) and the pool itself
	srvr := server.NewServer(dbQueries, pool, logger)

	httpServer := &http.Server{
		Addr:    fmt.Sprintf(":%v", cfg.port),
		Handler: srvr.Router,
	}
	go admin.Hub.Run()
	go func() {
		logger.Info(" ------------------------------------------------ ")
		helpers.LogInfo("main", "Starting server", "port", cfg.port)
		logger.Info(" ------------------------------------------------ ")

		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			helpers.LogError("main", "Server error", "error", err)
			os.Exit(1)
		}
	}()

	// --- Handle Graceful Shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	helpers.LogInfo("main", "Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		helpers.LogError("main", "Server shutdown failed", "error", err)
		os.Exit(1)
	}

	helpers.LogInfo("main", "Server exited gracefully")
}

func loadConfig() *config {
	cfg := &config{
		port:  os.Getenv("PORT"),
		dbURL: os.Getenv("DBURL"),
	}

	if cfg.port == "" {
		cfg.port = "3000"
	}

	if cfg.dbURL == "" {
		helpers.LogError("loadConfig", "DBURL must be set in environment variables")
		os.Exit(1) // Added Exit here as configuration is mandatory
	}

	return cfg
}
