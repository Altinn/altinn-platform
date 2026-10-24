# Changelog

## [0.5.1](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.5.0...dis-cache-v0.5.1) (2026-10-24)


### Bug Fixes

* **deps:** update golang builder images to 1.27.3 ([#4291](https://github.com/Altinn/altinn-platform/issues/4291)) ([63918a5](https://github.com/Altinn/altinn-platform/commit/63918a5ef1b5c7f16e41a80180758319d2c768be))


### Dependency Updates

* update dockerfile non-major dependencies ([#3929](https://github.com/Altinn/altinn-platform/issues/3929)) ([e1a185a](https://github.com/Altinn/altinn-platform/commit/e1a185a220861c8903395b93f048e48a3a5e212c))

## [0.5.0](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.4.0...dis-cache-v0.5.0) (2026-10-22)


### Features

* **dis-cache-operator:** rotate the Cache password on a schedule, every 90 days by default ([#4229](https://github.com/Altinn/altinn-platform/issues/4229)) ([ebc06aa](https://github.com/Altinn/altinn-platform/commit/ebc06aa944e01e6466d2f65774695096cd423541))


### Bug Fixes

* **deps:** build Go images with golang 1.26.10 ([#4263](https://github.com/Altinn/altinn-platform/issues/4263)) ([de514a3](https://github.com/Altinn/altinn-platform/commit/de514a3cbf7593e42f20eafd4c221b3cc17e0087))
* **deps:** bump grpc, otel and x/net to clear Go security advisories ([#4259](https://github.com/Altinn/altinn-platform/issues/4259)) ([b68ed8b](https://github.com/Altinn/altinn-platform/commit/b68ed8b590bd721fd6f94cf8fbec3ae6c9f82717))


### Dependency Updates

* update module github.com/google/cel-go to v0.29.0 [security] ([#4191](https://github.com/Altinn/altinn-platform/issues/4191)) ([17d13cd](https://github.com/Altinn/altinn-platform/commit/17d13cde807ecd06595055943c669750a4d36f24))

## [0.4.0](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.3.0...dis-cache-v0.4.0) (2026-10-15)


### Features

* **dis-cache-operator:** stage a new password and wait for every Valkey node before it replaces the old one ([#4206](https://github.com/Altinn/altinn-platform/issues/4206)) ([b27812c](https://github.com/Altinn/altinn-platform/commit/b27812c041821467bbe07865cabd09bcd39f47ff))


### Dependency Updates

* **dis-cache-operator:** update golang.org/x/net to v0.60.0 and build with Go 1.26.9 ([#4207](https://github.com/Altinn/altinn-platform/issues/4207)) ([f0157a3](https://github.com/Altinn/altinn-platform/commit/f0157a3dfa8df63b43ddb2dd1cc7853f810a5857))

## [0.3.0](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.2.0...dis-cache-v0.3.0) (2026-10-06)


### Features

* **dis-cache-operator:** authorize the metrics scraper by its mesh identity ([#4166](https://github.com/Altinn/altinn-platform/issues/4166)) ([5ad62c2](https://github.com/Altinn/altinn-platform/commit/5ad62c2e5d32ba8245067bce109e105cd27fcb27))
* **dis-cache-operator:** open the exporter port of a Cache to the metrics scraper ([#4163](https://github.com/Altinn/altinn-platform/issues/4163)) ([e12745b](https://github.com/Altinn/altinn-platform/commit/e12745b24c393c8505ed6c8c6ab96470a68f7ec5))

## [0.2.0](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.1.1...dis-cache-v0.2.0) (2026-10-01)


### Features

* **dis-cache-operator:** add the password rotation patches and the two-key user ([#4103](https://github.com/Altinn/altinn-platform/issues/4103)) ([b6989aa](https://github.com/Altinn/altinn-platform/commit/b6989aa1bf3df672b0b376bcb7fcc92f3879abd2))
* **dis-cache-operator:** add the password rotation request and status to the Cache API ([#4100](https://github.com/Altinn/altinn-platform/issues/4100)) ([2f28247](https://github.com/Altinn/altinn-platform/commit/2f28247ca58a516a2e545d52a5ea008002b97daf))
* **dis-cache-operator:** rotate the Cache password on request with a validity period for the old one ([#4155](https://github.com/Altinn/altinn-platform/issues/4155)) ([9f2dbbe](https://github.com/Altinn/altinn-platform/commit/9f2dbbe665f71b67cb9f4aeee1a54f961acaeaaa))

## [0.1.1](https://github.com/Altinn/altinn-platform/compare/dis-cache-v0.1.0...dis-cache-v0.1.1) (2026-09-18)


### Bug Fixes

* **dis-cache-operator:** allow the linkerd inbound port in the Cache NetworkPolicy ([#4097](https://github.com/Altinn/altinn-platform/issues/4097)) ([e6cc16b](https://github.com/Altinn/altinn-platform/commit/e6cc16bae2d4095c45e9534b9d54ec243a42fdf1))

## 0.1.0 (2026-09-14)


### Features

* **dis-cache-operator:** add auth Secret and NetworkPolicy builders ([#4018](https://github.com/Altinn/altinn-platform/issues/4018)) ([3535bba](https://github.com/Altinn/altinn-platform/commit/3535bba3d301df5778ce420f7adf10edd748cc3f))
* **dis-cache-operator:** add linkerd policy builders for the cache ports ([#4030](https://github.com/Altinn/altinn-platform/issues/4030)) ([3e307f8](https://github.com/Altinn/altinn-platform/commit/3e307f8bd4b5537f0d3b6c305283937149a3a28e))
* **dis-cache-operator:** create the auth Secret and the NetworkPolicy for a Cache ([#4037](https://github.com/Altinn/altinn-platform/issues/4037)) ([cc2d766](https://github.com/Altinn/altinn-platform/commit/cc2d766fe7dd2e9d96737a30e965399832728598))
* **dis-cache-operator:** create the linkerd policies for a Cache ([#4046](https://github.com/Altinn/altinn-platform/issues/4046)) ([dcdf491](https://github.com/Altinn/altinn-platform/commit/dcdf4914f6fe8df453212ed02a37eb1d01ed7f20))
* **dis-cache-operator:** map Cache to ValkeyCluster ([#4014](https://github.com/Altinn/altinn-platform/issues/4014)) ([f37cdf8](https://github.com/Altinn/altinn-platform/commit/f37cdf86f11a26e23b161d8e4afdb4a98619170a))
* **dis-cache-operator:** reconcile the ValkeyCluster and mirror its readiness ([#4035](https://github.com/Altinn/altinn-platform/issues/4035)) ([5f61da9](https://github.com/Altinn/altinn-platform/commit/5f61da9281ac0b779404f6f7297629ed17741790))
* **dis-cache-operator:** scaffold Cache CRD and empty controller ([#3949](https://github.com/Altinn/altinn-platform/issues/3949)) ([abfd938](https://github.com/Altinn/altinn-platform/commit/abfd938823c7ea6b7ed3d2708e5298c9f619426a))


### Bug Fixes

* **dis-cache-operator:** address the review findings on [#4035](https://github.com/Altinn/altinn-platform/issues/4035) ([#4038](https://github.com/Altinn/altinn-platform/issues/4038)) ([6cf6db4](https://github.com/Altinn/altinn-platform/commit/6cf6db4fafd6f68ccfc1b8872f74d9baea5ad91e))
* **dis-cache-operator:** lock the default Valkey user and harden the ValkeyCluster spec ([#4029](https://github.com/Altinn/altinn-platform/issues/4029)) ([237a19f](https://github.com/Altinn/altinn-platform/commit/237a19fd9313907e162d574870a63751ba24b6cf))
* **dis-cache-operator:** record step failures in the Ready condition and read back the applied ValkeyCluster ([#4049](https://github.com/Altinn/altinn-platform/issues/4049)) ([7e2ab05](https://github.com/Altinn/altinn-platform/commit/7e2ab05e31341664f5eacc73985d8d355f9a4172))
