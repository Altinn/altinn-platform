FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.6@sha256:6f5a69f507f2f1b6801a96b9767d9b1866534d686ddf2626895a3f873bcdeca9

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
