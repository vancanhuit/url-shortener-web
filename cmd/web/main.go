package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Repository interface {
	Insert(ctx context.Context, url, alias string) (string, error)
	GetOriginalURL(ctx context.Context, alias string) (string, error)
}

type Application struct {
	BaseURL string
	Logger  *slog.Logger
	Repo    Repository
}

var (
	version    = "unknown"
	commitHash = "unknown"
	commitDate = "unknown"
	buildDate  = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	var dsn string
	var port int
	var baseURL string
	var tls bool
	var tlsCertFile string
	var tlsKeyFile string
	var displayVersion bool

	flag.StringVar(&dsn, "dsn", os.Getenv("DB_DSN"), "PostgreSQL data source name")
	flag.IntVar(&port, "port", 8080, "HTTP server port")
	flag.StringVar(&baseURL, "base-url", "http://localhost:8080", "Base URL for the application")
	flag.BoolVar(&tls, "tls", false, "Enable TLS")
	flag.StringVar(&tlsCertFile, "tls-cert-file", "./tls/cert.pem", "Path to TLS certificate file")
	flag.StringVar(&tlsKeyFile, "tls-key-file", "./tls/key.pem", "Path to TLS key file")
	flag.BoolVar(&displayVersion, "version", false, "Display version information")
	flag.Parse()

	if displayVersion {
		fmt.Printf("Version: %s\n", version)
		fmt.Printf("Commit hash: %s\n", commitHash)
		fmt.Printf("Commit date: %s\n", commitDate)
		fmt.Printf("Build date: %s\n", buildDate)
		return nil
	}

	app := &Application{
		BaseURL: baseURL,
		Logger:  logger,
	}

	db, err := OpenDB(dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		logger.Info("closing database connection pool")
		if err := db.Close(); err != nil {
			logger.Error("failed to close database connection pool", "error", err)
		}
	}()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(15 * time.Minute)

	if err := Migrate(db); err != nil {
		return fmt.Errorf("run database migrations: %w", err)
	}

	app.Repo = &Repo{DB: db}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      app.Router(),
		IdleTimeout:  60 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", server.Addr, "base_url", baseURL)
		if !tls {
			serverErr <- server.ListenAndServe()
			return
		}
		serverErr <- server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sig)

	select {
	case err = <-serverErr:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error: %w", err)
		}
	case receivedSignal := <-sig:
		logger.Info("shutting down server", "signal", receivedSignal.String())

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("shutdown server: %w", err)
		}

		if err := <-serverErr; err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("server error during shutdown: %w", err)
		}

		logger.Info("server shutdown gracefully")
	}

	return nil
}
