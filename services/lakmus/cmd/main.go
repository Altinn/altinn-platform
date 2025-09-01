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

	"github.com/Altinn/altinn-platform/services/lakmus/internal/scan"
	"github.com/Altinn/altinn-platform/services/lakmus/test/azfakes"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Command-line flags
	metricsAddr    = flag.String("metrics-address", ":8080", "Address to expose metrics (e.g. :8080)")
	subscriptionID = flag.String("subscription-id", os.Getenv("AZURE_SUBSCRIPTION_ID"), "Azure subscription ID to scan (required)")
	tickInterval   = flag.Duration("tick-interval", time.Hour, "Scan interval (e.g. 10m, 1h)")
	useFakes       = flag.Bool("use-fakes", false, "Use Azure SDK fakes (no network) for KV and Secrets")

	// Prometheus metrics
	reg                = prometheus.NewRegistry()
	secretsExpiryGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "lakmus",
			Subsystem: "keyvault",
			Name:      "secret_expiration_timestamp_seconds",
			Help:      "Unix timestamp (seconds) when an Azure Key Vault secret expires.",
		},
		[]string{"keyvault_name", "secret_name"},
	)

	// Variables for Azure SDK clients
	cred    azcore.TokenCredential
	armOpts *arm.ClientOptions
	secOpts *azsecrets.ClientOptions
	azerr   error
)

func main() {
	log.SetFlags(0)
	flag.Parse()

	// Register metrics
	reg.MustRegister(secretsExpiryGauge)

	// Fail fast if SubscriptionID is not set.
	if *subscriptionID == "" {
		log.Fatalf("AZURE_SUBSCRIPTION_ID env not set or empty")
	}

	// Use fakes if requested (for local dev or testing).
	if *useFakes {
		kvSrv := azfakes.VaultsServerTwoPages()
		secSrv := azfakes.SecretsServerTwoPages()
		env := azfakes.NewEnv(&kvSrv, &secSrv)
		cred, armOpts, secOpts = env.Cred, env.ARM, env.Secrets
		log.Println("running with Azure SDK fakes")
	} else {
		// Use workload identity for Azure authentication.
		// This assumes the environment is set up with Azure AD Workload Identity.
		cred, azerr = azidentity.NewDefaultAzureCredential(nil)
		if azerr != nil {
			log.Fatalf("azure credential error: %v", azerr)
		}
	}

	// create the main ctx to handle shutdown signals in k8s
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// HTTP server for metrics and health.
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	server := &http.Server{
		Addr:    *metricsAddr,
		Handler: mux,
	}

	// start server in a goroutine so we can block on signals below
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// initial scan before starting the ticker
	scanAndUpdateSecretGauges(ctx, *subscriptionID, cred)

	// define a ticker to periodically scan secrets
	ticker := time.NewTicker(*tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			gracefulShutdown(server)
			return
		case <-ticker.C:
			scanAndUpdateSecretGauges(ctx, *subscriptionID, cred)
		}
	}
}

func scanAndUpdateSecretGauges(ctx context.Context, subID string, cred azcore.TokenCredential) {

	// Use a closure to adapt to MetricSetter type and define future labels here.
	set := func(kv, name string, ts float64) {
		secretsExpiryGauge.WithLabelValues(kv, name).Set(ts)
	}

	if err := scan.Scan(ctx, subID, cred, armOpts, secOpts, set); err != nil {
		log.Printf("scan error: %v", err)
	}
}

// Move this function later to internal
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
