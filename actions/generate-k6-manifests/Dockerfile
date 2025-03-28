FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.3

ADD generate.sh /generate.sh

# This was just to test calling this from another repo. Will remove eventually.
# The binary will already be available in the image.
ADD . actions/generate-k6-manifests
RUN cd actions/generate-k6-manifests && ls && go build . && chmod +x generate-k6-manifests && mv generate-k6-manifests /usr/local/bin && cd /

CMD ["/generate.sh"]
