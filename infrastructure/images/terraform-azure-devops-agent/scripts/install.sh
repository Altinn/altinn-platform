#!/bin/bash
set -e

# Versions
KUBECTL_VERSION="v1.32.1" #Get the latest version with: curl -L -s https://dl.k8s.io/release/stable.txt

# Install kubectl
echo "Installing kubectl $KUBECTL_VERSION"
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256"
# Verify the checksum
echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check
# Install kubectl
install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
# Clean up
rm kubectl kubectl.sha256 install.sh