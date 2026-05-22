/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"crypto/tls"
	"flag"
	"os"

	redisv1alpha1 "github.com/Altinn/altinn-platform/services/dis-cache-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/config"
	"github.com/Altinn/altinn-platform/services/dis-cache-operator/internal/controller"
	identityv1alpha1 "github.com/Altinn/altinn-platform/services/dis-identity-operator/api/v1alpha1"
	cachev1 "github.com/Azure/azure-service-operator/v2/api/cache/v1api20250401"
	pev1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20220701"
	networkv1 "github.com/Azure/azure-service-operator/v2/api/network/v1api20240601"

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(redisv1alpha1.AddToScheme(scheme))
	utilruntime.Must(identityv1alpha1.AddToScheme(scheme))
	utilruntime.Must(cachev1.AddToScheme(scheme))
	utilruntime.Must(networkv1.AddToScheme(scheme))
	utilruntime.Must(pev1.AddToScheme(scheme))
}

// nolint:gocyclo
func main() {
	var (
		metricsAddr                                            string
		metricsCertPath, metricsCertName, metricsCertKey       string
		webhookCertPath, webhookCertName, webhookCertKey       string
		enableLeaderElection                                   bool
		probeAddr                                              string
		secureMetrics                                          bool
		enableHTTP2                                            bool
		subscriptionID, resourceGroup, tenantID, location, env string
		aksSubnetIDs, aksVNetID, dnsZoneResourceGroup          string
		tlsOpts                                                []func(*tls.Config)
	)

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use :8443 for HTTPS, :8080 for HTTP, or 0 to disable.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", true, "If set, the metrics endpoint is served securely via HTTPS.")
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", "The directory that contains the webhook certificate.")
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", "The name of the webhook certificate file.")
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", "The name of the webhook key file.")
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "", "The directory that contains the metrics server certificate.")
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", "The name of the metrics server certificate file.")
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", "The name of the metrics server key file.")
	flag.BoolVar(&enableHTTP2, "enable-http2", false, "If set, HTTP/2 will be enabled for the metrics and webhook servers")

	flag.StringVar(&subscriptionID, "subscription-id", os.Getenv("DISREDIS_AZURE_SUBSCRIPTION_ID"), "Azure subscription ID (required)")
	flag.StringVar(&resourceGroup, "resource-group", os.Getenv("DISREDIS_RESOURCE_GROUP"), "Azure resource group for Redis resources (required)")
	flag.StringVar(&tenantID, "tenant-id", os.Getenv("DISREDIS_AZURE_TENANT_ID"), "Azure tenant ID (required)")
	flag.StringVar(&location, "location", os.Getenv("DISREDIS_LOCATION"), "Azure location for Redis resources (required)")
	flag.StringVar(&env, "env", os.Getenv("DISREDIS_ENV"), "DIS environment identifier (required)")
	flag.StringVar(&aksSubnetIDs, "aks-subnet-ids", os.Getenv("DISREDIS_AKS_SUBNET_IDS"), "Comma-separated AKS subnet ARM IDs (required)")
	flag.StringVar(&aksVNetID, "aks-vnet-id", os.Getenv("DISREDIS_AKS_VNET_ID"), "AKS VNet ARM ID for the shared DNS zone link (required)")
	flag.StringVar(&dnsZoneResourceGroup, "dns-zone-resource-group", os.Getenv("DISREDIS_DNS_ZONE_RESOURCE_GROUP"), "Resource group hosting the shared privatelink.redis.azure.net DNS zone (required)")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServerOptions := webhook.Options{TLSOpts: tlsOpts}
	if len(webhookCertPath) > 0 {
		webhookServerOptions.CertDir = webhookCertPath
		webhookServerOptions.CertName = webhookCertName
		webhookServerOptions.KeyName = webhookCertKey
	}
	webhookServer := webhook.NewServer(webhookServerOptions)

	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}
	if secureMetrics {
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}
	if len(metricsCertPath) > 0 {
		metricsServerOptions.CertDir = metricsCertPath
		metricsServerOptions.CertName = metricsCertName
		metricsServerOptions.KeyName = metricsCertKey
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "redis-operator.dis.altinn.cloud",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	opCfg, err := config.NewOperatorConfig(
		subscriptionID,
		resourceGroup,
		tenantID,
		location,
		env,
		aksSubnetIDs,
		aksVNetID,
		dnsZoneResourceGroup,
	)
	if err != nil {
		setupLog.Error(err, "invalid operator configuration")
		os.Exit(1)
	}

	if err = (&controller.RedisReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Config: *opCfg,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Redis")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
