import { Tooltip } from '@digdir/designsystemet-react';
import { shortRev, type DeployStatus } from '../lib/flux';
import { statusStyle } from '../lib/statusColor';
import type { MatrixCell } from '../lib/matrix';
import type { StageState } from '../lib/releases';
import { StageChip } from './StageChip';

interface Props {
  cell: MatrixCell | undefined;
  env: string;
  appName: string;
  onSelect: (cell: MatrixCell) => void;
}

function chipState(status: DeployStatus): StageState {
  switch (status) {
    case 'healthy':
      return 'current';
    case 'reconciling':
      return 'rolling';
    case 'failed':
      return 'failed';
    case 'suspended':
      return 'suspended';
    case 'absent':
      return 'absent';
    default:
      return 'unknown';
  }
}

/** One environment cell as a stage chip: state icon + the
 *  deployed revision. Inert outline chip when the app is absent from the env. */
export function StatusCell({ cell, env, appName, onSelect }: Props) {
  // A cell may carry only folded children (owner absent from this env).
  const primary = cell?.resource ?? cell?.children?.[0];
  if (!cell || !primary) {
    return <StageChip state="absent" label="—" />;
  }

  const { label } = statusStyle(cell.status);
  const rev = shortRev(primary.revision);
  const conflicts = cell.conflict?.length ?? 0;
  const tip = [
    label,
    primary.revision && `rev ${primary.revision}`,
    primary.reason,
    cell.children?.length ? `charts: ${cell.children.map((c) => c.name).join(', ')}` : null,
  ]
    .filter(Boolean)
    .join(' · ');

  return (
    <Tooltip content={tip}>
      <button
        type="button"
        className="cell-button"
        aria-label={`${appName} in ${env}: ${label}${rev ? `, revision ${rev}` : ''}`}
        onClick={() => onSelect(cell)}
      >
        <StageChip
          state={chipState(cell.status)}
          label={`${rev || label}${conflicts > 1 ? ` (${conflicts})` : ''}`}
        />
      </button>
    </Tooltip>
  );
}
