# Run K6 tests

This composite action combines Azure Authentication, kubeconfig configuration, manifest generation and manifest deployment which are the steps needed to run a k6 test in k8s.

## Inputs

## `azure-client-id`

**Required** The client id associated with the SP to authenticate towards Azure.

## `azure-tenant-id`

**Required** The Azure tenant id.

## `azure-subscription-id`

**Required** The subscription id where the Platform managed k8 cluster is located.

## `config_file`

**Required** Path to where the test configurations are located



## Secrets used
This action uses [azure/login](https://github.com/marketplace/actions/azure-login#login-with-openid-connect-oidc-recommended) so it uses the same secrets.

## Example Usage
```
    - name: Run k6 tests
      uses: ./actions/run-k6-tests/
      with:
        azure-client-id:       ${{ secrets.AZURE_CLIENT_ID }}
        azure-tenant-id:       ${{ secrets.AZURE_TENANT_ID }}
        azure-subscription-id: ${{ secrets.AZURE_PLATFORM_SUBSCRIPTION_ID }}

        config_file: "./services/k6/conf.yaml"
```
