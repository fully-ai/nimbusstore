package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/fully-ai/nimbusstore/internal/api"
	"github.com/fully-ai/nimbusstore/internal/cas"
	"github.com/fully-ai/nimbusstore/internal/storage"
)

func main() {
	addr := flag.String("addr", envOrDefault("NIMBUS_ADDR", ":8095"), "HTTP listen address")
	dataDir := flag.String("data-dir", envOrDefault("NIMBUS_DATA_DIR", "./data"), "Persistent data directory for CAS blobs and metadata catalog")
	seedDemo := flag.Bool("seed-demo", envOrDefault("NIMBUS_SEED_DEMO", "true") == "true", "Seed demo buckets and deduplicated object versions if catalog is empty")
	flag.Parse()

	casEngine, err := cas.NewEngine(filepath.Join(*dataDir, "cas"), 64*1024*1024)
	if err != nil {
		log.Fatalf("initialize cas engine: %v", err)
	}

	catalog, err := storage.NewCatalog(filepath.Join(*dataDir, "meta"))
	if err != nil {
		log.Fatalf("initialize metadata catalog: %v", err)
	}

	if *seedDemo && len(catalog.ListBuckets()) == 0 {
		seedDemoData(casEngine, catalog)
	}

	srv := api.NewServer(api.ServerConfig{
		AccessKey:     os.Getenv("NIMBUS_ACCESS_KEY"),
		SecretKey:     os.Getenv("NIMBUS_SECRET_KEY"),
		ApiToken:      os.Getenv("NIMBUS_API_TOKEN"),
		SigningSecret: os.Getenv("NIMBUS_SIGNING_SECRET"),
	}, casEngine, catalog)

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           srv,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("NimbusStore v0.1.0 listening on http://localhost%s (data-dir=%s)", *addr, *dataDir)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down NimbusStore gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func seedDemoData(casEngine *cas.Engine, catalog *storage.Catalog) {
	_, _ = catalog.CreateBucket("platform-configs", 50, true, false, 90, "bootstrap")
	_, _ = catalog.CreateBucket("public-assets", 100, true, true, 365, "bootstrap")

	payloadV1 := []byte(`{"cluster":"eu-central-1","replicas":3,"cas_sharding":"sha256"}`)
	payloadV2 := []byte(`{"cluster":"eu-central-1","replicas":5,"cas_sharding":"sha256","scrub_interval_hours":6}`)

	if res, err := casEngine.PutBytes(payloadV1); err == nil {
		_, _ = catalog.PutObjectVersion("platform-configs", "clusters/eu-central.json", res.SHA256, res.Size, "application/json", map[string]string{"env": "prod", "region": "eu"}, res.Deduplicated, "bootstrap")
	}
	if res, err := casEngine.PutBytes(payloadV2); err == nil {
		_, _ = catalog.PutObjectVersion("platform-configs", "clusters/eu-central.json", res.SHA256, res.Size, "application/json", map[string]string{"env": "prod", "region": "eu"}, res.Deduplicated, "bootstrap")
	}
	// Store identical payload in another key to demonstrate cross-object SHA-256 CAS deduplication
	if res, err := casEngine.PutBytes(payloadV2); err == nil {
		_, _ = catalog.PutObjectVersion("platform-configs", "clusters/eu-central-backup.json", res.SHA256, res.Size, "application/json", map[string]string{"env": "dr", "dedup": "true"}, res.Deduplicated, "bootstrap")
	}

	readmeBlob := []byte("# NimbusStore Public Artifact\nContent-addressed object served from public-read bucket.\n")
	if res, err := casEngine.PutBytes(readmeBlob); err == nil {
		_, _ = catalog.PutObjectVersion("public-assets", "docs/WELCOME.md", res.SHA256, res.Size, "text/markdown; charset=utf-8", map[string]string{"visibility": "public"}, res.Deduplicated, "bootstrap")
	}
	_, _ = casEngine.ScrubAll()
}
