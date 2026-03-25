FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.34@sha256:f033c51050f8aca6081fe25f59886960232d3cb66b71d9e1bdcc274b529ff82b

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
