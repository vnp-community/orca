import { describe, expect, it } from "vitest";
import type { ModuleNode } from "../../../../../shared/code-intel-graph-types";
import {
  flattenStructureRows,
  folderSymbolCount,
  isSafeStructurePath,
  normalizeModuleNodes,
  type FolderLoad,
  type StructureTreeState,
} from "./structure-tree-model";

const folder = (id: string, symbolCount = 0): ModuleNode => ({
  id,
  kind: "folder",
  symbolCount,
});
const file = (id: string, symbolCount = 1): ModuleNode => ({
  id,
  kind: "file",
  symbolCount,
});
function state(
  nodes: ModuleNode[],
  folders: Record<string, FolderLoad> = {},
): StructureTreeState {
  return {
    nodes: new Map(nodes.map((n) => [n.id, n])),
    folders: new Map(Object.entries(folders)),
  };
}
const ready: FolderLoad = { status: "ready", loaded: 0 };

describe("isSafeStructurePath", () => {
  it.each(["", "a", "a/b.ts"])("accepts %j", (p) =>
    expect(isSafeStructurePath(p)).toBe(true),
  );
  it.each(["..", "a/../b", "/etc", "C:\\x", "a\\b", "a//b", "a/./b", "a\0b"])(
    "rejects %j",
    (p) => expect(isSafeStructurePath(p)).toBe(false),
  );
});

describe("normalizeModuleNodes", () => {
  it("normalizes separators and drops unsafe ids", () => {
    const out = normalizeModuleNodes([file("a\\b.ts"), file("../x"), file("")]);
    expect(out.map((n) => n.id)).toEqual(["a/b.ts"]);
  });
});

describe("flattenStructureRows", () => {
  const nodes = [
    folder("src"),
    file("README.md"),
    folder("src/lib"),
    file("src/a.ts"),
    file("src/lib/z.ts"),
  ];
  it("lists folders before files, with level and aria set data", () => {
    const rows = flattenStructureRows(state(nodes), new Set(["src"]));
    expect(rows.map((r) => r.key)).toEqual([
      "src",
      "src/lib",
      "src/a.ts",
      "README.md",
    ]);
    const lib = rows[1];
    expect(lib).toMatchObject({ level: 2, setsize: 2, posinset: 1 });
    expect(rows[0]).toMatchObject({
      kind: "folder",
      expanded: true,
      setsize: 2,
      posinset: 1,
    });
  });
  it("hides children of collapsed folders", () => {
    expect(
      flattenStructureRows(state(nodes), new Set()).map((r) => r.key),
    ).toEqual(["src", "README.md"]);
  });
  it("adds loading, error and load-more rows", () => {
    const rows = flattenStructureRows(
      state(nodes, {
        src: { status: "loading", loaded: 0 },
        "": { status: "ready", loaded: 2, nextPageToken: "t", totalCount: 10 },
      }),
      new Set(["src"]),
    );
    expect(rows.find((r) => r.kind === "loading")?.path).toBe("src");
    expect(rows.find((r) => r.kind === "more")).toMatchObject({
      shown: 2,
      total: 10,
    });
    const err = flattenStructureRows(
      state(nodes, { src: { status: "error", loaded: 0, errorMessage: "x" } }),
      new Set(["src"]),
    );
    expect(err.some((r) => r.kind === "error")).toBe(true);
  });
  it("chip filter keeps only branches with changed files", () => {
    const rows = flattenStructureRows(
      state(nodes),
      new Set(["src", "src/lib"]),
      new Set(["src/lib/z.ts"]),
    );
    expect(rows.map((r) => r.key)).toEqual(["src", "src/lib", "src/lib/z.ts"]);
  });
  it("handles 10000 nodes", () => {
    const many = Array.from({ length: 10000 }, (_, i) =>
      file(`f${String(i).padStart(5, "0")}.ts`),
    );
    expect(flattenStructureRows(state(many), new Set())).toHaveLength(10000);
  });
});

describe("folderSymbolCount", () => {
  it("uses own count, else sums loaded children and marks it approximate until fully loaded", () => {
    const s = state([folder("a"), file("a/x.ts", 2), file("a/y.ts", 3)], {
      a: { ...ready, nextPageToken: "t" },
    });
    expect(folderSymbolCount(s, folder("a", 9))).toEqual({
      value: 9,
      approximate: false,
    });
    expect(folderSymbolCount(s, folder("a"))).toEqual({
      value: 5,
      approximate: true,
    });
    const full = state([folder("a"), file("a/x.ts", 2)], { a: ready });
    expect(folderSymbolCount(full, folder("a"))).toEqual({
      value: 2,
      approximate: false,
    });
  });
});
