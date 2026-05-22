package database

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	ServerParameterAzureExtensions        = "azure.extensions"
	ServerParameterSharedPreloadLibraries = "shared_preload_libraries"
	ServerParameterPgBouncerEnabled       = "pgbouncer.enabled"
	ServerParameterPgBouncerMaxPrepared   = "pgbouncer.max_prepared_statements"
	ServerParameterPgBouncerPoolMode      = "pgbouncer.pool_mode"
	ServerParameterMaxConnections         = "max_connections"
)

const (
	defaultPgBouncerEnabled     = "true"
	defaultPgBouncerMaxPrepared = "5000"
	defaultPgBouncerPoolMode    = "transaction"
)

var nonOverridableServerParameters = map[string]struct{}{
	ServerParameterPgBouncerEnabled:     {},
	ServerParameterPgBouncerMaxPrepared: {},
	ServerParameterPgBouncerPoolMode:    {},
	ServerParameterMaxConnections:       {},
}

var extensionManagedServerParameters = map[string]struct{}{
	ServerParameterAzureExtensions:        {},
	ServerParameterSharedPreloadLibraries: {},
}

type ServerParameter struct {
	Name  string
	Value string
}

func ResolveServerParameters(
	serverType string,
	requested []storagev1alpha1.DatabaseServerParameter,
) ([]ServerParameter, error) {
	profile := GetProfile(serverType)
	maxConnections, err := ResolveMaxConnections(profile)
	if err != nil {
		return nil, err
	}

	resolved := map[string]string{
		ServerParameterPgBouncerEnabled:     defaultPgBouncerEnabled,
		ServerParameterPgBouncerMaxPrepared: defaultPgBouncerMaxPrepared,
		ServerParameterPgBouncerPoolMode:    defaultPgBouncerPoolMode,
		ServerParameterMaxConnections:       strconv.Itoa(maxConnections),
	}

	for i := range requested {
		name := strings.TrimSpace(requested[i].Name)
		if name == "" {
			return nil, fmt.Errorf("serverParams[%d].name must not be empty", i)
		}

		if _, blocked := extensionManagedServerParameters[name]; blocked {
			return nil, fmt.Errorf("server parameter %q is managed by enableExtensions", name)
		}

		if _, blocked := nonOverridableServerParameters[name]; blocked {
			return nil, fmt.Errorf("server parameter %q is managed by the operator and cannot be overridden", name)
		}

		value, err := normalizeServerParameterValue(requested[i].Value)
		if err != nil {
			return nil, fmt.Errorf("serverParams[%d].value: %w", i, err)
		}

		resolved[name] = value
	}

	ordered := make([]ServerParameter, 0, len(resolved))
	for name, value := range resolved {
		ordered = append(ordered, ServerParameter{
			Name:  name,
			Value: value,
		})
	}

	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].Name < ordered[j].Name
	})

	return ordered, nil
}

func normalizeServerParameterValue(value intstr.IntOrString) (string, error) {
	switch value.Type {
	case intstr.Int:
		return strconv.Itoa(value.IntValue()), nil
	case intstr.String:
		trimmed := strings.TrimSpace(value.StrVal)
		if trimmed == "" {
			return "", fmt.Errorf("string value must not be empty")
		}
		return trimmed, nil
	default:
		return "", fmt.Errorf("unsupported value type %q", value.Type)
	}
}
