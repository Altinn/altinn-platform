FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.1.0@sha256:fe8ae202b5466d429d4902829f3210648587d7cb33d1764d633e8987e8322222

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
