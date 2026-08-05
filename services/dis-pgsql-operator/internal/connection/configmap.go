// Package connection renders the non-secret ConfigMaps that the operator
// publishes so consuming apps can read PostgreSQL connection coordinates.
package connection

import (
	"fmt"
	"strconv"
	"strings"

	storagev1alpha1 "github.com/Altinn/altinn-platform/services/dis-pgsql-operator/api/v1alpha1"
	"github.com/Altinn/altinn-platform/services/dis-pgsql-operator/internal/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configMapSuffix   = "dis-pgsql"
	configMapMaxLen   = 63
	configMapFallback = "db"

	// CNPG-style lowercase data keys.
	DataKeyHost    = "host"
	DataKeyPort    = "port"
	DataKeyDBName  = "dbname"
	DataKeyUser    = "user"
	DataKeySSLMode = "sslmode"
	DataKeyURI     = "uri"

	// SSLModeRequire is the only sslmode the operator publishes; Azure
	// PostgreSQL Flexible Server enforces TLS.
	SSLModeRequire = "require"

	// LabelDatabase, LabelPrincipal and LabelComponent form the binding
	// contract a consumer (or a kro resource graph) selects on.
	LabelDatabase  = "pgsql.dis.altinn.cloud/database"
	LabelPrincipal = "pgsql.dis.altinn.cloud/principal"
	LabelComponent = "pgsql.dis.altinn.cloud/component"
	ComponentValue = "connection"
)

// Coordinates is the resolved, non-secret connection input for one
// service-identity principal's ConfigMap.
type Coordinates struct {
	// Host is the PostgreSQL server FQDN (database.Status.Host).
	Host string
	// Port is the PostgreSQL server port (database.Status.Port).
	Port int32
	// DBName is the PostgreSQL database name (database.Status.DatabaseName).
	DBName string
	// User is the resolved managed-identity name the app authenticates as
	// (ApplicationIdentity.Status.ManagedIdentityName). It may differ from
	// IdentityRef.
	User string
	// IdentityRef is the spec principal identityRef.name. It drives the
	// ConfigMap name and the principal label, and is known at authoring time.
	IdentityRef string
}

// DeterministicConfigMapName returns the ConfigMap name for a database/principal
// pair. It is a pure function of values known before the database is deployed
// (database.metadata.name and identityRef.name), so a consumer can derive it up
// front. The name is "<database>-<identityRef>-dis-pgsql", sanitized to be a
// valid DNS-1123 name and hash-suffixed when it would exceed 63 characters.
func DeterministicConfigMapName(databaseName, identityRefName string) string {
	base := naming.SanitizeLowerHyphen(databaseName + "-" + identityRefName)
	if base == "" {
		base = configMapFallback
	}

	full := base + "-" + configMapSuffix
	if len(full) <= configMapMaxLen {
		return full
	}

	hash := naming.StableSHA256Hex(base)[:8]
	return naming.WithRequiredSuffix(base, "-"+configMapSuffix+"-"+hash, configMapMaxLen, configMapFallback)
}

// BuildConnectionConfigMap renders the desired (un-owned) ConfigMap for one
// principal. The caller is responsible for setting the controller owner
// reference before persisting it.
func BuildConnectionConfigMap(database *storagev1alpha1.Database, coords Coordinates) (*corev1.ConfigMap, error) {
	if database == nil {
		return nil, fmt.Errorf("database must not be nil")
	}
	if strings.TrimSpace(coords.IdentityRef) == "" {
		return nil, fmt.Errorf("coords.IdentityRef must not be empty")
	}

	port := strconv.Itoa(int(coords.Port))
	uri := fmt.Sprintf(
		"postgresql://%s@%s:%s/%s?sslmode=%s",
		coords.User, coords.Host, port, coords.DBName, SSLModeRequire,
	)

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DeterministicConfigMapName(database.Name, coords.IdentityRef),
			Namespace: database.Namespace,
			Labels: map[string]string{
				LabelDatabase:  database.Name,
				LabelPrincipal: coords.IdentityRef,
				LabelComponent: ComponentValue,
			},
		},
		Data: map[string]string{
			DataKeyHost:    coords.Host,
			DataKeyPort:    port,
			DataKeyDBName:  coords.DBName,
			DataKeyUser:    coords.User,
			DataKeySSLMode: SSLModeRequire,
			DataKeyURI:     uri,
		},
	}, nil
}
