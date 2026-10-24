# Changelog

## [1.0.6](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.5...dis-apim-v1.0.6) (2026-10-24)


### Bug Fixes

* **deps:** build Go images with golang 1.26.10 ([#4263](https://github.com/Altinn/altinn-platform/issues/4263)) ([de514a3](https://github.com/Altinn/altinn-platform/commit/de514a3cbf7593e42f20eafd4c221b3cc17e0087))
* **deps:** bump grpc, otel and x/net to clear Go security advisories ([#4259](https://github.com/Altinn/altinn-platform/issues/4259)) ([b68ed8b](https://github.com/Altinn/altinn-platform/commit/b68ed8b590bd721fd6f94cf8fbec3ae6c9f82717))
* **deps:** update golang builder images to 1.27.3 ([#4291](https://github.com/Altinn/altinn-platform/issues/4291)) ([63918a5](https://github.com/Altinn/altinn-platform/commit/63918a5ef1b5c7f16e41a80180758319d2c768be))


### Dependency Updates

* **dis-apim:** update services/dis apim operator to kubebuilder 4.16.0 ([#4298](https://github.com/Altinn/altinn-platform/issues/4298)) ([523ab9a](https://github.com/Altinn/altinn-platform/commit/523ab9af6b088cc3f9cc4b5140039c62e0954ef1))
* update actions/checkout digest to d23441a ([#3833](https://github.com/Altinn/altinn-platform/issues/3833)) ([3f65ca1](https://github.com/Altinn/altinn-platform/commit/3f65ca194367d67ec5c315f81a1d37493a60ab77))
* update actions/setup-go digest to 924ae3a ([#3834](https://github.com/Altinn/altinn-platform/issues/3834)) ([c7fcff2](https://github.com/Altinn/altinn-platform/commit/c7fcff2abfc9c1eaa50109d05bf05b1359fc7812))
* update dockerfile non-major dependencies ([#3929](https://github.com/Altinn/altinn-platform/issues/3929)) ([e1a185a](https://github.com/Altinn/altinn-platform/commit/e1a185a220861c8903395b93f048e48a3a5e212c))
* update github-actions non-major dependencies ([#3718](https://github.com/Altinn/altinn-platform/issues/3718)) ([90baeab](https://github.com/Altinn/altinn-platform/commit/90baeab00c1c07fa11d7708a8c0cd0863865f648))
* update golangci/golangci-lint-action digest to ba0d7d2 ([#4272](https://github.com/Altinn/altinn-platform/issues/4272)) ([cff6856](https://github.com/Altinn/altinn-platform/commit/cff685687fb9e0708a1abad105265b0e6bc7be51))
* update module github.com/google/cel-go to v0.29.0 [security] ([#4191](https://github.com/Altinn/altinn-platform/issues/4191)) ([17d13cd](https://github.com/Altinn/altinn-platform/commit/17d13cde807ecd06595055943c669750a4d36f24))

## [1.0.5](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.4...dis-apim-v1.0.5) (2026-05-26)


### Dependency Updates

* update github-actions non-major dependencies ([#3528](https://github.com/Altinn/altinn-platform/issues/3528)) ([a48285f](https://github.com/Altinn/altinn-platform/commit/a48285ff1a562e67cc4ddc4ff1b958d4f01ea426))

## [1.0.4](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.3...dis-apim-v1.0.4) (2026-05-26)


### Bug Fixes

* update golang.org/x/net to v0.55.0 to fix GO-2026-5026 ([#3552](https://github.com/Altinn/altinn-platform/issues/3552)) ([ee5738e](https://github.com/Altinn/altinn-platform/commit/ee5738ea01a69944efa8ba578e8ff6707e758a96))

## [1.0.3](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.2...dis-apim-v1.0.3) (2026-05-26)


### Dependency Updates

* update golangci/golangci-lint-action digest to 82606bf ([#3518](https://github.com/Altinn/altinn-platform/issues/3518)) ([7827eb9](https://github.com/Altinn/altinn-platform/commit/7827eb989caeed6b9dc47cc3988ac67c97ee81a9))

## [1.0.2](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.1...dis-apim-v1.0.2) (2026-05-22)


### Dependency Updates

* update k8s.io/utils digest to ff6756f ([#3396](https://github.com/Altinn/altinn-platform/issues/3396)) ([269c3e5](https://github.com/Altinn/altinn-platform/commit/269c3e5fab8895b12a130e49f5c3523451568f21))

## [1.0.1](https://github.com/Altinn/altinn-platform/compare/dis-apim-v1.0.0...dis-apim-v1.0.1) (2026-05-22)


### Dependency Updates

* update actions/setup-go digest to 4a36011 ([#3191](https://github.com/Altinn/altinn-platform/issues/3191)) ([3c78f2b](https://github.com/Altinn/altinn-platform/commit/3c78f2b53ad80d613b3c2f7ec686ddf9212829ef))
* update dockerfile non-major dependencies ([#3382](https://github.com/Altinn/altinn-platform/issues/3382)) ([f0e15d1](https://github.com/Altinn/altinn-platform/commit/f0e15d1823a539184149f82e0ab15493a393f4a8))
* update gcr.io/distroless/static:nonroot docker digest to 963fa6c ([#3466](https://github.com/Altinn/altinn-platform/issues/3466)) ([a17762f](https://github.com/Altinn/altinn-platform/commit/a17762f295ad24e80eefe0f78f19696d56af0a92))
* update gcr.io/distroless/static:nonroot docker digest to e3f9456 ([#3283](https://github.com/Altinn/altinn-platform/issues/3283)) ([dd7b157](https://github.com/Altinn/altinn-platform/commit/dd7b1578787e084472d1e5b5c6ed8a241afd6cdd))
* update github-actions non-major dependencies ([#3018](https://github.com/Altinn/altinn-platform/issues/3018)) ([100ab8d](https://github.com/Altinn/altinn-platform/commit/100ab8d5d0b9f413983c4b6ca7b5baac6afe51ed))
* update golangci/golangci-lint-action action to v9 ([#3378](https://github.com/Altinn/altinn-platform/issues/3378)) ([7184911](https://github.com/Altinn/altinn-platform/commit/7184911a4fb241f65d7917f3bd04383d6f9aa38f))
* update gomod non-major dependencies ([#3024](https://github.com/Altinn/altinn-platform/issues/3024)) ([0cd0452](https://github.com/Altinn/altinn-platform/commit/0cd0452c9083e202fca6a9fd4a047d5e364cd587))

## 1.0.0 (2026-03-21)


### Features

* **ci:** add auto workflows for kubebuilder base operators ([#2249](https://github.com/Altinn/altinn-platform/issues/2249)) ([9ee2da4](https://github.com/Altinn/altinn-platform/commit/9ee2da44f00caa05ddec640beff6c20e8e40a1f8))


### Bug Fixes

* **deps:** update gomod non-major dependencies ([#2422](https://github.com/Altinn/altinn-platform/issues/2422)) ([678a5e6](https://github.com/Altinn/altinn-platform/commit/678a5e6dbf11758b2d3949dda1162c026242fbd3))
* **deps:** update gomod non-major dependencies ([#2482](https://github.com/Altinn/altinn-platform/issues/2482)) ([ca54fca](https://github.com/Altinn/altinn-platform/commit/ca54fcad483da4a6ed5fb277c59a99a6d0d3c5f7))
* **deps:** update k8s.io/utils digest to bc988d5 ([#2213](https://github.com/Altinn/altinn-platform/issues/2213)) ([29d413d](https://github.com/Altinn/altinn-platform/commit/29d413d648ebc3b2cf2c0ad7754e599c9b7b9a61))
* dis-apim scaffold update: v4.8.0 -&gt; v4.9.0 ([#2665](https://github.com/Altinn/altinn-platform/issues/2665)) ([4c603cb](https://github.com/Altinn/altinn-platform/commit/4c603cb1418917efee4da248d49777739ee041fd))
* **dis-apim-operator:** clear stale resume token on ApiVersion delete ([#3279](https://github.com/Altinn/altinn-platform/issues/3279)) ([06873de](https://github.com/Altinn/altinn-platform/commit/06873def5027d4ae2de7428e6ff0522d97a0ae56))


### Dependency Updates

* bump grpc-GO to 1.79.3 ([#3269](https://github.com/Altinn/altinn-platform/issues/3269)) ([9166356](https://github.com/Altinn/altinn-platform/commit/916635673abdde61b55ebb24addd51ff4b159f85))
* update actions/checkout action to v5 ([#2710](https://github.com/Altinn/altinn-platform/issues/2710)) ([ecb6ba9](https://github.com/Altinn/altinn-platform/commit/ecb6ba9837235c5a07b85ed56cb2988f75205614))
* update actions/checkout action to v6 ([#2716](https://github.com/Altinn/altinn-platform/issues/2716)) ([ebfd5ac](https://github.com/Altinn/altinn-platform/commit/ebfd5ac6befe03fb64fa0539c600d37776bac9d9))
* update actions/checkout action to v6.0.1 ([#2750](https://github.com/Altinn/altinn-platform/issues/2750)) ([874104f](https://github.com/Altinn/altinn-platform/commit/874104ffa4f75053f0abcfd3fe5b711ed8fb10b4))
* update actions/checkout digest to de0fac2 ([#3062](https://github.com/Altinn/altinn-platform/issues/3062)) ([9bc47e2](https://github.com/Altinn/altinn-platform/commit/9bc47e2988c19a2a3d945b6dab755bd9c468b847))
* update actions/setup-go action to v6 ([#2704](https://github.com/Altinn/altinn-platform/issues/2704)) ([8f6502f](https://github.com/Altinn/altinn-platform/commit/8f6502ffd3ecd830ffbd266525c9daaca4a6bd30))
* update actions/setup-go digest to 7a3fe6c ([#2927](https://github.com/Altinn/altinn-platform/issues/2927)) ([bbf1efc](https://github.com/Altinn/altinn-platform/commit/bbf1efcd8b0271820c55bc58d37ed6bf40e882b4))
* update docker.io/golang:1.25.5 docker digest to 8bbd140 ([#2796](https://github.com/Altinn/altinn-platform/issues/2796)) ([0be6bf2](https://github.com/Altinn/altinn-platform/commit/0be6bf2f191006bb8011ebe55071c6796945cafa))
* update dockerfile non-major dependencies ([#2715](https://github.com/Altinn/altinn-platform/issues/2715)) ([8980089](https://github.com/Altinn/altinn-platform/commit/8980089c23bdcccf632ad41480d7e237b138e14b))
* update dockerfile non-major dependencies ([#2989](https://github.com/Altinn/altinn-platform/issues/2989)) ([2989b2b](https://github.com/Altinn/altinn-platform/commit/2989b2bca852ea641a8b6c4a97d4e9b565c67316))
* update dockerfile non-major dependencies ([#3050](https://github.com/Altinn/altinn-platform/issues/3050)) ([78f0a74](https://github.com/Altinn/altinn-platform/commit/78f0a74ec0d91f22fb0802156931a654c3ef57cf))
* update gcr.io/distroless/static:nonroot docker digest to 0376b51 ([#3131](https://github.com/Altinn/altinn-platform/issues/3131)) ([4b1a7bf](https://github.com/Altinn/altinn-platform/commit/4b1a7bff67199a92ebc3ca9506eef388e0b5062d))
* update gcr.io/distroless/static:nonroot docker digest to 2b7c93f ([#2749](https://github.com/Altinn/altinn-platform/issues/2749)) ([9bece8c](https://github.com/Altinn/altinn-platform/commit/9bece8c2bba210de92be2b411a42a26f9a181980))
* update gcr.io/distroless/static:nonroot docker digest to cba10d7 ([#2893](https://github.com/Altinn/altinn-platform/issues/2893)) ([bcd4db3](https://github.com/Altinn/altinn-platform/commit/bcd4db3d5d60b35a78ff113bf030920c573afde7))
* update k8s.io/utils digest to 718f0e5 ([#2828](https://github.com/Altinn/altinn-platform/issues/2828)) ([83bf8f1](https://github.com/Altinn/altinn-platform/commit/83bf8f19eaf0a7731bbc8f63da014e09900bd94a))
* update k8s.io/utils digest to 914a6e7 ([#2894](https://github.com/Altinn/altinn-platform/issues/2894)) ([d4e5014](https://github.com/Altinn/altinn-platform/commit/d4e50145d1892680d25b9075b47bc480081d18b1))
* update services/dis apim operator to kubebuilder 4.11.1 ([#3150](https://github.com/Altinn/altinn-platform/issues/3150)) ([4187300](https://github.com/Altinn/altinn-platform/commit/418730025da5374781fb6c074a27f35b9708c178))
