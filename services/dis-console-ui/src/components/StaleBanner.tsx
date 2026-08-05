import { Alert, Paragraph } from '@digdir/designsystemet-react';
import type { Cluster } from '../api/types';

export function StaleBanner({ clusters }: { clusters: Cluster[] }) {
  const stale = clusters.filter((c) => c.stale).map((c) => c.cluster);
  if (stale.length === 0) return null;
  return (
    <Alert data-color="warning">
      <Paragraph>
        {stale.length === 1 ? 'Cluster' : 'Clusters'} not reporting recently:{' '}
        <strong>{stale.join(', ')}</strong>. Their deployment state may be out of date.
      </Paragraph>
    </Alert>
  );
}
