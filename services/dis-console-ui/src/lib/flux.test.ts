import { describe, expect, it } from 'vitest';
import type { Resource } from '../api/types';
import { envLabel, envOf, shortRev, statusOf, tenantOf } from './flux';

function res(partial: Partial<Resource>): Resource {
  return {
    kind: 'Kustomization',
    apiVersion: 'kustomize.toolkit.fluxcd.io/v1',
    namespace: 'acme',
    name: 'app',
    ready: 'True',
    suspended: false,
    cluster: 'acme_at23',
    ...partial,
  };
}

describe('envOf / tenantOf', () => {
  it('splits <tenant>_<env> on the last underscore', () => {
    expect(envOf('acme_at23')).toBe('at23');
    expect(tenantOf('acme_at23')).toBe('acme');
  });

  it('keeps multi-segment tenants intact', () => {
    expect(envOf('acme_apps_production')).toBe('production');
    expect(tenantOf('acme_apps_production')).toBe('acme_apps');
  });

  it('handles ids with no environment suffix', () => {
    expect(envOf('acme')).toBe('');
    expect(tenantOf('acme')).toBe('acme');
  });
});

describe('envLabel', () => {
  it('uses friendly stage labels and upper-cases unknowns', () => {
    expect(envLabel('production')).toBe('Production');
    expect(envLabel('at22')).toBe('AT22');
    expect(envLabel('weird')).toBe('WEIRD');
  });
});

describe('statusOf', () => {
  it('maps Ready state to a coarse status', () => {
    expect(statusOf(res({ ready: 'True' }))).toBe('healthy');
    expect(statusOf(res({ ready: 'False' }))).toBe('failed');
    expect(statusOf(res({ ready: 'Unknown' }))).toBe('reconciling');
  });

  it('reports suspended before Ready state', () => {
    expect(statusOf(res({ ready: 'True', suspended: true }))).toBe('suspended');
  });

  it('treats a missing resource as absent', () => {
    expect(statusOf(undefined)).toBe('absent');
  });
});

describe('shortRev', () => {
  it('extracts a 7-char prefix from git shas', () => {
    expect(shortRev('main@sha1:9f3c1a27d8')).toBe('9f3c1a2');
    expect(shortRev('sha256:abcdef0123456789')).toBe('abcdef0');
    expect(shortRev('9f3c1a27d8e0')).toBe('9f3c1a2');
  });

  it('returns the version part of a Helm chart revision', () => {
    expect(shortRev('mychart@1.2.3')).toBe('1.2.3');
    expect(shortRev('1.2.3')).toBe('1.2.3');
  });

  it('is empty for a missing revision', () => {
    expect(shortRev(undefined)).toBe('');
  });
});
