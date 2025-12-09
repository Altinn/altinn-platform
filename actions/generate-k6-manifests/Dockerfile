FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.21@sha256:1c08b76a1dd3d3d5a5976307aebc47572645be1eb2383c648cfdbebb5b536d3b

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
