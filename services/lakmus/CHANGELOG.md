# Changelog

## [1.1.5](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.1.4...lakmus-v1.1.5) (2026-05-26)


### Dependency Updates

* update module github.com/azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault to v2 ([#3565](https://github.com/Altinn/altinn-platform/issues/3565)) ([576d1bb](https://github.com/Altinn/altinn-platform/commit/576d1bbb75a4c1029074b8dbebf11fee115a4f27))

## [1.1.4](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.1.3...lakmus-v1.1.4) (2026-05-26)


### Bug Fixes

* update golang.org/x/net to v0.55.0 to fix GO-2026-5026 ([#3552](https://github.com/Altinn/altinn-platform/issues/3552)) ([ee5738e](https://github.com/Altinn/altinn-platform/commit/ee5738ea01a69944efa8ba578e8ff6707e758a96))

## [1.1.3](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.1.2...lakmus-v1.1.3) (2026-05-22)


### Dependency Updates

* update module github.com/azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault to v2 ([#3536](https://github.com/Altinn/altinn-platform/issues/3536)) ([99216a0](https://github.com/Altinn/altinn-platform/commit/99216a017d8b72a4ff8bd2a8c499c54b42fb2163))

## [1.1.2](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.1.1...lakmus-v1.1.2) (2026-05-22)


### Dependency Updates

* update gomod non-major dependencies ([#3024](https://github.com/Altinn/altinn-platform/issues/3024)) ([0cd0452](https://github.com/Altinn/altinn-platform/commit/0cd0452c9083e202fca6a9fd4a047d5e364cd587))

## [1.1.1](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.1.0...lakmus-v1.1.1) (2026-05-22)


### Bug Fixes

* **lakmus:** align go toolchain to 1.26.1 ([#3280](https://github.com/Altinn/altinn-platform/issues/3280)) ([538767b](https://github.com/Altinn/altinn-platform/commit/538767b23e00ad5e476f2da0a1a31fd055bf8e02))


### Dependency Updates

* pin gcr.io/distroless/static docker tag to 0376b51 ([#3156](https://github.com/Altinn/altinn-platform/issues/3156)) ([bcec942](https://github.com/Altinn/altinn-platform/commit/bcec942accd274699f3dd8dccf50a89f58aa1b11))
* update dockerfile non-major dependencies ([#3382](https://github.com/Altinn/altinn-platform/issues/3382)) ([f0e15d1](https://github.com/Altinn/altinn-platform/commit/f0e15d1823a539184149f82e0ab15493a393f4a8))
* update gcr.io/distroless/static:nonroot docker digest to 963fa6c ([#3466](https://github.com/Altinn/altinn-platform/issues/3466)) ([a17762f](https://github.com/Altinn/altinn-platform/commit/a17762f295ad24e80eefe0f78f19696d56af0a92))
* update gcr.io/distroless/static:nonroot docker digest to e3f9456 ([#3283](https://github.com/Altinn/altinn-platform/issues/3283)) ([dd7b157](https://github.com/Altinn/altinn-platform/commit/dd7b1578787e084472d1e5b5c6ed8a241afd6cdd))

## [1.1.0](https://github.com/Altinn/altinn-platform/compare/lakmus-v1.0.0...lakmus-v1.1.0) (2026-02-26)


### Features

* **lakmus:** update manifests and workflows ([#3130](https://github.com/Altinn/altinn-platform/issues/3130)) ([9982ba1](https://github.com/Altinn/altinn-platform/commit/9982ba12a29cde15153dd60305cdd5d43e350749))


### Bug Fixes

* **deps:** update gomod non-major dependencies ([#2364](https://github.com/Altinn/altinn-platform/issues/2364)) ([aaad71d](https://github.com/Altinn/altinn-platform/commit/aaad71d9dda0cddad3df72e8a56abc9b53d56a0e))
* **deps:** update gomod non-major dependencies ([#2422](https://github.com/Altinn/altinn-platform/issues/2422)) ([678a5e6](https://github.com/Altinn/altinn-platform/commit/678a5e6dbf11758b2d3949dda1162c026242fbd3))
* **deps:** update gomod non-major dependencies ([#2482](https://github.com/Altinn/altinn-platform/issues/2482)) ([ca54fca](https://github.com/Altinn/altinn-platform/commit/ca54fcad483da4a6ed5fb277c59a99a6d0d3c5f7))


### Dependency Updates

* update dockerfile non-major dependencies ([#3050](https://github.com/Altinn/altinn-platform/issues/3050)) ([78f0a74](https://github.com/Altinn/altinn-platform/commit/78f0a74ec0d91f22fb0802156931a654c3ef57cf))
* update gcr.io/distroless/static:nonroot docker digest to 2b7c93f ([#2749](https://github.com/Altinn/altinn-platform/issues/2749)) ([9bece8c](https://github.com/Altinn/altinn-platform/commit/9bece8c2bba210de92be2b411a42a26f9a181980))
* update gcr.io/distroless/static:nonroot docker digest to cba10d7 ([#2893](https://github.com/Altinn/altinn-platform/issues/2893)) ([bcd4db3](https://github.com/Altinn/altinn-platform/commit/bcd4db3d5d60b35a78ff113bf030920c573afde7))
* update golang:1.25-alpine docker digest to 2611181 ([#2728](https://github.com/Altinn/altinn-platform/issues/2728)) ([aabf600](https://github.com/Altinn/altinn-platform/commit/aabf600cba2fd9cb08b5d3d1e0a4a69b76534ac4))
* update golang:1.25-alpine docker digest to ac09a5f ([#2816](https://github.com/Altinn/altinn-platform/issues/2816)) ([42ede50](https://github.com/Altinn/altinn-platform/commit/42ede50f4017eb7bc3556a8ff1e781f4fadcf11e))
* update golang:1.25-alpine docker digest to d9b2e14 ([#2969](https://github.com/Altinn/altinn-platform/issues/2969)) ([6a0d0b7](https://github.com/Altinn/altinn-platform/commit/6a0d0b795dd716d942460b50ea6744182a3c49e0))
