import { Table } from '@digdir/designsystemet-react';
import type { SortCol, SortState } from '../lib/tableSort';

interface Props {
  col: SortCol;
  label: string;
  sort: SortState;
  onSort: (col: SortCol) => void;
}

/** A clickable, aria-sorted table header cell. */
export function SortableTh({ col, label, sort, onSort }: Props) {
  const active = sort.col === col;
  return (
    <Table.HeaderCell
      scope="col"
      aria-sort={active ? (sort.dir === 'asc' ? 'ascending' : 'descending') : undefined}
    >
      <button type="button" className="sort-th" onClick={() => onSort(col)}>
        {label}
        <span aria-hidden className="sort-th__dir">
          {active ? (sort.dir === 'asc' ? ' ▲' : ' ▼') : ''}
        </span>
      </button>
    </Table.HeaderCell>
  );
}
