import type { ReactNode } from 'react';
import { Tag } from '@digdir/designsystemet-react';
import type { DeployStatus } from '../lib/flux';
import { statusStyle } from '../lib/statusColor';

interface Props {
  status: DeployStatus;
  children?: ReactNode;
}

/** A status chip; falls back to the status' default label when no children. */
export function StatusTag({ status, children }: Props) {
  const style = statusStyle(status);
  return (
    <Tag data-color={style.color} data-size="sm" variant={style.variant}>
      {children ?? style.label}
    </Tag>
  );
}
