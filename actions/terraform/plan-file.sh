#!/usr/bin/env bash
# Stores, fetches and deletes the Terraform plan file beside the Terraform state.
# Usage: plan-file.sh store|fetch|delete
set -euo pipefail

container=tfstates
blob="${TF_STATE_FILE:?is not set. Run the init action first.}.plans/${GITHUB_RUN_ID:?}.tfplan"
location="${STORAGE_ACCOUNT:?}/${container}/${blob}"

fail() {
  echo "::error title=Terraform plan file::$1"
  exit 1
}

az_blob() {
  local command=$1
  shift
  az storage blob "$command" --auth-mode login --account-name "$STORAGE_ACCOUNT" \
    --container-name "$container" --only-show-errors "$@"
}

store() {
  if [ "${GITHUB_EVENT_NAME:-}" = pull_request ] && [ "${STORE_ON_PULL_REQUEST:-}" != true ]; then
    echo "Pull request run. The plan file is not stored."
    echo "TF_PLAN_FILE=not stored on a pull request run" >> "$GITHUB_ENV"
    return
  fi
  [ -s tfplan.out ] || fail "tfplan.out is missing or empty."
  mkdir -p "$(dirname "$CHECKSUM_FILE")"
  sha256sum tfplan.out | cut -d' ' -f1 > "$CHECKSUM_FILE"
  [[ "$(cat "$CHECKSUM_FILE")" =~ ^[0-9a-f]{64}$ ]] || fail "Could not make a checksum of tfplan.out."
  az_blob upload --name "$blob" --file tfplan.out --overwrite true --no-progress --output none \
    || fail "Could not upload the plan file to ${location}."
  echo "Stored the plan file at ${location}."
  echo "TF_PLAN_FILE=${location}" >> "$GITHUB_ENV"
  echo "stored=true" >> "$GITHUB_OUTPUT"
}

fetch() {
  local expected exists actual
  expected=$(cat "$CHECKSUM_FILE" 2>/dev/null) || true
  [[ "$expected" =~ ^[0-9a-f]{64}$ ]] || fail "The checksum from the plan job is missing or not valid. Re-run all jobs to make a new plan."
  exists=$(az_blob exists --name "$blob" --query exists --output json) \
    || fail "Could not look for the plan file ${location}."
  [ "$exists" = true ] || fail "The plan file ${location} does not exist. A successful apply deletes the plan file. Re-run all jobs to make a new plan."
  rm -f tfplan.out
  az_blob download --name "$blob" --file tfplan.out --no-progress --output none \
    || fail "Could not download the plan file ${location}."
  actual=$(sha256sum tfplan.out | cut -d' ' -f1)
  if [ "$actual" != "$expected" ]; then
    rm -f tfplan.out
    fail "The plan file ${location} does not match the checksum from the plan job. Terraform did not apply it. Re-run all jobs to make a new plan."
  fi
  echo "Downloaded the plan file ${location}. The checksum matches."
}

# Blob versioning is on. A delete turns the blob into an old version, so delete each version too.
delete_all_versions() {
  local url versions version
  url=$(az_blob url --name "$blob" --output tsv) || return 1
  az_blob delete --name "$blob" --output none || return 1
  versions=$(az_blob list --prefix "$blob" --include v --query '[].versionId' --output tsv) || return 1
  for version in $versions; do
    az rest --method delete --url "${url}?versionid=${version}" --resource https://storage.azure.com/ \
      --headers x-ms-version=2023-11-03 --only-show-errors --output none || return 1
  done
}

[[ "$STORAGE_ACCOUNT" =~ ^[a-z0-9]{3,24}$ ]] || fail "The storage account name is not valid."

case "${1:-}" in
  store) store ;;
  fetch) fetch ;;
  delete)
    if delete_all_versions; then
      echo "Deleted the plan file ${location}."
    else
      echo "::warning title=Terraform plan file not deleted::Could not delete ${location} or one of its versions. The apply did succeed. Delete the blob and its versions by hand."
    fi
    ;;
  *) fail "Usage: plan-file.sh store|fetch|delete" ;;
esac
