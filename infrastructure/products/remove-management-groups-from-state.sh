#!/usr/bin/env bash
set -euo pipefail

# Management groups are managed outside this root module. This drops them - and
# everything that was scoped to them - from the terraform state, so that
# terraform stops touching them.
#
# Run this BEFORE applying the config where the blocks were removed, otherwise
# terraform will plan a destroy of the live management groups and their role
# assignments.
#
#   ./remove-management-groups-from-state.sh              -> dry run (default)
#   ./remove-management-groups-from-state.sh --apply      -> remove from state

# Resource types removed wholesale.
TYPES=(
  'azurerm_management_group'
  'azurerm_management_group_subscription_association'
)

# Role assignments that were scoped to a management group.
RESOURCES=(
  'azurerm_role_assignment.administrator_user_access_administrator'
  'azurerm_role_assignment.administrator_contributor'
  'azurerm_role_assignment.reader_reader'
  'azurerm_role_assignment.reader_azure_kubernetes_service_cluster_user_role'
  'azurerm_role_assignment.reader_azure_kubernetes_service_cluster_admin_role'
  'azurerm_role_assignment.apps_user_access_administrator'
  'azurerm_role_assignment.apps_contributor'
  'azurerm_role_assignment.readers'
  'azurerm_role_assignment.developers'
  'azurerm_role_assignment.admins'
)

apply=false
if [ "${1:-}" = "--apply" ]; then
  apply=true
elif [ -n "${1:-}" ]; then
  echo "unknown argument: $1" >&2
  echo "usage: $0 [--apply]" >&2
  exit 2
fi

matches() {
  local address="$1" prefix

  for prefix in "${TYPES[@]}"; do
    # any resource of this type, with or without a for_each/count index
    case "$address" in
    "$prefix".*) return 0 ;;
    esac
  done

  for prefix in "${RESOURCES[@]}"; do
    # exact address, or one instance of it
    case "$address" in
    "$prefix" | "$prefix"'['*) return 0 ;;
    esac
  done

  return 1
}

ADDRESSES=()
while IFS= read -r address; do
  if matches "$address"; then
    ADDRESSES+=("$address")
  fi
done < <(terraform state list)

if [ ${#ADDRESSES[@]} -eq 0 ]; then
  echo "Nothing to remove, state has no management group resources."
  exit 0
fi

echo "Removing from state:"
printf '  %s\n' "${ADDRESSES[@]}"

if [ "$apply" != true ]; then
  echo
  echo "Dry run. Re-run with --apply to remove these from the state."
  exit 0
fi

terraform state rm "${ADDRESSES[@]}"
