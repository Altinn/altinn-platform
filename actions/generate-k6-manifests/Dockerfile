FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.15@sha256:eb3871109edef311359e9253b396da7aa24032964cca2e5128251698e29b676e

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
