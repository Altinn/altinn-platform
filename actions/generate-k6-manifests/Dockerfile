FROM ghcr.io/altinn/altinn-platform/k6-action-image:v0.0.2

# ADD ./default_scenarios/ actions/generate-k6-manifests/default_scenarios/
# ADD generate.sh /generate.sh
ADD . actions/generate-k6-manifests


RUN go build . && chmod +x generate-k6-manifests && mv generate-k6-manifests /usr/local/bin

CMD ["/generate.sh"]
