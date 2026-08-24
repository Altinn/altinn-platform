// Command flux-dispatch receives Flux reconciliation webhooks and turns them
// into GitHub repository_dispatch calls. See RFC 0010 for the design.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/config"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dedup"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/dispatch"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/githubauth"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/metrics"
	"github.com/Altinn/altinn-platform/services/flux-dispatch/internal/server"
)

const (
	// githubTimeout bounds an outbound GitHub call. It sits below the server's
	// 30s WriteTimeout so a slow GitHub surfaces as a 502 rather than a dropped
	// connection.
	githubTimeout = 15 * time.Second
	// shutdownTimeout bounds graceful shutdown on SIGTERM.
	shutdownTimeout = 15 * time.Second
	// maxSweepInterval caps how rarely the dedup tracker is swept.
	maxSweepInterval = time.Hour
	// minSweepInterval keeps a short TTL from busy-looping the sweeper.
	minSweepInterval = time.Minute
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("flux-dispatch exited with an error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	privateKey, err := os.ReadFile(cfg.GitHubPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("read GitHub App private key: %w", err)
	}

	m := metrics.New()
	httpClient := &http.Client{Timeout: githubTimeout}

	app := githubauth.New(cfg.GitHubAppID, cfg.GitHubInstallationID, privateKey, cfg.GitHubAPIURL, httpClient)
	dispatcher := dispatch.New(cfg.GitHubAPIURL, app, httpClient)
	tracker := dedup.New(cfg.DedupTTL, cfg.DedupMaxEntries, m.DedupEntries)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	tracker.StartEviction(ctx, sweepInterval(cfg.DedupTTL))

	srv := server.New(server.Options{
		ListenAddr:           cfg.ListenAddr,
		MetricsAddr:          cfg.MetricsAddr,
		DefaultDispatchEvent: cfg.DefaultDispatchEvent,
		Tracker:              tracker,
		Dispatcher:           dispatcher,
		Metrics:              m,
		Logger:               logger,
	})

	webhook := srv.WebhookServer()
	metricsServer := srv.MetricsServer()

	logger.Info("flux-dispatch starting",
		"listen_addr", cfg.ListenAddr,
		"metrics_addr", cfg.MetricsAddr,
		"github_api_url", cfg.GitHubAPIURL,
		"default_dispatch_event", cfg.DefaultDispatchEvent,
		"dedup_ttl", cfg.DedupTTL.String(),
		"dedup_max_entries", cfg.DedupMaxEntries)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	for name, s := range map[string]*http.Server{"webhook": webhook, "metrics": metricsServer} {
		wg.Add(1)
		go func(name string, s *http.Server) {
			defer wg.Done()
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("%s server: %w", name, err)
			}
		}(name, s)
	}

	var runErr error
	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	for _, s := range []*http.Server{webhook, metricsServer} {
		if err := s.Shutdown(shutdownCtx); err != nil {
			logger.Warn("server shutdown was not clean", "error", err)
		}
	}
	wg.Wait()

	return runErr
}

// sweepInterval keeps the dedup sweep proportional to the TTL, within bounds.
// Expired entries already read as unseen, so the sweep only reclaims memory.
func sweepInterval(ttl time.Duration) time.Duration {
	return min(max(ttl/10, minSweepInterval), maxSweepInterval)
}
