// Command api runs the Pulse REST API server. On boot it applies database
// migrations, then serves HTTP.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aji/pulse/internal/config"
	"github.com/aji/pulse/internal/db"
	"github.com/aji/pulse/internal/httpapi"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Println("running migrations...")
	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	ctx := context.Background()
	store, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer store.Close()

	cookieSecure := os.Getenv("COOKIE_SECURE") == "true"
	srv := httpapi.New(store, cfg, cookieSecure, os.Getenv("ALLOW_ORIGIN"))

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("api listening on %s", cfg.HTTPAddr)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
