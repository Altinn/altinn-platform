FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.30@sha256:8d0067c9bdbc632f91c9453a8c04da3393b12483897ae218e46c55691717133d

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
