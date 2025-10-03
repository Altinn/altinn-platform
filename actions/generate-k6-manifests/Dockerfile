FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.13@sha256:c947c451de4a41632b5985f71ba4cb5cbc835620fcfaf8651bc050f8b7ef9ac1

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
