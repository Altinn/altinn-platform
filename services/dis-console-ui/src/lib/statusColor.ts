import type { DeployStatus } from './flux';

/** Designsystemet Tag colors used for deployment status. */
export type TagColor = 'success' | 'info' | 'danger' | 'warning' | 'neutral';

export interface StatusStyle {
  color: TagColor;
  /** Designsystemet Tag variant; outline reads as "empty/not-started". */
  variant: 'default' | 'outline';
  label: string;
}

// The single place that knows how a deployment status maps to Designsystemet
// colors. Keep domain logic (flux.ts) free of presentation concerns.
const STATUS_STYLE: Record<DeployStatus, StatusStyle> = {
  healthy: { color: 'success', variant: 'default', label: 'Healthy' },
  reconciling: { color: 'info', variant: 'default', label: 'Reconciling' },
  failed: { color: 'danger', variant: 'default', label: 'Failed' },
  suspended: { color: 'warning', variant: 'default', label: 'Suspended' },
  unknown: { color: 'neutral', variant: 'default', label: 'Unknown' },
  absent: { color: 'neutral', variant: 'outline', label: 'Not deployed' },
};

export function statusStyle(status: DeployStatus): StatusStyle {
  return STATUS_STYLE[status];
}

/** Tag style for a workload's container-image cell: a failed workload trumps
 *  cross-environment image drift, drift trumps reconciling, else neutral. */
export function imageTagStyle(
  status: DeployStatus,
  drifting: boolean,
): Pick<StatusStyle, 'color' | 'variant'> {
  const color: TagColor =
    status === 'failed' ? 'danger' : drifting ? 'warning' : status === 'reconciling' ? 'info' : 'neutral';
  return { color, variant: color === 'neutral' ? 'outline' : 'default' };
}
