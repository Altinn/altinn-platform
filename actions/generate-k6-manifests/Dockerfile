FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.16@sha256:0810f1e2e0d5ee847e39d3ef5501e89d48aad0c4162c5612eef0f7e68ec6a8cc

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
