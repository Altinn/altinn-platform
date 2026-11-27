package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/probe"
	"github.com/Altinn/altinn-platform/services/dis-messaging-canary/internal/servicebus"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// shutdownTimeout is the time to stop the HTTP server and to close the
// connection to the Azure Service Bus namespace.
const shutdownTimeout = 5 * time.Second

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := run(ctx, os.Getenv)
	stop()
	if err != nil {
		slog.Error("dis-messaging-canary stopped", "error", err)
		os.Exit(1)
	}
}

// run runs the canary until ctx ends or the metrics server fails. It reads
// the settings with getenv.
func run(ctx context.Context, getenv func(string) string) error {
	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	// The workload identity webhook gives the pod AZURE_CLIENT_ID,
	// AZURE_TENANT_ID and AZURE_FEDERATED_TOKEN_FILE. Without them the canary
	// stops here. It does not try another way to log in.
	credential, err := azidentity.NewWorkloadIdentityCredential(nil)
	if err != nil {
		return fmt.Errorf("create the workload identity credential: %w", err)
	}
	messenger, err := servicebus.New(cfg.Endpoint, cfg.Queue, credential)
	if err != nil {
		return err
	}
	// The registry holds only the metrics of the canary. Every series on
	// /metrics goes to the metrics store, so there are no Go runtime metrics.
	registry := prometheus.NewRegistry()
	prober := probe.New(messenger, probe.NewMetrics(registry), cfg.Interval, cfg.Timeout)

	listener, err := net.Listen("tcp", cfg.MetricsAddr)
	if err != nil {
		return fmt.Errorf("CANARY_METRICS_ADDR: %w", err)
	}
	server := &http.Server{Handler: handler(registry), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := context.WithCancel(ctx)
	defer stop()
	// Serve returns when the server shuts down or when it fails. The canary is
	// of no use without its metrics, so a failure stops the canary.
	served := make(chan error, 1)
	go func() {
		served <- server.Serve(listener)
		stop()
	}()

	slog.Info("dis-messaging-canary started", "endpoint", cfg.Endpoint, "queue", cfg.Queue,
		"interval", cfg.Interval.String(), "timeout", cfg.Timeout.String(), "metricsAddr", cfg.MetricsAddr)
	prober.Run(ctx)

	slog.Info("dis-messaging-canary stops")
	closeCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	err = errors.Join(server.Shutdown(closeCtx), messenger.Close(closeCtx))
	if serveErr := <-served; !errors.Is(serveErr, http.ErrServerClosed) {
		err = errors.Join(fmt.Errorf("the metrics server failed: %w", serveErr), err)
	}
	return err
}

// handler serves the metrics of registry and the liveness probe.
func handler(registry *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}
