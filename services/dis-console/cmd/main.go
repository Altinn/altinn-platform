package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/api"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/central"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/dbauth"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/health"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/store"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// version is the agent build, stamped into the tenant DB's meta row so the
// central console can coordinate schema rollout across clusters. Override at
// build time with -ldflags "-X main.version=...".
var version = "dev"

const (
	// sweepTimeout bounds a single Flux sweep so a hung apiserver call cannot
	// stall the initial load or freeze the polling loop.
	sweepTimeout = 20 * time.Second
	// storeTimeout bounds the database write of one sweep's results.
	storeTimeout = 20 * time.Second
	// migrateTimeout bounds the one-shot schema migration at startup.
	migrateTimeout = 30 * time.Second
)

func main() {
	log.SetFlags(0)

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "agent":
		runAgent(os.Args[2:])
	case "server":
		runServer(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "dis-console: unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dis-console — Flux fleet console

Usage:
  dis-console agent [flags]    Sweep this cluster's Flux resources into its own tenant database.
  dis-console server [flags]   Sync tenant databases into the central read model and serve the fleet API.

Run "dis-console <subcommand> -h" for that subcommand's flags.
`)
}

// runAgent runs the per-cluster sweep-and-store loop: every poll interval it
// lists the Flux resources and persists a normalized snapshot to this cluster's
// tenant database. It exposes only liveness/readiness probes — the fleet API
// lives in the server, reading the central database.
func runAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	httpAddr := fs.String("http-address", ":8080", "Address for the health probes (/healthz, /readyz)")
	pollInterval := fs.Duration("poll-interval", 30*time.Second, "Flux resource poll interval (e.g. 30s, 1m)")
	local := fs.Bool("local", false, "Use the local kubeconfig instead of in-cluster config (laptop dev)")
	dbURI := fs.String("db-uri", os.Getenv("DB_URI"),
		"PostgreSQL connection URI without password (default from DB_URI env)")
	dbDisableEntra := fs.Bool("db-disable-entra", envBool("DB_DISABLE_ENTRA"),
		"Skip Entra token auth; use PGPASSWORD or trust auth instead. For Kind/CI/local "+
			"without Azure workload identity (default from DB_DISABLE_ENTRA env)")
	_ = fs.Parse(args)

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

	// Stamp this agent's schema/build into the tenant DB before sweeping.
	initCtx, cancelInit := context.WithTimeout(ctx, migrateTimeout)
	err = st.InitMeta(initCtx, version)
	cancelInit()
	if err != nil {
		log.Fatalf("init meta: %v", err)
	}

	h := health.New(st)
	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           h.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("dis-console agent: health probes on %s", *httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// Initial sweep before starting the ticker so readiness flips quickly.
	poll(ctx, client, st, h)

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			gracefulShutdown(httpServer)
			return
		case <-ticker.C:
			poll(ctx, client, st, h)
		}
	}
}

// runServer runs the central read model: it migrates the central schema, then
// incrementally syncs every tenant database on the shared server into it and
// serves the fleet JSON API (plus health probes) reading only the central DB.
func runServer(args []string) {
	fs := flag.NewFlagSet("server", flag.ExitOnError)
	httpAddr := fs.String("http-address", ":8080", "Address for the fleet API and health probes")
	syncInterval := fs.Duration("sync-interval", 30*time.Second, "Tenant database sync interval (e.g. 30s, 1m)")
	staleAfter := fs.Duration("stale-after", 2*time.Minute,
		"Mark a cluster stale when its agent sweep / console sync is older than this")
	dbURI := fs.String("db-uri", os.Getenv("DB_URI"),
		"Central PostgreSQL connection URI without password (default from DB_URI env). "+
			"Tenant databases are discovered on the same server.")
	dbDisableEntra := fs.Bool("db-disable-entra", envBool("DB_DISABLE_ENTRA"),
		"Skip Entra token auth; use PGPASSWORD or trust auth instead. For Kind/CI/local "+
			"without Azure workload identity (default from DB_DISABLE_ENTRA env)")
	_ = fs.Parse(args)

	if *syncInterval <= 0 {
		log.Fatalf("--sync-interval must be greater than 0, got %s", *syncInterval)
	}
	if *staleAfter <= 0 {
		log.Fatalf("--stale-after must be greater than 0, got %s", *staleAfter)
	}
	if *dbURI == "" {
		log.Fatalf("--db-uri (or DB_URI env) must be set")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var cred azcore.TokenCredential
	if !*dbDisableEntra {
		var err error
		cred, err = azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			log.Fatalf("azure credential: %v", err)
		}
	}

	pool, err := dbauth.NewPool(ctx, *dbURI, cred)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	cs := central.New(pool)
	defer cs.Close()

	migrateCtx, cancel := context.WithTimeout(ctx, migrateTimeout)
	err = cs.Migrate(migrateCtx)
	cancel()
	if err != nil {
		log.Fatalf("migrate central schema: %v", err)
	}
	log.Println("central schema migrated")

	srv := api.NewServer(cs, *staleAfter)
	httpServer := &http.Server{
		Addr:              *httpAddr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		log.Printf("dis-console server: fleet API on %s", *httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// Run the sync loop until shutdown; readiness flips after the first cycle.
	engine := central.NewEngine(cs, *dbURI, cred, *syncInterval)
	engine.Run(ctx, srv.MarkSynced)

	gracefulShutdown(httpServer)
}

func poll(ctx context.Context, client *flux.Client, st *store.Store, h *health.Server) {
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

	h.MarkReady()
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
