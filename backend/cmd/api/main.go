package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"trendcortex/api/internal/audit"
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
	tchttp "trendcortex/api/internal/http"
	"trendcortex/api/internal/platforms"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v\n\nSet the required environment variables in .env and restart.\nSee .env.example for the full list.", err)
	}

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v\n\nEnsure DATABASE_URL is a valid postgres:// connection string.", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("database migration: %v", err)
	}
	log.Println("database: schema applied")

	registry := platforms.BuildRegistry(cfg)
	al := audit.New(db.DB)

	srv := tchttp.NewServer(cfg, db, registry, al)

	addr := ":" + cfg.Port
	log.Printf("trendcortex-api listening on %s (app base: %s)", addr, cfg.AppBase)
	log.Printf("health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
