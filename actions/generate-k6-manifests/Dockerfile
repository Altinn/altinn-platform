FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.26@sha256:b61cba419067a3dfb9e537548bac334c75406737028b24a0781c97f30212a7c8

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
