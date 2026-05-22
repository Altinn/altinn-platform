# Changelog

## [1.4.6](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.5...dis-vault-v1.4.6) (2026-05-22)


### Dependency Updates

* update github.com/altinn/altinn-platform/services/dis-identity-operator digest to 5189c1d ([#3515](https://github.com/Altinn/altinn-platform/issues/3515)) ([99ceb5a](https://github.com/Altinn/altinn-platform/commit/99ceb5ab342c27f15317ba894b82701a38c329a3))
* update github.com/external-secrets/external-secrets/apis digest to eba2610 ([#3516](https://github.com/Altinn/altinn-platform/issues/3516)) ([34ec763](https://github.com/Altinn/altinn-platform/commit/34ec7631a2297d4cd5537dcdda8b69eb189aacb4))

## [1.4.5](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.4...dis-vault-v1.4.5) (2026-05-22)


### Dependency Updates

* update gomod non-major dependencies ([#3024](https://github.com/Altinn/altinn-platform/issues/3024)) ([0cd0452](https://github.com/Altinn/altinn-platform/commit/0cd0452c9083e202fca6a9fd4a047d5e364cd587))

## [1.4.4](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.3...dis-vault-v1.4.4) (2026-05-22)


### Dependency Updates

* update gcr.io/distroless/static:nonroot docker digest to 963fa6c ([#3466](https://github.com/Altinn/altinn-platform/issues/3466)) ([a17762f](https://github.com/Altinn/altinn-platform/commit/a17762f295ad24e80eefe0f78f19696d56af0a92))

## [1.4.3](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.2...dis-vault-v1.4.3) (2026-05-18)


### Bug Fixes

* **dis-vault-operator:** omit tenant id from managed SecretStore ([#3463](https://github.com/Altinn/altinn-platform/issues/3463)) ([f0637bf](https://github.com/Altinn/altinn-platform/commit/f0637bf3ae366b49edf4ae6f22e9a4c38837117a))


### Dependency Updates

* update gcr.io/distroless/static:nonroot docker digest to e3f9456 ([#3283](https://github.com/Altinn/altinn-platform/issues/3283)) ([dd7b157](https://github.com/Altinn/altinn-platform/commit/dd7b1578787e084472d1e5b5c6ed8a241afd6cdd))

## [1.4.2](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.1...dis-vault-v1.4.2) (2026-04-27)


### Bug Fixes

* **dis-vault:** avoid role assignment guid collisions ([#3386](https://github.com/Altinn/altinn-platform/issues/3386)) ([f2fa9e4](https://github.com/Altinn/altinn-platform/commit/f2fa9e43dbda80e5b149913bdedf888aebddf740))

## [1.4.1](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.4.0...dis-vault-v1.4.1) (2026-04-21)


### Bug Fixes

* **dis-vault:** create configmap before vault uri is ready ([#3350](https://github.com/Altinn/altinn-platform/issues/3350)) ([d751120](https://github.com/Altinn/altinn-platform/commit/d7511207ab024950119d80a54bf14ddb8140ac67))

## [1.4.0](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.3.0...dis-vault-v1.4.0) (2026-04-21)


### Features

* **dis-vault:** support service account-backed vault identities ([#3347](https://github.com/Altinn/altinn-platform/issues/3347)) ([6b70042](https://github.com/Altinn/altinn-platform/commit/6b70042d8c7492cc449e222f7146bc97fbfe9c4b))

## [1.3.0](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.2.1...dis-vault-v1.3.0) (2026-04-20)


### Features

* **dis-vault:** add app-scoped akv configmap ([#3344](https://github.com/Altinn/altinn-platform/issues/3344)) ([01fc691](https://github.com/Altinn/altinn-platform/commit/01fc691df2b7570ff9a988151f93d0faa5c2535d))

## [1.2.1](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.2.0...dis-vault-v1.2.1) (2026-04-15)


### Bug Fixes

* **dis-vault:** replace role assignments on write-once changes ([#3337](https://github.com/Altinn/altinn-platform/issues/3337)) ([16a25d9](https://github.com/Altinn/altinn-platform/commit/16a25d95c6af271f0566a04743e3cbd2bcd871cb))

## [1.2.0](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.1.1...dis-vault-v1.2.0) (2026-04-14)


### Features

* **dis-vault:** add group role assignment for owners of the vault ([#3326](https://github.com/Altinn/altinn-platform/issues/3326)) ([b7a8697](https://github.com/Altinn/altinn-platform/commit/b7a86979b2d22e9de954a8aeed592ce969734431))

## [1.1.1](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.1.0...dis-vault-v1.1.1) (2026-04-10)


### Bug Fixes

* **dis-vault-operator:** inject runtime env vars ([#3315](https://github.com/Altinn/altinn-platform/issues/3315)) ([32a7a71](https://github.com/Altinn/altinn-platform/commit/32a7a7169a862163c8db1c2c8a25fa0b5fa3adc6))

## [1.1.0](https://github.com/Altinn/altinn-platform/compare/dis-vault-v1.0.0...dis-vault-v1.1.0) (2026-04-09)


### Features

* **dis-vault:** add vault status projection and managed SecretStore ([#3311](https://github.com/Altinn/altinn-platform/issues/3311)) ([32dd5ae](https://github.com/Altinn/altinn-platform/commit/32dd5aef0625339cca1bc647a80ddc36ad6fcc2c))

## 1.0.0 (2026-03-27)


### Features

* **dis-vault:** create a vault with initial conditions ([#3243](https://github.com/Altinn/altinn-platform/issues/3243)) ([8582ac7](https://github.com/Altinn/altinn-platform/commit/8582ac74e748dae0732c976948686188d314d26b))
* **dis-vault:** init project ([#3242](https://github.com/Altinn/altinn-platform/issues/3242)) ([9ab5906](https://github.com/Altinn/altinn-platform/commit/9ab5906e1fca77e6175546fd5440aef1594717c6))


### Bug Fixes

* **dis-vault:** ensure reconciles happen for update/delete ([#3267](https://github.com/Altinn/altinn-platform/issues/3267)) ([bd4f5f8](https://github.com/Altinn/altinn-platform/commit/bd4f5f84c472dc3907137106698dec0bee999ee4))


### Dependency Updates

* bump grpc-GO to 1.79.3 ([#3269](https://github.com/Altinn/altinn-platform/issues/3269)) ([9166356](https://github.com/Altinn/altinn-platform/commit/916635673abdde61b55ebb24addd51ff4b159f85))
