import { describe, expect, it } from 'vitest';
import { sortRows, toggleSort, type SortState } from './tableSort';

const rows = [
  { kind: 'Vault', namespace: 'b', name: 'kv', status: 'healthy' as const },
  { kind: 'Database', namespace: 'a', name: 'db2', status: 'failed' as const },
  { kind: 'Database', namespace: 'a', name: 'db1', status: 'reconciling' as const },
];

describe('sortRows', () => {
  it('sorts by a string column with direction', () => {
    const asc = sortRows(rows, { col: 'kind', dir: 'asc' });
    expect(asc.map((r) => r.kind)).toEqual(['Database', 'Database', 'Vault']);
    const desc = sortRows(rows, { col: 'kind', dir: 'desc' });
    expect(desc[0].kind).toBe('Vault');
  });

  it('sorts by status severity, worst first when desc', () => {
    const worst = sortRows(rows, { col: 'status', dir: 'desc' });
    expect(worst.map((r) => r.status)).toEqual(['failed', 'reconciling', 'healthy']);
  });

  it('tie-breaks deterministically', () => {
    const s = sortRows(rows, { col: 'namespace', dir: 'asc' });
    expect(s.map((r) => r.name)).toEqual(['db1', 'db2', 'kv']);
  });
});

describe('toggleSort', () => {
  it('flips direction on the same column and resets on a new one', () => {
    let s: SortState = { col: 'kind', dir: 'asc' };
    s = toggleSort(s, 'kind');
    expect(s).toEqual({ col: 'kind', dir: 'desc' });
    s = toggleSort(s, 'name');
    expect(s).toEqual({ col: 'name', dir: 'asc' });
    s = toggleSort(s, 'status');
    expect(s).toEqual({ col: 'status', dir: 'desc' });
  });
});
