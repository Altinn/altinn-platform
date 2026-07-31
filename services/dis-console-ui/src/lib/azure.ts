// Build an Azure Portal deep-link from an ARM resource id (which begins
// "/subscriptions/..."). With a tenant it pins the AAD directory; without it,
// the Portal resolves/prompts. Callers pass VITE_AZURE_PORTAL_TENANT when set.
const PORTAL = 'https://portal.azure.com';

export function portalUrl(azureResourceId: string, tenant?: string): string {
  const id = azureResourceId.startsWith('/') ? azureResourceId : `/${azureResourceId}`;
  return tenant ? `${PORTAL}/#@${tenant}/resource${id}` : `${PORTAL}/#resource${id}`;
}

/** The resource-group segment of an ARM id (for display), or ''. */
export function resourceGroupOf(azureResourceId: string): string {
  return azureResourceId.match(/\/resourceGroups\/([^/]+)/i)?.[1] ?? '';
}
