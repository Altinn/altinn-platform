FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.36@sha256:a1adf14104f851b7a0c7996cf9ff2104cb13b3f4271ab408048aa8eae18948d0

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
