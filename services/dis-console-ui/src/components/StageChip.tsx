import { Tag } from '@digdir/designsystemet-react';
import type { StageState } from '../lib/releases';

type ChipColor = 'success' | 'info' | 'danger' | 'warning' | 'neutral';

const STYLE: Record<StageState, { color: ChipColor; variant: 'default' | 'outline'; icon: string }> = {
  current: { color: 'success', variant: 'default', icon: '✓' },
  deployed: { color: 'success', variant: 'outline', icon: '✓' },
  rolling: { color: 'info', variant: 'default', icon: '↻' },
  failed: { color: 'danger', variant: 'default', icon: '✕' },
  suspended: { color: 'warning', variant: 'default', icon: '⏸' },
  unknown: { color: 'neutral', variant: 'default', icon: '?' },
  absent: { color: 'neutral', variant: 'outline', icon: '○' },
};

/** A release stage chip: state icon + label. */
export function StageChip({ state, label }: { state: StageState; label: string }) {
  const s = STYLE[state];
  return (
    <Tag data-color={s.color} data-size="sm" variant={s.variant} className="stage-chip">
      <span aria-hidden>{s.icon}</span> {label}
    </Tag>
  );
}
