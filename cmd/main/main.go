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
		helpers.LogError("main", "error parsing db config", "error", err.Error())
		os.Exit(1)
	}

	// --- EXISTING SUPABASE COMPATIBILITY SETTINGS ---
	// (Keep these! They are important for Supabase Transaction Mode)
	dbConfig.ConnConfig.StatementCacheCapacity = 0
	// -------------------------------------------------
	dbConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	// --- NEW: STALE CONNECTION FIXES ---
	// 1. HealthCheckPeriod: The pool will ping the DB every 15s to check if
	//    idle connections are still alive. If one is dead, it is removed *before*
	//    your request tries to use it.
	dbConfig.HealthCheckPeriod = 15 * time.Second

	// 2. MaxConnIdleTime: Close connections that haven't been used for 30s.
	//    This prevents holding onto connections that the cloud firewall
	//    is about to cut off anyway.
	dbConfig.MaxConnIdleTime = 30 * time.Second

	// 3. MaxConnLifetime: Recycle connections every 30m to prevent memory bloat/drift.
	dbConfig.MaxConnLifetime = 30 * time.Minute

	// 4. Min/Max Conns (Optional tune for Free Tier)
	dbConfig.MinConns = 0
	// If using Supabase Transaction pooler (port 6543), you can go higher (e.g., 20).
	// If using Direct (port 5432), keep this lower (e.g., 10).
	dbConfig.MaxConns = 10

	// 3. Create the Pool
	pool, err := pgxpool.NewWithConfig(dbCtx, dbConfig)
	if err != nil {
		helpers.LogError("main", "error connecting to db pool", "error", err.Error())
		os.Exit(1)
	}
	// Note: Do NOT defer pool.Close() here if this is inside a function that returns the pool.
	// Only defer Close() in main() if the app is shutting down.
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
			helpers.LogError("main", "Server error", "error", err.Error())
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
		helpers.LogError("main", "Server shutdown failed", "error", err.Error())
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
