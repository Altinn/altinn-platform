package database

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	// AccessPrincipalsEnv is the single serialized access payload consumed by
	// the user provisioning Job.
	AccessPrincipalsEnv = "DISPG_ACCESS_PRINCIPALS"

	AccessPayloadVersion = 1
)

// Environment variable names that form the contract between the operator
// (producer, in internal/controller) and the user-provisioning Job entrypoint
// (consumer, in this package). Keep both sides referring to these constants
// so a rename can't silently drift the two halves out of sync.
const (
	DatabaseServerNameEnv  = "DISPG_DATABASE_NAME"
	AdminAppIdentityEnv    = "DISPG_ADMIN_APP_IDENTITY"
	DBSchemaEnv            = "DISPG_DB_SCHEMA"
	DBHostEnv              = "DISPG_DB_HOST"
	DBNameEnv              = "DISPG_DB_NAME"
	DBAdminUserEnv         = "DISPG_DB_ADMIN_USER"
	DisableAADEnv          = "DISPG_DISABLE_AAD"
	RevokePublicConnectEnv = "DISPG_REVOKE_PUBLIC_CONNECT"
	DBSearchPathScopeEnv   = "DISPG_DB_SEARCH_PATH_SCOPE"

	// ServerDebugAccessEnv toggles the server-wide debug-access provisioning mode.
	// In this mode the Job grants each principal read-only visibility across every
	// database on the server (one managed NOLOGIN role holding the built-in
	// monitoring roles plus CONNECT on all databases) instead of per-database
	// schema access. The per-principal Role field in the access payload is unused.
	ServerDebugAccessEnv = "DISPG_SERVER_DEBUG_ACCESS"

	// DebugBuiltinRolesEnv carries the comma-separated set of built-in PostgreSQL
	// roles granted to the managed debug role (e.g. "pg_monitor,pg_read_all_data").
	DebugBuiltinRolesEnv = "DISPG_DEBUG_BUILTIN_ROLES"
)

type AccessRole string

const (
	AccessRoleReader AccessRole = "Reader"
	AccessRoleWriter AccessRole = "Writer"
	AccessRoleOwner  AccessRole = "Owner"
)

type PrincipalType string

const (
	PrincipalTypeService PrincipalType = "service"
	PrincipalTypeGroup   PrincipalType = "group"
)

type AccessPrincipal struct {
	Role          AccessRole    `json:"role"`
	Name          string        `json:"name"`
	PrincipalID   string        `json:"principalId,omitempty"`
	PrincipalType PrincipalType `json:"principalType"`
}

type AccessPrincipalsPayload struct {
	Version    int               `json:"version"`
	Principals []AccessPrincipal `json:"principals"`
}

func MarshalAccessPrincipals(principals []AccessPrincipal) (string, error) {
	payload := AccessPrincipalsPayload{
		Version:    AccessPayloadVersion,
		Principals: normalizedAccessPrincipals(principals),
	}
	content, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal access principals payload: %w", err)
	}
	return string(content), nil
}

func ParseAccessPrincipalsPayload(raw string) ([]AccessPrincipal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%s must be set", AccessPrincipalsEnv)
	}

	var payload AccessPrincipalsPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("parse %s: %w", AccessPrincipalsEnv, err)
	}
	if payload.Version != AccessPayloadVersion {
		return nil, fmt.Errorf("%s version %d is not supported", AccessPrincipalsEnv, payload.Version)
	}
	if len(payload.Principals) == 0 {
		return nil, fmt.Errorf("%s must contain at least one principal", AccessPrincipalsEnv)
	}

	return normalizedAccessPrincipals(payload.Principals), nil
}

func normalizedAccessPrincipals(principals []AccessPrincipal) []AccessPrincipal {
	normalized := make([]AccessPrincipal, 0, len(principals))
	for _, principal := range principals {
		principal.Role = AccessRole(strings.TrimSpace(string(principal.Role)))
		principal.Name = strings.TrimSpace(principal.Name)
		principal.PrincipalID = strings.TrimSpace(principal.PrincipalID)
		principal.PrincipalType = PrincipalType(strings.TrimSpace(string(principal.PrincipalType)))
		normalized = append(normalized, principal)
	}

	slices.SortFunc(normalized, func(a, b AccessPrincipal) int {
		for _, cmp := range []int{
			strings.Compare(string(a.Role), string(b.Role)),
			strings.Compare(string(a.PrincipalType), string(b.PrincipalType)),
			strings.Compare(a.Name, b.Name),
			strings.Compare(a.PrincipalID, b.PrincipalID),
		} {
			if cmp != 0 {
				return cmp
			}
		}
		return 0
	})

	return normalized
}

// NormalizeBuiltinRoles parses a comma-separated built-in role list into a
// trimmed, de-duplicated, sorted slice. It is deterministic so the same set
// always yields the same order, keeping the derived managed-role name and Job
// hash stable across reconciles regardless of input ordering or whitespace.
func NormalizeBuiltinRoles(raw string) []string {
	seen := map[string]struct{}{}
	roles := make([]string, 0)
	for part := range strings.SplitSeq(raw, ",") {
		role := strings.TrimSpace(part)
		if role == "" {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	slices.Sort(roles)
	return roles
}
