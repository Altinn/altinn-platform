FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.9@sha256:48d8707a9f1ab0fd0fc5f1b3c91afd49c807b438a0d04199197f4e5c0c4bfc65

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
