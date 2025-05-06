FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.3@sha256:b41d726e24e8073a5fbad04ab1390fd968181685cdf523d5435aad31e63902b2

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
