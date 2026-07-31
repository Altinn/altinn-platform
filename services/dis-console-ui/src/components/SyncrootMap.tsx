import { useEffect, useMemo, useRef, useState } from 'react';
import { Paragraph, Spinner } from '@digdir/designsystemet-react';
import type { Artifact, Resource } from '../api/types';
import type { ResourceRef } from '../hooks/useResourceDetail';
import { useSyncrootMap, type SyncrootMapNode } from '../hooks/useSyncrootMap';
import { layoutTree } from '../lib/mapLayout';

const NODE_W = 190;
const NODE_H = 48;
const COL_GAP = 70;
const ROW_GAP = 14;
const PAD = 24;

interface Props {
  artifact: Artifact;
  resources: Resource[];
  onSelectResource: (ref: ResourceRef) => void;
  onSelectArtifact: (artifact: Artifact) => void;
}

interface ViewBox {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** The syncroot deployment map: its declared objects as a
 *  pannable/zoomable node graph with progressive disclosure — it opens on the
 *  syncroot and its Kustomizations; clicking a node with children expands or
 *  collapses it (the badge shows how many are hidden). Leaves open details. */
export function SyncrootMap({ artifact, resources, onSelectResource, onSelectArtifact }: Props) {
  const { nodes, edges, loading, error } = useSyncrootMap(artifact, resources);
  const svgRef = useRef<SVGSVGElement>(null);
  const [vb, setVb] = useState<ViewBox | null>(null);
  const drag = useRef<{ x: number; y: number; vb: ViewBox; moved: boolean } | null>(null);

  const rootId = `syncroot|${artifact.namespace}|${artifact.name}`;
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set([rootId]));
  useEffect(() => {
    setExpanded(new Set([rootId]));
    setVb(null);
  }, [rootId]);

  const childrenOf = useMemo(() => {
    const m = new Map<string, string[]>();
    for (const e of edges) m.set(e.from, [...(m.get(e.from) ?? []), e.to]);
    return m;
  }, [edges]);

  const visibleIds = useMemo(() => {
    const vis = new Set<string>([rootId]);
    const walk = (id: string) => {
      if (!expanded.has(id)) return;
      for (const c of childrenOf.get(id) ?? []) {
        if (!vis.has(c)) {
          vis.add(c);
          walk(c);
        }
      }
    };
    walk(rootId);
    return vis;
  }, [rootId, expanded, childrenOf]);

  const visNodes = useMemo(() => nodes.filter((n) => visibleIds.has(n.id)), [nodes, visibleIds]);
  const visEdges = useMemo(
    () => edges.filter((e) => visibleIds.has(e.from) && visibleIds.has(e.to)),
    [edges, visibleIds],
  );

  const layout = useMemo(
    () =>
      layoutTree(
        visNodes.map((n) => n.id),
        visEdges,
        { nodeWidth: NODE_W, nodeHeight: NODE_H, colGap: COL_GAP, rowGap: ROW_GAP },
      ),
    [visNodes, visEdges],
  );

  const fit: ViewBox = {
    x: -PAD,
    y: -PAD,
    w: layout.width + PAD * 2,
    h: layout.height + PAD * 2,
  };
  const view = vb ?? fit;

  // Wheel zoom needs a non-passive listener (React's onWheel can't preventDefault).
  useEffect(() => {
    const el = svgRef.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      setVb((prev) => {
        const v = prev ?? fit;
        const factor = e.deltaY < 0 ? 0.85 : 1 / 0.85;
        const rect = el.getBoundingClientRect();
        const px = v.x + ((e.clientX - rect.left) / rect.width) * v.w;
        const py = v.y + ((e.clientY - rect.top) / rect.height) * v.h;
        const w = Math.min(Math.max(v.w * factor, 200), fit.w * 4);
        const h = w * (v.h / v.w);
        return { x: px - ((px - v.x) / v.w) * w, y: py - ((py - v.y) / v.h) * h, w, h };
      });
    };
    el.addEventListener('wheel', onWheel, { passive: false });
    return () => el.removeEventListener('wheel', onWheel);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fit.w, fit.h]);

  if (loading) return <Spinner aria-label="Building the map" data-size="sm" />;
  if (error) return <Paragraph data-color="danger">{error}</Paragraph>;
  if (nodes.length <= 1) {
    return <Paragraph data-color="neutral">Nothing to map for this syncroot yet.</Paragraph>;
  }

  const posOf = (id: string) => layout.pos.get(id) ?? { x: 0, y: 0 };
  const openDetails = (n: SyncrootMapNode) => {
    if (n.kind === 'syncroot') onSelectArtifact(artifact);
    else if (n.swept)
      onSelectResource({
        cluster: artifact.cluster,
        kind: n.kind,
        namespace: n.namespace ?? '',
        name: n.name,
      });
  };

  return (
    <div className="map">
      <div className="map__controls">
        <button type="button" aria-label="Zoom in" onClick={() => setVb((p) => scaleView(p ?? fit, 0.8))}>
          +
        </button>
        <button type="button" aria-label="Zoom out" onClick={() => setVb((p) => scaleView(p ?? fit, 1.25))}>
          −
        </button>
        <button type="button" aria-label="Fit" onClick={() => setVb(null)}>
          ⤢
        </button>
      </div>
      <svg
        ref={svgRef}
        viewBox={`${view.x} ${view.y} ${view.w} ${view.h}`}
        role="img"
        aria-label="Deployment map"
        onPointerDown={(e) => {
          drag.current = { x: e.clientX, y: e.clientY, vb: view, moved: false };
        }}
        onPointerMove={(e) => {
          if (!drag.current || !svgRef.current || e.buttons === 0) return;
          const rect = svgRef.current.getBoundingClientRect();
          const dx = ((e.clientX - drag.current.x) / rect.width) * drag.current.vb.w;
          const dy = ((e.clientY - drag.current.y) / rect.height) * drag.current.vb.h;
          if (Math.abs(e.clientX - drag.current.x) + Math.abs(e.clientY - drag.current.y) > 3) {
            drag.current.moved = true;
          }
          setVb({ ...drag.current.vb, x: drag.current.vb.x - dx, y: drag.current.vb.y - dy });
        }}
        onPointerUp={() => {
          drag.current = null;
        }}
      >
        <g>
          {visEdges.map((e) => {
            const a = posOf(e.from);
            const b = posOf(e.to);
            const x1 = a.x + NODE_W;
            const y1 = a.y + NODE_H / 2;
            const x2 = b.x;
            const y2 = b.y + NODE_H / 2;
            const mx = (x1 + x2) / 2;
            return (
              <path
                key={`${e.from}->${e.to}`}
                className="map__edge"
                d={`M ${x1} ${y1} C ${mx} ${y1}, ${mx} ${y2}, ${x2} ${y2}`}
              />
            );
          })}
          {visNodes.map((n) => {
            const p = posOf(n.id);
            const kids = childrenOf.get(n.id)?.length ?? 0;
            const isOpen = expanded.has(n.id);
            return (
              <foreignObject key={n.id} x={p.x} y={p.y} width={NODE_W} height={NODE_H}>
                <MapNodeBox
                  node={n}
                  childCount={kids}
                  expanded={isOpen}
                  onClick={() => {
                    if (drag.current?.moved) return;
                    if (kids > 0) {
                      setExpanded((prev) => {
                        const next = new Set(prev);
                        if (next.has(n.id)) next.delete(n.id);
                        else next.add(n.id);
                        return next;
                      });
                    } else {
                      openDetails(n);
                    }
                  }}
                  onDetails={kids > 0 && (n.swept || n.kind === 'syncroot') ? () => openDetails(n) : undefined}
                />
              </foreignObject>
            );
          })}
        </g>
      </svg>
    </div>
  );
}

function scaleView(v: ViewBox, factor: number): ViewBox {
  const cx = v.x + v.w / 2;
  const cy = v.y + v.h / 2;
  const w = v.w * factor;
  const h = v.h * factor;
  return { x: cx - w / 2, y: cy - h / 2, w, h };
}

function MapNodeBox({
  node,
  childCount,
  expanded,
  onClick,
  onDetails,
}: {
  node: SyncrootMapNode;
  childCount: number;
  expanded: boolean;
  onClick: () => void;
  onDetails?: () => void;
}) {
  const clickable = childCount > 0 || node.swept || node.kind === 'syncroot';
  const cls = `map-node map-node--${node.status ?? 'unswept'}`;
  const inner = (
    <>
      <span className="map-node__kind">{node.kind}</span>
      <span
        className="map-node__name"
        title={`${node.namespace ? `${node.namespace}/` : ''}${node.name}`}
      >
        {node.name}
      </span>
      {childCount > 0 && (
        <span className="map-node__badge" aria-label={expanded ? 'Collapse' : `${childCount} children`}>
          {expanded ? '−' : childCount}
        </span>
      )}
      {onDetails && (
        <span
          role="button"
          tabIndex={0}
          className="map-node__info"
          aria-label="Details"
          onClick={(e) => {
            e.stopPropagation();
            onDetails();
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.stopPropagation();
              onDetails();
            }
          }}
        >
          ⓘ
        </span>
      )}
    </>
  );
  return clickable ? (
    <button type="button" className={cls} onClick={onClick}>
      {inner}
    </button>
  ) : (
    <div className={cls}>{inner}</div>
  );
}
