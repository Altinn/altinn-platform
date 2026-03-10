FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.33@sha256:8bd9eb85991e61993733dc3d82e9859e935c5c69991d6d201bb4c87b2eed2895

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
