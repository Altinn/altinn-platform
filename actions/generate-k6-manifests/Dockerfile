FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.42@sha256:3d34a4a4bf30b0e93b68a27788104002713e01de74fd3846075986bf26afb303

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
