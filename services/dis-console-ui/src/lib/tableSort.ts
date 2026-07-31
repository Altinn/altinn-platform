import { STATUS_SEVERITY, type DeployStatus } from './flux';

export type SortCol = 'kind' | 'namespace' | 'name' | 'status';

export interface SortState {
  col: SortCol;
  dir: 'asc' | 'desc';
}

export interface SortableRow {
  kind: string;
  namespace: string;
  name: string;
  status: DeployStatus;
}

/** Clicking a column: same column flips direction, a new column starts asc
 *  (status starts desc — worst first is what you sort by status for). */
export function toggleSort(state: SortState, col: SortCol): SortState {
  if (state.col === col) return { col, dir: state.dir === 'asc' ? 'desc' : 'asc' };
  return { col, dir: col === 'status' ? 'desc' : 'asc' };
}

export function sortRows<T extends SortableRow>(rows: T[], state: SortState): T[] {
  const sign = state.dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    const c =
      state.col === 'status'
        ? STATUS_SEVERITY[a.status] - STATUS_SEVERITY[b.status]
        : a[state.col].localeCompare(b[state.col]);
    if (c !== 0) return sign * c;
    // Stable tie-break so equal values keep a deterministic order.
    return (
      a.kind.localeCompare(b.kind) ||
      a.namespace.localeCompare(b.namespace) ||
      a.name.localeCompare(b.name)
    );
  });
}
