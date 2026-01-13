FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.27@sha256:b0dfcbc49641f7143966b0b19274526e0e5c278a17e92db88b7933bc56e13b3d

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
