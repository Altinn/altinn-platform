FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.4@sha256:029a07561a9c63d864abe6a03f1c471461757907169ea8e29819e6f7f79519c0

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
