#!/bin/bash
set -e

# Versions
KUBECTL_VERSION="v1.32.1" #Get the latest version with: curl -L -s https://dl.k8s.io/release/stable.txt
HELM_VERSION="v3.17.0" #Find the latest version at https://github.com/helm/helm/releases

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
powershell -Command {Install-Module -Name Az -Repository PSGallery -Force}
# remove apt cache
rm -rf /var/lib/apt/lists/*


###################################
# Install Helm
###################################
curl -LO "https://github.com/helm/helm/releases/download/${HELM_VERSION}/helm-${HELM_VERSION}-linux-amd64.tar.gz.asc"
curl -LO "https://github.com/helm/helm/releases/download/${HELM_VERSION}/helm-${HELM_VERSION}-linux-386.tar.gz.sha256sum.asc"
echo "$(cat kubectl.sha256)  kubectl" | sha256sum --check
tar -zxvf helm-${HELM_VERSION}-linux-amd64.tar.gz
mv linux-amd64/helm /usr/local/bin/helm

rm -rf helm-${HELM_VERSION}-linux-amd64.tar.gz.asc helm-${HELM_VERSION}-linux-386.tar.gz.sha256sum.asc linux-amd64


###################################
# Remove self
###################################
rm install.sh
