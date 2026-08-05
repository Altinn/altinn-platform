// A tidy left-to-right tree layout for the syncroot map: depth decides the
// column, leaves take consecutive rows, parents center on their children.

export interface MapEdge {
  from: string;
  to: string;
}

export interface MapLayoutOptions {
  nodeWidth: number;
  nodeHeight: number;
  colGap: number;
  rowGap: number;
}

export interface MapLayout {
  /** Top-left position per node id. */
  pos: Map<string, { x: number; y: number }>;
  width: number;
  height: number;
}

export function layoutTree(
  nodeIds: string[],
  edges: MapEdge[],
  { nodeWidth, nodeHeight, colGap, rowGap }: MapLayoutOptions,
): MapLayout {
  const children = new Map<string, string[]>();
  const hasParent = new Set<string>();
  for (const e of edges) {
    children.set(e.from, [...(children.get(e.from) ?? []), e.to]);
    hasParent.add(e.to);
  }
  const roots = nodeIds.filter((id) => !hasParent.has(id));

  const pos = new Map<string, { x: number; y: number }>();
  const rowH = nodeHeight + rowGap;
  let nextRow = 0;
  let maxDepth = 0;

  const place = (id: string, depth: number, seen: Set<string>): number => {
    if (seen.has(id)) return nextRow; // cycle guard — should not happen
    seen.add(id);
    maxDepth = Math.max(maxDepth, depth);
    const kids = (children.get(id) ?? []).filter((k) => !seen.has(k));
    let y: number;
    if (kids.length === 0) {
      y = nextRow * rowH;
      nextRow += 1;
    } else {
      const ys = kids.map((k) => place(k, depth + 1, seen));
      y = (Math.min(...ys) + Math.max(...ys)) / 2;
    }
    pos.set(id, { x: depth * (nodeWidth + colGap), y });
    return y;
  };

  const seen = new Set<string>();
  for (const root of roots) place(root, 0, seen);
  // Nodes unreachable from any root (defensive) get appended below.
  for (const id of nodeIds) {
    if (!pos.has(id)) {
      pos.set(id, { x: 0, y: nextRow * rowH });
      nextRow += 1;
    }
  }

  return {
    pos,
    width: (maxDepth + 1) * nodeWidth + maxDepth * colGap,
    height: Math.max(nextRow, 1) * rowH - rowGap,
  };
}
