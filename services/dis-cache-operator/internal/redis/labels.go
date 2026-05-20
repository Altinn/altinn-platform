package redis

const (
	// ManagedResourceOwnerLabel marks a managed resource with the Redis CR name that owns it.
	ManagedResourceOwnerLabel = "redis.dis.altinn.cloud/name"
	// ManagedByLabel marks shared resources (DNS zone, VNet link) as operator-managed.
	ManagedByLabel = "redis.dis.altinn.cloud/managed-by"
	// ManagedByValue is the canonical operator identifier.
	ManagedByValue = "dis-cache-operator"
)
