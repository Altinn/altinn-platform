# Changelog

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
