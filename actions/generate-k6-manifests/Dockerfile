FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.19@sha256:9dd2860532daf80aca9e6b196893bd41797e0e8e1aeaad73978b18f74e1cde0e

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
