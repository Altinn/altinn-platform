# Changelog

## [1.7.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.6.0...dis-console-v1.7.0) (2026-07-23)


### Features

* **dis-console:** sweep GitOps-applied workloads and surface container images ([#3836](https://github.com/Altinn/altinn-platform/issues/3836)) ([33d21f2](https://github.com/Altinn/altinn-platform/commit/33d21f2f8fc8e0f29388e1f87a5a3383e8d13785))

## [1.6.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.5.0...dis-console-v1.6.0) (2026-07-15)


### Features

* **dis-console:** add --event-retention to purge aged status events ([#3822](https://github.com/Altinn/altinn-platform/issues/3822)) ([312d067](https://github.com/Altinn/altinn-platform/commit/312d067566a1d72a16c3507bf252354ea8a0a054))
* **dis-console:** track base-layer OCI artifacts and their applied-object inventory ([#3820](https://github.com/Altinn/altinn-platform/issues/3820)) ([b9c7c0e](https://github.com/Altinn/altinn-platform/commit/b9c7c0e44163a12104353015cb355937ef5b2fe5))


### Bug Fixes

* **dis-console:** backfill appliedBy into the central mirror for pre-existing rows ([#3812](https://github.com/Altinn/altinn-platform/issues/3812)) ([f7db0d0](https://github.com/Altinn/altinn-platform/commit/f7db0d0d104fc6380a4bd9f2fe47742a67a42ca0))

## [1.5.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.4.0...dis-console-v1.5.0) (2026-07-03)


### Features

* **dis-console:** project the owning Kustomization (appliedBy) into normalized resources ([#3793](https://github.com/Altinn/altinn-platform/issues/3793)) ([fdddca1](https://github.com/Altinn/altinn-platform/commit/fdddca15c257fd2cb68473d95306276fe1652d83))

## [1.4.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.3.1...dis-console-v1.4.0) (2026-07-03)


### Features

* **dis-console:** sweep DIS custom resources and project azureResourceId + parent ([#3785](https://github.com/Altinn/altinn-platform/issues/3785)) ([39f674d](https://github.com/Altinn/altinn-platform/commit/39f674db9670d9724983ede1fb724bbbcfbee978))

## [1.3.1](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.3.0...dis-console-v1.3.1) (2026-06-24)


### Bug Fixes

* **dis-console:** bump golang.org/x/crypto to v0.52.0+ for 8 HIGH CVEs ([#3753](https://github.com/Altinn/altinn-platform/issues/3753)) ([4701fde](https://github.com/Altinn/altinn-platform/commit/4701fdefa1d2791f4fe8d98f7077f63b4b170704))

## [1.3.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.2.0...dis-console-v1.3.0) (2026-06-23)


### Features

* **dis-console:** status history on the resource detail endpoint ([#3737](https://github.com/Altinn/altinn-platform/issues/3737)) ([937b770](https://github.com/Altinn/altinn-platform/commit/937b770952055fb2bc96658578f6fcf25d0f088d))

## [1.2.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.1.0...dis-console-v1.2.0) (2026-06-18)


### Features

* **dis-console:** agent/server split + write hygiene + typed Flux reader ([#3726](https://github.com/Altinn/altinn-platform/issues/3726)) ([4f3fec2](https://github.com/Altinn/altinn-platform/commit/4f3fec2ce935deaa932e5679f3391e14a68421be))
* **dis-console:** server — central sync engine + fleet API ([#3734](https://github.com/Altinn/altinn-platform/issues/3734)) ([6e37f47](https://github.com/Altinn/altinn-platform/commit/6e37f4711a35c04ca7562752b91b7091a223e730))

## [1.1.0](https://github.com/Altinn/altinn-platform/compare/dis-console-v1.0.0...dis-console-v1.1.0) (2026-06-04)


### Features

* **dis-console:** Postgres persistence (poll-and-store) + Kind e2e ([#3637](https://github.com/Altinn/altinn-platform/issues/3637)) ([9e74e98](https://github.com/Altinn/altinn-platform/commit/9e74e98fe9eda1d3034004eb67a7a911693ace09))

## 1.0.0 (2026-06-03)


### Features

* **dis-console:** read Flux state and serve a JSON API ([#3629](https://github.com/Altinn/altinn-platform/issues/3629)) ([d608a39](https://github.com/Altinn/altinn-platform/commit/d608a39989969146a408fbcfe4d72de7df2038c3))
