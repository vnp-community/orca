import { describe, expect, it } from "vitest";
import {
  layoutSquarifiedTreemap,
  SMALL_CELL_ID,
} from "./structure-treemap-layout";

const RECT = { x: 0, y: 0, w: 800, h: 500 };
const items = (n: number) =>
  Array.from({ length: n }, (_, i) => ({ id: `i${i}`, value: n - i + 1 }));

const area = (cs: { w: number; h: number }[]) =>
  cs.reduce((s, c) => s + c.w * c.h, 0);
function overlaps(
  a: { x: number; y: number; w: number; h: number },
  b: typeof a,
): boolean {
  const e = 1e-6;
  return (
    a.x < b.x + b.w - e &&
    b.x < a.x + a.w - e &&
    a.y < b.y + b.h - e &&
    b.y < a.y + a.h - e
  );
}

describe("layoutSquarifiedTreemap", () => {
  it("fills the frame (< 0.5 % error) without overlaps", () => {
    const { cells } = layoutSquarifiedTreemap(items(30), RECT);
    expect(Math.abs(area(cells) - 800 * 500) / (800 * 500)).toBeLessThan(0.005);
    for (let i = 0; i < cells.length; i++) {
      for (let j = i + 1; j < cells.length; j++) {
        expect(overlaps(cells[i], cells[j])).toBe(false);
      }
    }
  });
  it("is deterministic and breaks ties by id", () => {
    const tied = [
      { id: "b", value: 5 },
      { id: "a", value: 5 },
      { id: "c", value: 5 },
    ];
    const r1 = layoutSquarifiedTreemap(tied, RECT);
    const r2 = layoutSquarifiedTreemap(tied.toReversed(), RECT);
    expect(r1).toEqual(r2);
    expect(r1.cells[0].id).toBe("a");
  });
  it("drops zero/negative/NaN values and handles a single item", () => {
    const r = layoutSquarifiedTreemap(
      [
        { id: "z", value: 0 },
        { id: "n", value: -2 },
        { id: "x", value: Number.NaN },
        { id: "one", value: 4 },
      ],
      RECT,
    );
    expect(r.cells).toHaveLength(1);
    expect(r.cells[0]).toMatchObject({ id: "one", x: 0, y: 0, w: 800, h: 500 });
    expect(layoutSquarifiedTreemap([], RECT).cells).toEqual([]);
    expect(
      layoutSquarifiedTreemap(items(3), { x: 0, y: 0, w: 0, h: 10 }).cells,
    ).toEqual([]);
  });
  it('caps at 400 cells and folds the rest into "+N small"', () => {
    const r = layoutSquarifiedTreemap(items(1000), {
      x: 0,
      y: 0,
      w: 4000,
      h: 3000,
    });
    expect(r.cells.length).toBeLessThanOrEqual(401);
    expect(r.cells.some((c) => c.id === SMALL_CELL_ID)).toBe(true);
    expect(r.mergedIds.length).toBeGreaterThanOrEqual(600);
  });
  it("folds cells that are too small to draw", () => {
    const r = layoutSquarifiedTreemap(
      [
        { id: "big", value: 10_000 },
        ...Array.from({ length: 20 }, (_, i) => ({ id: `t${i}`, value: 1 })),
      ],
      RECT,
    );
    expect(r.mergedIds.length).toBeGreaterThan(0);
    expect(r.cells.find((c) => c.id === "big")).toBeTruthy();
  });
  it("padding insets cells but keeps them inside the frame", () => {
    const { cells } = layoutSquarifiedTreemap(items(10), RECT, { padding: 4 });
    for (const c of cells) {
      expect(c.x).toBeGreaterThanOrEqual(0);
      expect(c.x + c.w).toBeLessThanOrEqual(800 + 1e-6);
      expect(c.w).toBeGreaterThan(0);
    }
  });
  it("5000 items do not throw", () => {
    const t = Date.now();
    expect(() => layoutSquarifiedTreemap(items(5000), RECT)).not.toThrow();
    expect(Date.now() - t).toBeLessThan(5000);
  });
});
