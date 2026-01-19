FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.29@sha256:ccf4a8063a62d823961f4fb40c7a1943016dca1fc994261214f8653abba32f39

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
