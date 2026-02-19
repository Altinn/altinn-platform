# Changelog

## [0.2.0](https://github.com/Altinn/altinn-platform/compare/dis-pgsql-v0.1.1...dis-pgsql-v0.2.0) (2026-02-19)


### Features

* **dis-pgsql-operator:** add private dns zone management ([#2767](https://github.com/Altinn/altinn-platform/issues/2767)) ([6c47038](https://github.com/Altinn/altinn-platform/commit/6c47038e4e97ab367ef698e8fa26847def5aac61))
* **dis-pgsql-operator:** create a flexible server when requested ([#2924](https://github.com/Altinn/altinn-platform/issues/2924)) ([ae7c7f9](https://github.com/Altinn/altinn-platform/commit/ae7c7f9479748c42f7e5a84f1cc66fed2aadcc7e))
* **dis-pgsql:** accept dis app identities for users ([#3107](https://github.com/Altinn/altinn-platform/issues/3107)) ([ad8a7f6](https://github.com/Altinn/altinn-platform/commit/ad8a7f61fe1dea98b64d6aaac95ad850ca35868c))
* **dis-pgsql:** add skeleton for dis-pgsql-operator ([#2561](https://github.com/Altinn/altinn-platform/issues/2561)) ([e7e0d3a](https://github.com/Altinn/altinn-platform/commit/e7e0d3a64ae119eee994a2bf6437ffb2c40b5a11))
* **dis-pgsql:** add user to db ([#3104](https://github.com/Altinn/altinn-platform/issues/3104)) ([656cf60](https://github.com/Altinn/altinn-platform/commit/656cf60551033b9b85e5d1fd74a4f58c2a3e4d42))
* **dis-pgsql:** enable db extensions ([#3112](https://github.com/Altinn/altinn-platform/issues/3112)) ([ea1d467](https://github.com/Altinn/altinn-platform/commit/ea1d4674dff9b58973a73e09ae94901409874c3d))
* **dis-pgsql:** support storage spec ([#3110](https://github.com/Altinn/altinn-platform/issues/3110)) ([8c02e8b](https://github.com/Altinn/altinn-platform/commit/8c02e8bf45f7d986b383860799cc90ad750add3e))
* **dis-pgsql:** use workload identity and fix aks vnet role ([#3036](https://github.com/Altinn/altinn-platform/issues/3036)) ([a62dcb2](https://github.com/Altinn/altinn-platform/commit/a62dcb26346f22c00aadee4f46e743ebdd972daa))
* **dis-psql-operator:** add subnet management ([#2628](https://github.com/Altinn/altinn-platform/issues/2628)) ([82cd4e6](https://github.com/Altinn/altinn-platform/commit/82cd4e6ee1b5bed1b6af124b6f6c48b7ba378032))


### Bug Fixes

* **dis-pgsql-operator:** reference armID in private dns zone owner ([#2801](https://github.com/Altinn/altinn-platform/issues/2801)) ([c6f3cbc](https://github.com/Altinn/altinn-platform/commit/c6f3cbcbd8785d99f36521f451f679f7f5c35312))
* **dis-pgsql:** use ARMid instead of rgName for ownership of a db ([#3016](https://github.com/Altinn/altinn-platform/issues/3016)) ([2dbcb69](https://github.com/Altinn/altinn-platform/commit/2dbcb698ae59af0bb04551fb64dd24fdee601fa1))


### Dependency Updates

* update actions/checkout action to v6 ([#2703](https://github.com/Altinn/altinn-platform/issues/2703)) ([6ec1140](https://github.com/Altinn/altinn-platform/commit/6ec11408ee1ef3a753f645043b66ee40d428939f))
* update actions/checkout action to v6.0.1 ([#2750](https://github.com/Altinn/altinn-platform/issues/2750)) ([874104f](https://github.com/Altinn/altinn-platform/commit/874104ffa4f75053f0abcfd3fe5b711ed8fb10b4))
* update actions/setup-go action to v6 ([#2704](https://github.com/Altinn/altinn-platform/issues/2704)) ([8f6502f](https://github.com/Altinn/altinn-platform/commit/8f6502ffd3ecd830ffbd266525c9daaca4a6bd30))
* update dockerfile non-major dependencies ([#3050](https://github.com/Altinn/altinn-platform/issues/3050)) ([78f0a74](https://github.com/Altinn/altinn-platform/commit/78f0a74ec0d91f22fb0802156931a654c3ef57cf))
* update gcr.io/distroless/static:nonroot docker digest to 2b7c93f ([#2749](https://github.com/Altinn/altinn-platform/issues/2749)) ([9bece8c](https://github.com/Altinn/altinn-platform/commit/9bece8c2bba210de92be2b411a42a26f9a181980))
* update gcr.io/distroless/static:nonroot docker digest to cba10d7 ([#2893](https://github.com/Altinn/altinn-platform/issues/2893)) ([bcd4db3](https://github.com/Altinn/altinn-platform/commit/bcd4db3d5d60b35a78ff113bf030920c573afde7))
* update golang:1.25 docker digest to 20b91ed ([#2696](https://github.com/Altinn/altinn-platform/issues/2696)) ([346b0e0](https://github.com/Altinn/altinn-platform/commit/346b0e087a448593e696403ab5c2d525ae7bcea4))
* update golang:1.25 docker digest to ce63a16 ([#2809](https://github.com/Altinn/altinn-platform/issues/2809)) ([06416ea](https://github.com/Altinn/altinn-platform/commit/06416ea20794621f0131dcb8c7cd9cdf6c899fd6))
