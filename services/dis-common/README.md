# dis-common

Shared Go library for the DIS operators and services in this repository.

Consumers import it as a normal module dependency (`github.com/Altinn/altinn-platform/services/dis-common`) pinned to a pseudo-version, the same way the operators depend on each other. There are no `replace` directives: operator images fetch dependencies from the module proxy.

Packages:

- `platformtags`: the RFC 0007 finops base tag set applied to Azure resources created by the DIS operators (see `rfcs/0007-finops-resource-tags.md`).
- `identityref`: resolves the owner identity a DIS custom resource references in its spec (`identityRef` to an ApplicationIdentity, or `serviceAccountRef` to an annotated ServiceAccount), with shared pending semantics and condition reasons.
