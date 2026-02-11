FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.31@sha256:c61445bfaf8702d9b1bf467a07942e087767847150142facb1185e5a32dc8064 

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
