FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.7@sha256:fa58535e1a4dfbe9670df81c9b8da61fd3f7269959c0777008ae01ce8ebdb109

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
