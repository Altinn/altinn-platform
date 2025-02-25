FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.1
COPY --from=golang:1.22-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

ADD generate.sh /generate.sh

CMD ["/generate.sh"]
