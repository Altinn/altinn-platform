FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.14@sha256:02065e5e68914e9082bc032bc4ca5e287d3b76b56fe814d57cfdbad74bef0041

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
