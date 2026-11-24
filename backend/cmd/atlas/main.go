package main

import (
	"context"
	"github.com/atlas/atlas/backend/internal/platform/database"
	"github.com/atlas/atlas/backend/internal/transport/httpapi"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx := context.Background()
	dbPath := os.Getenv("ATLAS_DATABASE_URL")
	if dbPath == "" {
		dbPath = "atlas.db"
	}
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		log.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	addr := os.Getenv("ATLAS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	server := &http.Server{Addr: addr, Handler: httpapi.New(db, log), ReadHeaderTimeout: 5 * time.Second}
	log.Info("atlas listening", "address", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
