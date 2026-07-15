package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"trendcortex/api/internal/audit"
	"trendcortex/api/internal/blobstore"
	"trendcortex/api/internal/config"
	"trendcortex/api/internal/database"
	tchttp "trendcortex/api/internal/http"
	"trendcortex/api/internal/platforms"
)

func main() {
	if err := config.LoadDotEnv(); err != nil {
		log.Fatalf("dotenv: %v", err)
	}

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

	mediaStore, err := blobstore.New(context.Background(), blobstore.Config{
		Provider:        cfg.MediaStorageProvider,
		LocalDir:        cfg.MediaStorageLocalDir,
		Endpoint:        cfg.MediaStorageEndpoint,
		Region:          cfg.MediaStorageRegion,
		Bucket:          cfg.MediaStorageBucket,
		AccessKeyID:     cfg.MediaStorageAccessKeyID,
		SecretAccessKey: cfg.MediaStorageSecretAccessKey,
		SessionToken:    cfg.MediaStorageSessionToken,
		ForcePathStyle:  cfg.MediaStorageForcePathStyle,
		Prefix:          cfg.MediaStoragePrefix,
		SignedURLTTL:    cfg.MediaStorageSignedURLTTL,
	})
	if err != nil {
		log.Fatalf("media storage: %v", err)
	}
	if cfg.AppEnv == "production" && mediaStore.Provider() == blobstore.ProviderLocal {
		log.Printf("warning: MEDIA_STORAGE_PROVIDER=local in production; uploaded media is not durable across redeploys")
	}
	log.Printf("media storage: provider=%s", mediaStore.Provider())

	srv := tchttp.NewServerWithMediaStore(cfg, db, registry, al, mediaStore)

	addr := ":" + cfg.Port
	log.Printf("trendcortex-api listening on %s (app base: %s)", addr, cfg.AppBase)
	log.Printf("health check: http://localhost%s/health", addr)

	if err := http.ListenAndServe(addr, srv.Routes()); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
