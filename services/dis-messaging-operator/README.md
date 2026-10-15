# dis-messaging-operator

Kubernetes operator that gives DIS app teams self-service queues on Azure Service Bus.

A team creates a `Queue` custom resource in its product namespace. The operator creates the queue in the shared Service Bus namespace of the cluster and grants the team's DIS application identities the sender or receiver role on it. The operator does not call Azure itself: it creates Azure Service Operator (ASO) resources, as `dis-pgsql-operator` and `dis-vault-operator` do.

See [RFC 0018 - Self-service messaging](../../rfcs/0018-self-service-messaging.md) for the full design.

## Status

This is the `kubebuilder init` scaffold. It has no API and no controller yet. The `Queue` CRD (`messaging.dis.altinn.cloud/v1alpha1`) and the logic that creates the ASO resources land in follow-up pull requests.

## Development

```sh
make setup-local-env   # install tooling into bin/ (once per checkout)
make run-checks-ci-cache   # fmt, generate, manifests, test, lint
```

Unit and envtest suites run with `make test-cache`. See `AGENTS.md` for the required verification steps.
