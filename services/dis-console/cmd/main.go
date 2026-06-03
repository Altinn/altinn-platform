package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-console/internal/api"
	"github.com/Altinn/altinn-platform/services/dis-console/internal/flux"
)

// sweepTimeout bounds a single Flux sweep so a hung apiserver call cannot stall
// the initial load or freeze the polling loop. It stays below the poll interval.
const sweepTimeout = 20 * time.Second

var (
	httpAddr     = flag.String("http-address", ":8080", "Address for the HTTP API (e.g. :8080)")
	pollInterval = flag.Duration("poll-interval", 30*time.Second, "Flux resource poll interval (e.g. 30s, 1m)")
	local        = flag.Bool("local", false, "Use the local kubeconfig instead of in-cluster config (laptop dev)")
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	if *pollInterval <= 0 {
		log.Fatalf("--poll-interval must be greater than 0, got %s", *pollInterval)
	}

	client, err := flux.NewClient(*local)
	if err != nil {
		log.Fatalf("flux client: %v", err)
	}

	srv := api.NewServer()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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
	poll(ctx, client, srv)

	ticker := time.NewTicker(*pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			gracefulShutdown(httpServer)
			return
		case <-ticker.C:
			poll(ctx, client, srv)
		}
	}
}

func poll(ctx context.Context, client *flux.Client, srv *api.Server) {
	sweepCtx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()

	resources, warnings, err := client.Sweep(sweepCtx)
	for _, w := range warnings {
		log.Printf("sweep warning: %v", w)
	}
	if err != nil {
		log.Printf("sweep failed, keeping previous snapshot: %v", err)
		return
	}
	srv.SetSnapshot(resources, time.Now())
	log.Printf("swept %d Flux resources", len(resources))
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
