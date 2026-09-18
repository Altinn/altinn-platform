# Changelog

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
