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
