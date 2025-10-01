FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.12@sha256:70580942a6884e28a51dacc8fb2e691599d1ed756d28c6c74a304aa3d1f03bbf

COPY generate.sh /generate.sh
RUN chmod +x /generate.sh

CMD ["/generate.sh"]
