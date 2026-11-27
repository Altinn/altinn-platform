#!/usr/bin/env bash
# Tests "plan-file.sh delete" with a stub of the Azure CLI. It makes no call to Azure.
set -euo pipefail

script="$(cd "$(dirname "$0")/.." && pwd)/plan-file.sh"
location=account/tfstates/github.com/owner/repo/environments/test/name.tfstate.plans/1.tfplan
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

cat > "$work/az" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "$STUB_CALLS"
case "$1 $3" in
  "storage url") echo "https://account.blob.core.windows.net/tfstates/blob" ;;
  "storage list") printf '%s\n' 2026-01-01T00:00:01.0000000Z 2026-01-01T00:00:02.0000000Z ;;
  "storage delete")
    [ "$STUB_FAIL" != blob ] || { echo "ERROR: stub: the blob delete failed" >&2; exit 1; } ;;
  "rest delete")
    case "$STUB_FAIL" in
      version)
        echo 'ERROR: The specified operation is not allowed on version.(<?xml version="1.0" encoding="utf-8"?><Error><Code>OperationNotAllowedOnAutomaticSnapshot</Code><Message>The specified operation is not allowed on version.' >&2
        echo 'RequestId:00000000-0000-0000-0000-000000000000' >&2
        echo 'Time:2026-01-01T00:00:03.0000000Z</Message></Error>)' >&2
        exit 1 ;;
      no-code) echo "ERROR: stub: no code" >&2; exit 1 ;;
    esac ;;
  *) echo "ERROR: stub: unknown command: $*" >&2; exit 2 ;;
esac
STUB
chmod +x "$work/az"

failed=0
# Usage: expect <stub failure> <number of version deletes> <output>
expect() {
  local output deletes
  : > "$work/calls"
  output=$(PATH="$work:$PATH" STUB_CALLS="$work/calls" STUB_FAIL=$1 GITHUB_RUN_ID=1 STORAGE_ACCOUNT=account \
    TF_STATE_FILE=github.com/owner/repo/environments/test/name.tfstate bash "$script" delete 2>&1) || output="exit code $?: ${output}"
  deletes=$(grep -c '^rest ' "$work/calls" || true)
  if [ "$output" = "$3" ] && [ "$deletes" = "$2" ]; then
    echo "ok: $1"
  else
    echo "FAILED: $1. Version deletes: ${deletes}, want $2. Output: ${output//::/: :}"
    failed=1
  fi
}

expect none 2 "Deleted the plan file ${location}."
expect version 2 "Deleted the plan file ${location}. An old version of it stays in the storage account. Could not delete blob versions (error code: OperationNotAllowedOnAutomaticSnapshot)."
expect no-code 2 "ERROR: stub: no code
ERROR: stub: no code
Deleted the plan file ${location}. An old version of it stays in the storage account. Could not delete blob versions (error code: unknown)."
expect blob 0 "ERROR: stub: the blob delete failed
::warning title=Terraform plan file not deleted::Could not delete the plan file ${location}. It stays in the storage account. Delete the blob and its versions by hand."
exit "$failed"
