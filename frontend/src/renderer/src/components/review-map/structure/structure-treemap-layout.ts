/**
 * structure-treemap-layout.ts — FE-CV-TASK-054-01
 *
 * Single-level squarified treemap for the Structure lens, capped at 400 cells. Reuses the
 * squarified algorithm already in quality-charts (aspect ratio is much better than the
 * status-bar bisecting layout, so that one is not used); this adds the cap, the "+N small"
 * aggregate cell and padding the lens needs.
 *
 * @module components/review-map/structure/structure-treemap-layout
 */

import { squarify } from "../../quality-charts/treemap-squarified-layout";

export const SMALL_CELL_ID = "__small__";
export const TREEMAP_MAX_CELLS = 400;
export const TREEMAP_MIN_CELL_W = 24;
export const TREEMAP_MIN_CELL_H = 16;

export type TreemapItem = { id: string; value: number };
export type TreemapRect = { x: number; y: number; w: number; h: number };
export type TreemapCell = TreemapRect & { id: string };

export type TreemapLayoutResult = {
  cells: TreemapCell[];
  /** Ids folded into the SMALL_CELL_ID aggregate (too many, or too small to draw). */
  mergedIds: string[];
};

type Options = { padding?: number; maxCells?: number };

function compare(a: TreemapItem, b: TreemapItem): number {
  return b.value - a.value || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
}

export function layoutSquarifiedTreemap(
  items: readonly TreemapItem[],
  rect: TreemapRect,
  { padding = 0, maxCells = TREEMAP_MAX_CELLS }: Options = {},
): TreemapLayoutResult {
  const valid = items
    .filter((i) => Number.isFinite(i.value) && i.value > 0)
    .sort(compare);
  const bounds = { x: rect.x, y: rect.y, width: rect.w, height: rect.h };
  if (valid.length === 0 || !(rect.w > 0) || !(rect.h > 0)) {
    return { cells: [], mergedIds: [] };
  }

  let kept = valid.slice(0, maxCells);
  let merged = valid.slice(maxCells);
  let result = run(kept, merged);
  // Why: a second pass folds cells that came out too small to draw, so labels stay legible.
  for (let pass = 0; pass < 2; pass++) {
    const tooSmall = new Set(
      result
        .filter(
          (c) =>
            c.id !== SMALL_CELL_ID &&
            (c.width < TREEMAP_MIN_CELL_W || c.height < TREEMAP_MIN_CELL_H),
        )
        .map((c) => c.id),
    );
    if (tooSmall.size === 0) {
      break;
    }
    merged = [...merged, ...kept.filter((k) => tooSmall.has(k.id))];
    kept = kept.filter((k) => !tooSmall.has(k.id));
    result = run(kept, merged);
  }

  function run(
    keep: readonly TreemapItem[],
    fold: readonly TreemapItem[],
  ): { id: string; x: number; y: number; width: number; height: number }[] {
    const sized = keep.map((i) => ({ id: i.id, size: i.value }));
    if (fold.length > 0) {
      sized.push({
        id: SMALL_CELL_ID,
        size: fold.reduce((s, i) => s + i.value, 0),
      });
    }
    return squarify(sized, bounds).rects;
  }

  const half = padding / 2;
  const cells = result.map((c) => {
    const inset = Math.min(half, c.width / 4, c.height / 4);
    return {
      id: c.id,
      x: c.x + inset,
      y: c.y + inset,
      w: c.width - inset * 2,
      h: c.height - inset * 2,
    };
  });
  return { cells, mergedIds: merged.map((m) => m.id).sort() };
}
