# dis-cache-operator

Kubernetes operator that gives DIS app teams a self-service cache.

A team creates a `Cache` custom resource in its namespace. The operator creates a Valkey instance in the cluster. The operator does not run Valkey itself: it creates `ValkeyCluster` resources for the official [valkey-operator](https://github.com/valkey-io/valkey-operator), and that operator runs Valkey. This is the same pattern that `dis-pgsql-operator` and `dis-vault-operator` use with Azure Service Operator.

See RFC 0014 (self-service cache) for the full design.

## Status

This is a scaffold. It serves the `Cache` CRD (`cache.dis.altinn.cloud/v1alpha1`) and registers an empty controller. The logic that creates `ValkeyCluster` resources lands in follow-up pull requests.

## Development

```sh
make setup-local-env   # install tooling into bin/ (once per checkout)
make run-checks-ci-cache   # fmt, generate, manifests, test, lint
```

Unit and envtest suites run with `make test-cache`. See `AGENTS.md` for the required verification steps.
