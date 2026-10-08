import { describe, expect, it } from "vitest";
import type { ModuleNode } from "../../../../../shared/code-intel-graph-types";
import { buildChangedPathIndex } from "./structure-changed-index";
import {
  buildTreemapCells,
  hasLocData,
  treemapValue,
} from "./structure-treemap-model";
import type { StructureTreeState } from "./structure-tree-model";

const state: StructureTreeState = { nodes: new Map(), folders: new Map() };
const nodes: ModuleNode[] = [
  {
    id: "a/x.go",
    kind: "file",
    symbolCount: 5,
    loc: 100,
    language: "go",
    area: "svc",
  },
  { id: "a/y.ts", kind: "file", symbolCount: 2, language: "ts", area: "ui" },
  { id: "a/dir", kind: "folder", symbolCount: 9, area: "svc" },
];
const changed = buildChangedPathIndex(
  [{ path: "a/x.go", status: "modified" }],
  ["a/x.go"],
  [],
);
const base = { nodes, state, changed, changedFiles: [], keepFiles: null };

describe("structure treemap model", () => {
  it("sizes by symbols or loc and detects missing loc", () => {
    expect(treemapValue(nodes[0], "symbols", state)).toBe(5);
    expect(treemapValue(nodes[0], "loc", state)).toBe(100);
    expect(treemapValue(nodes[1], "loc", state)).toBe(0);
    expect(hasLocData(nodes)).toBe(true);
    expect(hasLocData([nodes[1]])).toBe(false);
  });
  it("colours by area in alphabetical order with text group names", () => {
    const { cells, groups } = buildTreemapCells({
      ...base,
      sizeBy: "symbols",
      colorBy: "area",
    });
    expect(cells.find((c) => c.id === "a/x.go")?.tokenVar).toBe(
      "--review-area-1",
    );
    expect(cells.find((c) => c.id === "a/y.ts")?.tokenVar).toBe(
      "--review-area-2",
    );
    expect([...groups.keys()].sort()).toEqual(["svc", "ui"]);
  });
  it("colours by language; folders stay neutral", () => {
    const { cells } = buildTreemapCells({
      ...base,
      sizeBy: "symbols",
      colorBy: "language",
    });
    expect(cells.find((c) => c.kind === "folder")?.tokenVar).toBeNull();
    expect(cells.find((c) => c.id === "a/x.go")?.tokenVar).toMatch(
      /^--review-area-/,
    );
  });
  it('carries overlay flags and never an "affected" flag', () => {
    const { cells } = buildTreemapCells({
      ...base,
      sizeBy: "symbols",
      colorBy: "area",
    });
    const x = cells.find((c) => c.id === "a/x.go")!;
    expect([...x.flags].sort()).toEqual(["changed", "untested"]);
    expect(cells.some((c) => c.flags.has("affected"))).toBe(false);
  });
  it("chip filter dims branches without changed files", () => {
    const { cells } = buildTreemapCells({
      ...base,
      sizeBy: "symbols",
      colorBy: "area",
      keepFiles: new Set(["a/x.go"]),
    });
    expect(cells.find((c) => c.id === "a/x.go")?.dimmed).toBe(false);
    expect(cells.find((c) => c.id === "a/y.ts")?.dimmed).toBe(true);
  });
});
