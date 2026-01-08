FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.24@sha256:886cef1b55eae3b2b906e49effad3f2efcad24f9b37f26695937f5bf7809ca9a

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
