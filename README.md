# Altinn Platform

## Altinn Products
To configure or add identity federation (GitHub to Azure), Azure IAM, and handle Terraform state for a product in Altinn, modify the `products.yaml` file and create a pull request.

## Release Please

We have setup Release Please in our repo to automate and simplify the releasing of different artifacts.

Since we have a monorepo we have opted to use the Manifest Driven release-please setup.

This setup consists of two files:
* [.release-please-manifest.json](./.release-please-manifest.json) that contains the latest release version of all packages
* [release-please-config.json](./release-please-config.json) that contains the configuration of all the packages handled by release please

Further documentation about this can be found in the documentation for [Manifest Driven release-please](https://github.com/googleapis/release-please/blob/main/docs/manifest-releaser.md)
