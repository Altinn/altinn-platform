FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.28@sha256:b17a0ee9ffa9bf2d893210def2d1916d7e6d9e84e5fcc6b500a26e36bb229b26

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
