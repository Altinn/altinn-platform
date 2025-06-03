FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.5@sha256:2674a87c102de48a1033eba267e9c8a37b42d0bdef54ae1329e504da22606fe7

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
