package config

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

const (
	envSubscriptionID        = "DISMSG_AZURE_SUBSCRIPTION_ID"
	envTenantID              = "DISMSG_AZURE_TENANT_ID"
	envNamespaceResourceID   = "DISMSG_NAMESPACE_RESOURCE_ID"
	envEnvironment           = "DISMSG_ENV"
	envMaxQueuesPerNamespace = "DISMSG_MAX_QUEUES_PER_NAMESPACE"

	defaultMaxQueuesPerNamespace = 50
	serviceBusHostSuffix         = ".servicebus.windows.net"
)

var (
	uuidPattern        = regexp.MustCompile(`^[0-9a-fA-F]{8}(-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}$`)
	namespaceIDPattern = regexp.MustCompile(`(?i)^/subscriptions/([^/]+)/resourceGroups/[^/]+/providers/Microsoft\.ServiceBus/namespaces/([^/]+)$`)
	// Azure allows 6-50 letters, numbers and hyphens in a namespace name. It
	// starts with a letter and ends with a letter or number.
	namespaceNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]{4,48}[A-Za-z0-9]$`)
)

// OperatorConfig is runtime configuration for the messaging operator.
type OperatorConfig struct {
	SubscriptionID string
	TenantID       string
	Environment    string

	// NamespaceResourceID is the ARM ID of the shared Service Bus namespace.
	NamespaceResourceID string
	// NamespaceName is the last segment of NamespaceResourceID.
	NamespaceName string
	// Endpoint is the host name that apps connect to.
	Endpoint string

	// MaxQueuesPerNamespace is the Queue quota per product namespace.
	MaxQueuesPerNamespace int
}

// Load reads the DISMSG_* environment variables with getenv and validates
// them. main passes os.Getenv.
func Load(getenv func(string) string) (*OperatorConfig, error) {
	value := func(key string) string { return strings.TrimSpace(getenv(key)) }

	cfg := &OperatorConfig{
		SubscriptionID:        value(envSubscriptionID),
		TenantID:              value(envTenantID),
		Environment:           value(envEnvironment),
		NamespaceResourceID:   value(envNamespaceResourceID),
		MaxQueuesPerNamespace: defaultMaxQueuesPerNamespace,
	}

	var missing []string
	for _, required := range [][2]string{
		{envSubscriptionID, cfg.SubscriptionID},
		{envTenantID, cfg.TenantID},
		{envNamespaceResourceID, cfg.NamespaceResourceID},
		{envEnvironment, cfg.Environment},
	} {
		if required[1] == "" {
			missing = append(missing, required[0])
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	if !uuidPattern.MatchString(cfg.SubscriptionID) {
		return nil, fmt.Errorf("invalid %s: must be a UUID", envSubscriptionID)
	}
	if !uuidPattern.MatchString(cfg.TenantID) {
		return nil, fmt.Errorf("invalid %s: must be a UUID", envTenantID)
	}

	match := namespaceIDPattern.FindStringSubmatch(cfg.NamespaceResourceID)
	if match == nil {
		return nil, fmt.Errorf("invalid %s %q: must be the ARM ID of a Microsoft.ServiceBus/namespaces resource",
			envNamespaceResourceID, cfg.NamespaceResourceID)
	}
	if !strings.EqualFold(match[1], cfg.SubscriptionID) {
		return nil, fmt.Errorf("invalid %s: the namespace is in subscription %s, not in %s %s",
			envNamespaceResourceID, match[1], envSubscriptionID, cfg.SubscriptionID)
	}
	if !namespaceNamePattern.MatchString(match[2]) {
		return nil, fmt.Errorf("invalid %s: %q is not a valid Service Bus namespace name", envNamespaceResourceID, match[2])
	}
	cfg.NamespaceName = match[2]
	cfg.Endpoint = strings.ToLower(match[2]) + serviceBusHostSuffix

	if raw := value(envMaxQueuesPerNamespace); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 {
			return nil, fmt.Errorf("invalid %s %q: must be a positive integer", envMaxQueuesPerNamespace, raw)
		}
		cfg.MaxQueuesPerNamespace = limit
	}

	return cfg, nil
}
