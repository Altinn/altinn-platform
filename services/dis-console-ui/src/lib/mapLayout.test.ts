import { describe, expect, it } from 'vitest';
import { layoutTree } from './mapLayout';

const OPTS = { nodeWidth: 100, nodeHeight: 40, colGap: 50, rowGap: 10 };

describe('layoutTree', () => {
  it('places children in the next column and centers the parent', () => {
    const { pos, width, height } = layoutTree(
      ['root', 'a', 'b'],
      [
        { from: 'root', to: 'a' },
        { from: 'root', to: 'b' },
      ],
      OPTS,
    );
    expect(pos.get('a')).toEqual({ x: 150, y: 0 });
    expect(pos.get('b')).toEqual({ x: 150, y: 50 });
    expect(pos.get('root')).toEqual({ x: 0, y: 25 });
    expect(width).toBe(250);
    expect(height).toBe(90);
  });

  it('stacks multiple roots and deep chains', () => {
    const { pos } = layoutTree(
      ['r1', 'r2', 'c'],
      [{ from: 'r2', to: 'c' }],
      OPTS,
    );
    expect(pos.get('r1')?.y).not.toEqual(pos.get('c')?.y);
    expect(pos.get('r2')?.x).toBe(0);
    expect(pos.get('c')?.x).toBe(150);
  });

  it('keeps orphan nodes visible', () => {
    const { pos } = layoutTree(['a', 'orphan'], [], OPTS);
    expect(pos.has('orphan')).toBe(true);
  });
});
