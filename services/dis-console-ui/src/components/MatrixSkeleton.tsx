import { Skeleton } from '@digdir/designsystemet-react';

export function MatrixSkeleton() {
  return (
    <div className="skeleton-stack">
      {Array.from({ length: 6 }).map((_, i) => (
        <Skeleton key={i} variant="rectangle" width="100%" height={44} />
      ))}
    </div>
  );
}
