package controller

// Access policy assignment reconciliation lives here in a future PR. The Azure resource type
// `RedisEnterpriseDatabaseAccessPolicyAssignment` is not yet exposed by Azure Service Operator at
// the ASO version pinned for this slice (v2.17.0). When ASO publishes the type in v1api20250401
// (tracked upstream), wire reconciliation here mirroring the dis-vault-operator role assignment
// pattern: deterministic GUID for AzureName, owner-ref to the database, principalId from the
// resolved identity, and label-managed lifecycle for delete-and-recreate replacement.
