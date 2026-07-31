import { describe, expect, it } from 'vitest';
import { portalUrl, resourceGroupOf } from './azure';

const id =
  '/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/acme-at23/providers/Microsoft.KeyVault/vaults/acme-kv';

describe('portalUrl', () => {
  it('builds a tenant-less deep link', () => {
    expect(portalUrl(id)).toBe(`https://portal.azure.com/#resource${id}`);
  });

  it('pins the AAD tenant when given', () => {
    expect(portalUrl(id, 'contoso.onmicrosoft.com')).toBe(
      `https://portal.azure.com/#@contoso.onmicrosoft.com/resource${id}`,
    );
  });

  it('tolerates an id without a leading slash', () => {
    expect(portalUrl('subscriptions/x')).toBe('https://portal.azure.com/#resource/subscriptions/x');
  });
});

describe('resourceGroupOf', () => {
  it('extracts the resource group', () => {
    expect(resourceGroupOf(id)).toBe('acme-at23');
  });

  it('is empty when absent', () => {
    expect(resourceGroupOf('/subscriptions/x')).toBe('');
  });
});
