FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.11@sha256:30c448bd02cda6b34bb215d625760930c2a05feace5debea59c9a77a05027f24

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
