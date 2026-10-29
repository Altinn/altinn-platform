# Changelog

## 0.1.0 (2026-10-27)


### Features

* **dis-messaging-operator:** add the Queue CRD and an empty controller ([#4212](https://github.com/Altinn/altinn-platform/issues/4212)) ([771da6b](https://github.com/Altinn/altinn-platform/commit/771da6bb80171e248d57537c978f2e4ab0c9cc30))
* **dis-messaging-operator:** build the ASO RoleAssignments that grant roles on a queue ([#4274](https://github.com/Altinn/altinn-platform/issues/4274)) ([bc3e25b](https://github.com/Altinn/altinn-platform/commit/bc3e25b8f88bb3d74edef362cf3fcdd55100f510))
* **dis-messaging-operator:** build the dis-messaging ConfigMap and ResourceQuota of a namespace ([#4280](https://github.com/Altinn/altinn-platform/issues/4280)) ([13bad38](https://github.com/Altinn/altinn-platform/commit/13bad3850ca1b72eb10162fe43f60c03eedaeb8e))
* **dis-messaging-operator:** create the Azure queue of a Queue through ASO and report its state ([#4304](https://github.com/Altinn/altinn-platform/issues/4304)) ([dada789](https://github.com/Altinn/altinn-platform/commit/dada78912081d766ee8b9e869897b342d8b47ead))
* **dis-messaging-operator:** grant group and service principals their roles on the queue and report AccessReady ([#4316](https://github.com/Altinn/altinn-platform/issues/4316)) ([809bafb](https://github.com/Altinn/altinn-platform/commit/809bafbd604389e18b6aa17a0167bfa1edd84cdd))
* **dis-messaging-operator:** grant identityRef principals their roles on the queue ([#4318](https://github.com/Altinn/altinn-platform/issues/4318)) ([a98210b](https://github.com/Altinn/altinn-platform/commit/a98210bb8ef3c3eb2fe899acfef7f9669d6eb62b))
* **dis-messaging-operator:** init project with the kubebuilder scaffold ([#4210](https://github.com/Altinn/altinn-platform/issues/4210)) ([5ec9ea4](https://github.com/Altinn/altinn-platform/commit/5ec9ea4ae526c0ee98769585a808dc13e1f482cc))
* **dis-messaging-operator:** limit the Queues of each product namespace with a ResourceQuota ([#4324](https://github.com/Altinn/altinn-platform/issues/4324)) ([be2a892](https://github.com/Altinn/altinn-platform/commit/be2a892fb512e1a99c12a8960134bdf2e83048a9))
* **dis-messaging-operator:** map a Queue to an ASO NamespacesQueue ([#4267](https://github.com/Altinn/altinn-platform/issues/4267)) ([e7ea879](https://github.com/Altinn/altinn-platform/commit/e7ea879a992260aa2b82355889f19e20720c5d6e))
* **dis-messaging-operator:** reconcile Queues again when the namespace prefix changes and skip ASO poll writes ([#4307](https://github.com/Altinn/altinn-platform/issues/4307)) ([4322ff2](https://github.com/Altinn/altinn-platform/commit/4322ff2091a262f238595fb49bc10dcb3396233f))
* **dis-messaging-operator:** report Conflict when two Queues have the same queue name ([#4330](https://github.com/Altinn/altinn-platform/issues/4330)) ([5821644](https://github.com/Altinn/altinn-platform/commit/5821644b0e72b215d45ccef963955da309049c6e))
* **dis-messaging-operator:** validate the operator configuration and the queue name ([#4216](https://github.com/Altinn/altinn-platform/issues/4216)) ([c34f1c4](https://github.com/Altinn/altinn-platform/commit/c34f1c4dd3477ff587e9937ef33e10fdab18a5d8))
* **dis-messaging-operator:** write the connection ConfigMap of each product namespace ([#4327](https://github.com/Altinn/altinn-platform/issues/4327)) ([daffaa0](https://github.com/Altinn/altinn-platform/commit/daffaa0ef438a5a69860e2d42704eafc61380290))


### Bug Fixes

* **deps:** build Go images with golang 1.26.10 ([#4263](https://github.com/Altinn/altinn-platform/issues/4263)) ([de514a3](https://github.com/Altinn/altinn-platform/commit/de514a3cbf7593e42f20eafd4c221b3cc17e0087))
* **deps:** bump grpc, otel and x/net to clear Go security advisories ([#4259](https://github.com/Altinn/altinn-platform/issues/4259)) ([b68ed8b](https://github.com/Altinn/altinn-platform/commit/b68ed8b590bd721fd6f94cf8fbec3ae6c9f82717))
* **deps:** update golang builder images to 1.27.3 ([#4291](https://github.com/Altinn/altinn-platform/issues/4291)) ([63918a5](https://github.com/Altinn/altinn-platform/commit/63918a5ef1b5c7f16e41a80180758319d2c768be))
* **dis-messaging-operator:** name the namespace of the ApplicationIdentity in IdentityNotReady messages ([#4326](https://github.com/Altinn/altinn-platform/issues/4326)) ([b6e8a89](https://github.com/Altinn/altinn-platform/commit/b6e8a896983a09f6ce5caddeaaf252450d4a717a))
* **dis-messaging-operator:** reject a Queue name longer than 63 characters ([#4271](https://github.com/Altinn/altinn-platform/issues/4271)) ([17f7b5a](https://github.com/Altinn/altinn-platform/commit/17f7b5a38b9fdf7b3eaadd1d1a839fd76d08394f))


### Dependency Updates

* update dockerfile non-major dependencies ([#3929](https://github.com/Altinn/altinn-platform/issues/3929)) ([e1a185a](https://github.com/Altinn/altinn-platform/commit/e1a185a220861c8903395b93f048e48a3a5e212c))
* update module github.com/google/cel-go to v0.29.0 [security] ([#4191](https://github.com/Altinn/altinn-platform/issues/4191)) ([17d13cd](https://github.com/Altinn/altinn-platform/commit/17d13cde807ecd06595055943c669750a4d36f24))
