FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.18@sha256:160df1188871e425a9eb954d672e9329d5d2ab6a1d986586f2297141bcf95af4

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
