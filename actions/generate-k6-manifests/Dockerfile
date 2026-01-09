FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.25@sha256:6c8b1c9ee90ae5d2757571dd9210ea9f8aa4095e2bdf1d9886c355f78b4d1a76 

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
