FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.41@sha256:4b652998e31dea4604aa1ce62a97342c1160b1acf44b076a32c17d80298843a3

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
