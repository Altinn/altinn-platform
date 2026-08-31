FROM ubuntu:24.04@sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517
RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq --no-install-recommends \
    apt-transport-https \
    apt-utils \
    ca-certificates \
    curl \
    git \
    iputils-ping \
    jq \
    unzip \
    lsb-release \
    software-properties-common && \
    rm -rf /var/lib/apt/lists/*

RUN curl -sL https://aka.ms/InstallAzureCLIDeb | bash

# Can be 'linux-x64', 'linux-arm64', 'linux-arm', 'rhel.6-x64'.
ENV TARGETARCH=linux-x64

WORKDIR /azp

COPY scripts/start.sh .
RUN chmod +x start.sh

ENTRYPOINT [ "./start.sh" ]
