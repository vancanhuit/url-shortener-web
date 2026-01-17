package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

type Application struct {
	BaseURL string
	Logger  *slog.Logger
	Repo    *Repo
}

var (
	version    = "unknown"
	commitHash = "unknown"
	commitDate = "unknown"
	buildDate  = "unknown"
)

func main() {
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
		fmt.Printf("Built at: %s\n", buildDate)
		os.Exit(0)
	}

	app := &Application{
		BaseURL: baseURL,
		Logger:  slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}

	db, err := OpenDB(dsn)
	if err != nil {
		app.Logger.Error("failed to open database connection pool", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(); err != nil {
			app.Logger.Error("failed to close database connection", "error", err)
		}
	}()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(15 * time.Minute)

	if err := Migrate(db); err != nil {
		app.Logger.Error("failed to run database migrations", "error", err)
		os.Exit(1)
	}

	app.Repo = &Repo{DB: db}

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      app.Router(),
		IdleTimeout:  60 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	app.Logger.Info("starting server", "addr", server.Addr, "base_url", baseURL)
	if !tls {
		err = server.ListenAndServe()
	} else {
		err = server.ListenAndServeTLS(tlsCertFile, tlsKeyFile)
	}

	if err != nil && err != http.ErrServerClosed {
		app.Logger.Error("failed to start server", "error", err)
		os.Exit(1)
	}
}
