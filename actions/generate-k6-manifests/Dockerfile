FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.10@sha256:03d342a41d7db0ebd3179970617380a389b180754c05d9b2fcf036df8f9f2a70

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
