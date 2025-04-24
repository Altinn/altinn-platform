FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.1@sha256:39d18c2771c2a69fd59384249e5571dd7e80a4699052607df566c58345110cff

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
