#!/bin/bash
set -e

# Versions
# renovate: datasource=github-tags depName=kubectl packageName=kubernetes/kubectl versioning=semver extractVersion=^kubernetes-(?<version>.*)$
KUBECTL_VERSION="v1.33.4" #Get the latest version with: curl -L -s https://dl.k8s.io/release/stable.txt
# renovate: datasource=github-releases depName=helm packageName=helm/helm versioning=semver
HELM_VERSION="v3.19.0" #Find the latest version at https://github.com/helm/helm/releases

###################################
### Install kubectl
###################################
echo "Installing kubectl $KUBECTL_VERSION"
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl"
curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256"
# Verify the checksum
echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check
# Install kubectl
install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl
# Clean up
rm kubectl kubectl.sha256

###################################
### Install powershell
###################################
# Install dependencies
apt-get update
apt-get install -y wget
# Get the version of Ubuntu
source /etc/os-release

# Download the Microsoft repository keys
wget -q https://packages.microsoft.com/config/ubuntu/$VERSION_ID/packages-microsoft-prod.deb

# Register the Microsoft repository keys
dpkg -i packages-microsoft-prod.deb

# Delete the Microsoft repository keys file
rm packages-microsoft-prod.deb

# Update the list of packages after we added packages.microsoft.com
apt-get update

###################################
# Install PowerShell
apt-get install -y powershell
# Install the Azure PowerShell module
pwsh -Command "Install-Module -Name Az -Repository PSGallery -Force"
# remove apt cache
rm -rf /var/lib/apt/lists/*


###################################
# Install Helm
###################################
curl -LO "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz"
curl -LO "https://get.helm.sh/helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum"
cat helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum | sha256sum --check
tar -zxvf helm-${HELM_VERSION}-linux-amd64.tar.gz
mv linux-amd64/helm /usr/local/bin/helm

rm -rf helm-${HELM_VERSION}-linux-amd64.tar.gz helm-${HELM_VERSION}-linux-amd64.tar.gz.sha256sum linux-amd64


###################################
# Remove self
###################################
rm install.sh
