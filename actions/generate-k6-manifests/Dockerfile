FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.8@sha256:4674f6dadc55442530ffe2b0758bef3b188244e8d9022486b8d324df3a59fa70

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
