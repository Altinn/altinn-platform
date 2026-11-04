#!/bin/bash
set -e

# Versions
# renovate: datasource=github-tags depName=kubectl packageName=kubernetes/kubectl versioning=semver extractVersion=^kubernetes-(?<version>.*)$
KUBECTL_VERSION="v1.37.1" #Get the latest version with: curl -L -s https://dl.k8s.io/release/stable.txt
# renovate: datasource=github-releases depName=helm packageName=helm/helm versioning=semver
HELM_VERSION="v4.3.1" #Find the latest version at https://github.com/helm/helm/releases
# renovate: datasource=github-releases depName=powershell packageName=PowerShell/PowerShell versioning=semver
POWERSHELL_VERSION="v7.6.9" #Find the latest version at https://github.com/PowerShell/PowerShell/releases

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
apt-get install -y wget openssl

# Install PowerShell from the GitHub release instead of packages.microsoft.com,
# which does not publish the stable package for every Ubuntu release
POWERSHELL_DEB="powershell_${POWERSHELL_VERSION#v}-1.deb_amd64.deb"
curl -LO "https://github.com/PowerShell/PowerShell/releases/download/${POWERSHELL_VERSION}/${POWERSHELL_DEB}"
curl -LO "https://github.com/PowerShell/PowerShell/releases/download/${POWERSHELL_VERSION}/hashes.sha256"
# Verify the checksum (hashes.sha256 is UTF-16LE with CRLF line endings)
iconv -f UTF-16LE -t UTF-8 hashes.sha256 | tr -d '\r' | grep " \*${POWERSHELL_DEB}$" | sha256sum --check
apt-get install -y "./${POWERSHELL_DEB}"
rm "${POWERSHELL_DEB}" hashes.sha256

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
