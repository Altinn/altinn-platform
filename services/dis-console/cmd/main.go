package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/api"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/dbauth"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	// sweepTimeout bounds a single Flux sweep so a hung apiserver call cannot
	// stall the initial load or freeze the polling loop.
	sweepTimeout = 20 * time.Second
	// storeTimeout bounds the database write of one sweep's results.
	storeTimeout = 20 * time.Second
	// migrateTimeout bounds the one-shot schema migration at startup.
	migrateTimeout = 30 * time.Second
)

var (
	httpAddr     = flag.String("http-address", ":8080", "Address for the HTTP API (e.g. :8080)")
	pollInterval = flag.Duration("poll-interval", 30*time.Second, "Flux resource poll interval (e.g. 30s, 1m)")
	local        = flag.Bool("local", false, "Use the local kubeconfig instead of in-cluster config (laptop dev)")
	dbURI        = flag.String("db-uri", os.Getenv("DB_URI"),
		"PostgreSQL connection URI without password (default from DB_URI env)")
	dbDisableEntra = flag.Bool("db-disable-entra", envBool("DB_DISABLE_ENTRA"),
		"Skip Entra token auth; use PGPASSWORD or trust auth instead. For Kind/CI/local "+
			"without Azure workload identity (default from DB_DISABLE_ENTRA env)")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	if *pollInterval <= 0 {
		log.Fatalf("--poll-interval must be greater than 0, got %s", *pollInterval)
	}
	if *dbURI == "" {
		log.Fatalf("--db-uri (or DB_URI env) must be set")
	}

	client, err := flux.NewClient(*local)
	if err != nil {
		log.Fatalf("flux client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// In the cluster, authenticate to Postgres with a workload-identity Entra
	// token. With --db-disable-entra we skip the credential entirely so dbauth
	// uses PGPASSWORD/trust — the only option in Kind/CI/local.
	var cred azcore.TokenCredential
	if !*dbDisableEntra {
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatalf("azure credential: %v", err)
		}
	}

	pool, err := dbauth.NewPool(ctx, *dbURI, cred)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	st := store.New(pool)
	defer st.Close()

	migrateCtx, cancel := context.WithTimeout(ctx, migrateTimeout)
	err = st.Migrate(migrateCtx)
	cancel()
	if err != nil {
		log.Fatalf("migrate schema: %v", err)
	}
	log.Println("schema migrated")

	srv := api.NewServer(st)

	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("dis-console listening on %s", *httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// Initial sweep before starting the ticker so the API has data quickly.
	poll(ctx, client, st, srv)

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			gracefulShutdown(httpServer)
			return
		case <-ticker.C:
			poll(ctx, client, st, srv)
		}
	}
}

func poll(ctx context.Context, client *flux.Client, st *store.Store, srv *api.Server) {
	sweepCtx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()

	resources, warnings, err := client.Sweep(sweepCtx)
	for _, w := range warnings {
		log.Printf("sweep warning: %v", w)
	}
	if err != nil {
		log.Printf("sweep failed, keeping previous data: %v", err)
		return
	}

	storeCtx, cancel2 := context.WithTimeout(ctx, storeTimeout)
	defer cancel2()
	stats, err := st.Sync(storeCtx, resources)
	if err != nil {
		log.Printf("store sync failed, keeping previous data: %v", err)
		return
	}

	srv.MarkSynced()
	log.Printf("swept %d Flux resources (%d changed, %d pruned)", stats.Upserted, stats.Changed, stats.Pruned)
}

func gracefulShutdown(srv *http.Server) {
	log.Println("shutdown signal received")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	} else {
		log.Println("server shutdown complete")
	}
}

func envBool(key string) bool {
	b, _ := strconv.ParseBool(os.Getenv(key))
	return b
}
