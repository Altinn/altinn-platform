FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.39@sha256:8ab30350f554c62fb5073ea70520789a49a74bdabacabd3da51e39c3a79f0c70

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
