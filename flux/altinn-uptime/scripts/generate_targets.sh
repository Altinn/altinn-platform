#!/bin/bash

# Script to generate and apply blackbox exporter targets and servicemonitors from altinn-orgs.json
# for organizations with tt02 and production environments directly to Kubernetes cluster

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "Error: jq is not installed. Please install jq to continue."
    exit 1
fi

# Check if curl is installed
if ! command -v curl &> /dev/null; then
    echo "Error: curl is not installed. Please install curl to continue."
    exit 1
fi

# Check if kubectl is installed and configured
if ! command -v kubectl &> /dev/null; then
    echo "Error: kubectl is not installed. Please install kubectl to continue."
    exit 1
fi

# Note: kubectl connectivity will be validated when first kubectl command is executed
# Skipping explicit cluster-info check as it requires elevated RBAC permissions

# Input files - use environment variables with defaults
EXTRA_TARGETS_FILE="${EXTRA_TARGETS_FILE:-/config/extra-targets.json}"
MAINTENANCE_TARGETS_FILE="${MAINTENANCE_TARGETS_FILE:-/config/maintenance-targets.json}"
NAMESPACE="${NAMESPACE:-monitoring}"
DRY_RUN="${DRY_RUN:-false}"
OUTPUT_DIR="/tmp/output"

# Create output directory if it doesn't exist
mkdir -p "$OUTPUT_DIR"

# Organizations file URL - use environment variable with default
ORGANIZATIONS_URL="${ORG_DATA_URL:-https://altinncdn.no/orgs/altinn-orgs.json}"
INPUT_FILE="$OUTPUT_DIR/altinn-orgs.json"
# Unique identifier for our ServiceMonitors to avoid conflicts
UNIQUE_ID="${UNIQUE_ID:-altinn-uptime}"

# Create a temporary directory for our work
TEMP_DIR=$(mktemp -d)
# Clean up temporary files on exit
trap 'rm -rf "$TEMP_DIR"' EXIT

# Helper function to download organizations data
download_organizations() {
    echo "Downloading organizations data from $ORGANIZATIONS_URL..."
    if curl -f -s "$ORGANIZATIONS_URL" -o "$INPUT_FILE"; then
        echo "Successfully downloaded organizations data"
        ORGANIZATIONS_JSON=$(cat "$INPUT_FILE")
    else
        echo "Error: Failed to download organizations data from $ORGANIZATIONS_URL"
        exit 1
    fi
}

# Helper function to load configuration from mounted ConfigMaps
load_configuration() {
    # Read configuration from mounted ConfigMap files
    if [ -f "$EXTRA_TARGETS_FILE" ]; then
        EXTRA_TARGETS_JSON=$(cat "$EXTRA_TARGETS_FILE")
        echo "Loaded extra targets configuration"
    else
        echo "Warning: Extra targets file not found at $EXTRA_TARGETS_FILE"
        EXTRA_TARGETS_JSON='{}'
    fi

    if [ -f "$MAINTENANCE_TARGETS_FILE" ]; then
        MAINTENANCE_TARGETS_JSON=$(cat "$MAINTENANCE_TARGETS_FILE")
        echo "Loaded maintenance targets configuration"
    else
        echo "Warning: Maintenance targets file not found at $MAINTENANCE_TARGETS_FILE"
        MAINTENANCE_TARGETS_JSON='{}'
    fi

    # Read maintenance targets from the loaded JSON
    if [ -n "$MAINTENANCE_TARGETS_JSON" ]; then
        MAINTENANCE_IPV4=$(echo "$MAINTENANCE_TARGETS_JSON" | jq -r '.ipv4[]?' 2>/dev/null)
        MAINTENANCE_IPV6=$(echo "$MAINTENANCE_TARGETS_JSON" | jq -r '.ipv6[]?' 2>/dev/null)
        MAINTENANCE_TLS_IPV4=$(echo "$MAINTENANCE_TARGETS_JSON" | jq -r '.tls_ipv4[]?' 2>/dev/null)
        MAINTENANCE_TLS_IPV6=$(echo "$MAINTENANCE_TARGETS_JSON" | jq -r '.tls_ipv6[]?' 2>/dev/null)
    else
        MAINTENANCE_IPV4=""
        MAINTENANCE_IPV6=""
        MAINTENANCE_TLS_IPV4=""
        MAINTENANCE_TLS_IPV6=""
    fi
}

# Helper function to get and filter organizations
get_organizations() {
    # Get organizations with tt02 environment
    if [ -n "$ORGANIZATIONS_JSON" ]; then
        ORGS_WITH_TT02=$(echo "$ORGANIZATIONS_JSON" | jq -r '.orgs | to_entries[] | select(.value.environments[] | contains("tt02")) | .key' | sort | uniq)
    else
        ORGS_WITH_TT02=""
    fi

    # Get organizations with production environment
    if [ -n "$ORGANIZATIONS_JSON" ]; then
        ORGS_WITH_PROD=$(echo "$ORGANIZATIONS_JSON" | jq -r '.orgs | to_entries[] | select(.value.environments[] | contains("production")) | .key' | sort | uniq)
    else
        ORGS_WITH_PROD=""
    fi

    # Filter out organizations that have targets in maintenance
    if [ -n "$MAINTENANCE_IPV4" ] || [ -n "$MAINTENANCE_IPV6" ]; then
        echo "Filtering out organizations with targets in maintenance..."

        # Extract org names from all maintenance URLs using jq
        MAINTENANCE_ORGS=$(echo "$MAINTENANCE_TARGETS_JSON" | jq -r '
            [.ipv4[]?, .ipv6[]?] |
            map(select(length > 0) | capture("https://(?<org>[^.]+)\\..*"; "g").org) |
            unique |
            .[]
        ' 2>/dev/null || echo "")

        # Filter organizations using grep if we have maintenance orgs
        if [ -n "$MAINTENANCE_ORGS" ]; then
            # Build exclude pattern for each maintenance org (anchored to prevent partial matches)
            exclude_pattern=""
            while IFS= read -r maint_org; do
                if [ -n "$maint_org" ]; then
                    exclude_pattern="${exclude_pattern}${maint_org}$|"
                fi
            done <<< "$MAINTENANCE_ORGS"
            exclude_pattern="${exclude_pattern%|}"  # Remove trailing |

            if [ -n "$exclude_pattern" ]; then
                ORGS_WITH_TT02=$(echo "$ORGS_WITH_TT02" | grep -v -E "^($exclude_pattern)$" || echo "$ORGS_WITH_TT02")
                ORGS_WITH_PROD=$(echo "$ORGS_WITH_PROD" | grep -v -E "^($exclude_pattern)$" || echo "$ORGS_WITH_PROD")
            fi
        fi
    fi
}

# Function to apply a ServiceMonitor to Kubernetes
apply_servicemonitor() {
    local yaml_content="$1"
    local name="$2"

    echo "$yaml_content" > "$TEMP_DIR/servicemonitor.yaml"

    if [ "$DRY_RUN" = "true" ]; then
        echo "[DRY RUN] Would apply ServiceMonitor: $name"
        echo "$name" >> "$TEMP_DIR/applied_names"
        return 0
    fi

    if kubectl apply -f "$TEMP_DIR/servicemonitor.yaml" -n "$NAMESPACE" &> /dev/null; then
        echo "Applied ServiceMonitor: $name"
        echo "$name" >> "$TEMP_DIR/applied_names"
        return 0
    else
        echo "Failed to apply ServiceMonitor: $name"
        return 1
    fi
}

# Function to delete a ServiceMonitor from Kubernetes
delete_servicemonitor() {
    local name="$1"

    if [ "$DRY_RUN" = "true" ]; then
        echo "[DRY RUN] Would delete ServiceMonitor: $name"
        return 0
    fi

    if kubectl delete servicemonitor "$name" -n "$NAMESPACE" &> /dev/null; then
        echo "Deleted obsolete ServiceMonitor: $name"
        return 0
    else
        echo "Failed to delete ServiceMonitor: $name"
        return 1
    fi
}

# Execute main workflow
download_organizations
load_configuration
get_organizations

# Initialize applied names file
: > "$TEMP_DIR/applied_names"

# Safety check to prevent deleting all ServiceMonitors if data is missing
if [ -z "$ORGANIZATIONS_JSON" ]; then
    echo "ERROR: Organizations data is empty! Cannot proceed safely."
    exit 1
fi

# Function to generate expected ServiceMonitor names
generate_expected_names() {
    local expected_names=""

    # Organization targets
    for org in $ORGS_WITH_TT02; do
        expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv4"
        expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv6"
    done

    for org in $ORGS_WITH_PROD; do
        expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv4"
        expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv6"
    done

    # Extra targets
    if [ -n "$EXTRA_TARGETS_JSON" ]; then
        # IPv4 extra targets
        IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.ipv4[]?' 2>/dev/null)
        for target in $IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-ipv4"
            fi
        done

        # IPv6 extra targets
        IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.ipv6[]?' 2>/dev/null)
        for target in $IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-ipv6"
            fi
        done

        # Process kuberneteswrapper IPv4 extra targets
        K8S_IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.kuberneteswrapper_ipv4[]?' 2>/dev/null)
        for target in $K8S_IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-k8s-ipv4"
            fi
        done

        # Process kuberneteswrapper IPv6 extra targets
        K8S_IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.kuberneteswrapper_ipv6[]?' 2>/dev/null)
        for target in $K8S_IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-k8s-ipv6"
            fi
        done

        # Process TLS IPv4 extra targets
        TLS_IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.tls_ipv4[]?' 2>/dev/null)
        for target in $TLS_IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_TLS_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-tls-ipv4"
            fi
        done

        # Process TLS IPv6 extra targets
        TLS_IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.tls_ipv6[]?' 2>/dev/null)
        for target in $TLS_IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_TLS_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                expected_names="$expected_names blackbox-exporter-${UNIQUE_ID}-extra-${name}-tls-ipv6"
            fi
        done
    fi

    echo "$expected_names" | tr ' ' '\n' | grep -v '^$' | sort
}

# Get only our ServiceMonitors (with unique ID)
echo "Getting existing ServiceMonitors from cluster..."
existing_names=$(kubectl get servicemonitors -n "$NAMESPACE" -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep "^blackbox-exporter-${UNIQUE_ID}-" | sort)

# Generate expected ServiceMonitor names based on source files
echo "Generating expected ServiceMonitor names based on source files..."
expected_names=$(generate_expected_names)

# Safety check: refuse to proceed if expected names is empty
if [ -z "$expected_names" ]; then
    echo "ERROR: Expected ServiceMonitor names list is empty! This would delete all existing ServiceMonitors."
    echo "Refusing to proceed. Please check the source data files."
    exit 1
fi




# Find ServiceMonitors that need to be added (in expected but not in existing)
echo "$expected_names" > "$TEMP_DIR/expected.txt"
echo "$existing_names" > "$TEMP_DIR/existing.txt"
to_add=$(grep -F -x -v -f "$TEMP_DIR/existing.txt" "$TEMP_DIR/expected.txt")

# Find ServiceMonitors that need to be deleted (in existing but not in expected)
to_delete=$(grep -F -x -v -f "$TEMP_DIR/expected.txt" "$TEMP_DIR/existing.txt")

# Count operations
add_count=$(echo "$to_add" | grep -c . || echo "0")
delete_count=$(echo "$to_delete" | grep -c . || echo "0")

echo "Found $add_count ServiceMonitors to add and $delete_count ServiceMonitors to delete"

# Add new ServiceMonitors
if [ -n "$to_add" ]; then
    echo "Adding new ServiceMonitors..."

    # Generate and apply ServiceMonitors for organizations with tt02 environment
    for org in $ORGS_WITH_TT02; do
        # Check if we need to add IPv4 ServiceMonitor for tt02
        if echo "$to_add" | grep -q "^blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv4$"; then
            cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv4
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: tt02
    altinn.no/organization: ${org}
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv4-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: ${org}.apps.tt02.altinn.no
      targetLabel: instance
    - action: replace
      replacement: ${org}-tt02
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v4
      targetLabel: ip_family
    params:
      hostname:
      - ${org}.apps.tt02.altinn.no
      module:
      - http_2xx_ipv4_kuberneteswrapper
      target:
      - https://${org}.apps.tt02.altinn.no/kuberneteswrapper/api/v1/deployments
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv4-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
            apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv4"
        fi

        # Check if we need to add IPv6 ServiceMonitor for tt02
        if echo "$to_add" | grep -q "^blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv6$"; then
            cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv6
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: tt02
    altinn.no/organization: ${org}
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv6-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: ${org}.apps.tt02.altinn.no
      targetLabel: instance
    - action: replace
      replacement: ${org}-tt02
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v6
      targetLabel: ip_family
    params:
      hostname:
      - ${org}.apps.tt02.altinn.no
      module:
      - http_2xx_ipv6_kuberneteswrapper
      target:
      - https://${org}.apps.tt02.altinn.no/kuberneteswrapper/api/v1/deployments
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv6-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
            apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "blackbox-exporter-${UNIQUE_ID}-${org}-tt02-ipv6"
        fi
    done

    # Generate and apply ServiceMonitors for organizations with production environment
    for org in $ORGS_WITH_PROD; do
        # Check if we need to add IPv4 ServiceMonitor for production
        if echo "$to_add" | grep -q "^blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv4$"; then
            cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv4
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: production
    altinn.no/organization: ${org}
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv4-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: ${org}.apps.altinn.no
      targetLabel: instance
    - action: replace
      replacement: ${org}-prod
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v4
      targetLabel: ip_family
    params:
      hostname:
      - ${org}.apps.altinn.no
      module:
      - http_2xx_ipv4_kuberneteswrapper
      target:
      - https://${org}.apps.altinn.no/kuberneteswrapper/api/v1/deployments
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv4-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
            apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv4"
        fi

        # Check if we need to add IPv6 ServiceMonitor for production
        if echo "$to_add" | grep -q "^blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv6$"; then
            cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv6
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: production
    altinn.no/organization: ${org}
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv6-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: ${org}.apps.altinn.no
      targetLabel: instance
    - action: replace
      replacement: ${org}-prod
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v6
      targetLabel: ip_family
    params:
      hostname:
      - ${org}.apps.altinn.no
      module:
      - http_2xx_ipv6_kuberneteswrapper
      target:
      - https://${org}.apps.altinn.no/kuberneteswrapper/api/v1/deployments
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv6-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
            apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "blackbox-exporter-${UNIQUE_ID}-${org}-prod-ipv6"
        fi
    done

    # Process extra targets
    if [ -n "$EXTRA_TARGETS_JSON" ]; then
        # IPv4 extra targets
        IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.ipv4[]?' 2>/dev/null)
        for target in $IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-ipv4"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv4
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v4
      targetLabel: ip_family
    params:
      module:
      - http_2xx_ipv4
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv4
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done

        # IPv6 extra targets
        IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.ipv6[]?' 2>/dev/null)
        for target in $IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-ipv6"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv6
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v6
      targetLabel: ip_family
    params:
      module:
      - http_2xx_ipv6
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv6
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done

        # Kuberneteswrapper IPv4 extra targets
        K8S_IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.kuberneteswrapper_ipv4[]?' 2>/dev/null)
        for target in $K8S_IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-k8s-ipv4"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv4-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v4
      targetLabel: ip_family
    params:
      hostname:
      - $hostname
      module:
      - http_2xx_ipv4_kuberneteswrapper
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv4-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done

        # Kuberneteswrapper IPv6 extra targets
        K8S_IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.kuberneteswrapper_ipv6[]?' 2>/dev/null)
        for target in $K8S_IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-k8s-ipv6"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv6-kuberneteswrapper
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v6
      targetLabel: ip_family
    params:
      hostname:
      - $hostname
      module:
      - http_2xx_ipv6_kuberneteswrapper
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv6-kuberneteswrapper
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done

        # TLS IPv4 extra targets
        TLS_IPV4_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.tls_ipv4[]?' 2>/dev/null)
        for target in $TLS_IPV4_TARGETS; do
            if ! echo "$MAINTENANCE_TLS_IPV4" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-tls-ipv4"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv4-tls
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}-tls
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v4
      targetLabel: ip_family
    params:
      module:
      - http_alive_tls_ipv4
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv4-tls
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done

        # TLS IPv6 extra targets
        TLS_IPV6_TARGETS=$(echo "$EXTRA_TARGETS_JSON" | jq -r '.tls_ipv6[]?' 2>/dev/null)
        for target in $TLS_IPV6_TARGETS; do
            if ! echo "$MAINTENANCE_TLS_IPV6" | grep -q "^$target$"; then
                hostname=$(echo "$target" | sed -E 's|https?://([^/]+).*|\1|')
                name=$(echo "$hostname" | sed 's/[^a-zA-Z0-9-]/-/g' | tr '[:upper:]' '[:lower:]')
                # Ensure name doesn't exceed k8s limits (253 chars total)
                max_name_len=$((253 - ${#UNIQUE_ID} - 40))  # Reserve space for prefix/suffix
                if [ ${#name} -gt $max_name_len ]; then
                    name="${name:0:$max_name_len}"
                fi
                sm_name="blackbox-exporter-${UNIQUE_ID}-extra-${name}-tls-ipv6"

                cat > "$TEMP_DIR/servicemonitor.yaml" << EOF
---
apiVersion: azmonitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: $sm_name
  namespace: $NAMESPACE
  labels:
    app: prometheus-blackbox-exporter
    release: monitor
    app.kubernetes.io/name: altinn-uptime
    app.kubernetes.io/managed-by: altinn-uptime-sync
    app.kubernetes.io/component: monitoring
    altinn.no/environment: extra
    altinn.no/organization: extra
spec:
  labelLimit: 63
  labelNameLengthLimit: 511
  labelValueLengthLimit: 1023
  endpoints:
  - honorTimestamps: true
    interval: 15s
    scrapeTimeout: 15s
    metricRelabelings:
    - action: replace
      replacement: blackbox-http-ipv6-tls
      targetLabel: job
    - action: replace
      replacement: $hostname
      targetLabel: instance
    - action: replace
      replacement: extra-${name}-tls
      sourceLabels:
      - target
      targetLabel: target
    - action: replace
      replacement: v6
      targetLabel: ip_family
    params:
      module:
      - http_alive_tls_ipv6
      target:
      - $target
    path: /probe
    port: http
    scheme: http
  jobLabel: blackbox-http-ipv6-tls
  namespaceSelector:
    matchNames:
    - $NAMESPACE
  selector:
    matchLabels:
      app.kubernetes.io/instance: monitoring-prometheus-blackbox-exporter
      app.kubernetes.io/name: prometheus-blackbox-exporter
EOF
                apply_servicemonitor "$(cat "$TEMP_DIR/servicemonitor.yaml")" "$sm_name"
            fi
        done
    fi
fi

# Delete obsolete ServiceMonitors
if [ -n "$to_delete" ]; then
    echo "Deleting obsolete ServiceMonitors..."
    while IFS= read -r name; do
        if [ -n "$name" ]; then
            delete_servicemonitor "$name"
        fi
    done <<< "$to_delete"
fi

# Count total ServiceMonitors applied
total_monitors=$(cat "$TEMP_DIR/applied_names" | wc -l)
echo "Total ServiceMonitors applied: $total_monitors"
echo "ServiceMonitor synchronization completed successfully!"
