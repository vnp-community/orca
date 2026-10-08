import { describe, expect, it } from "vitest";
import type { ImpactGraph } from "../../../../../shared/code-intel-graph-types";
import { countImpactNodes, layoutImpactColumns } from "./impact-column-layout";

const sym = (name: string, dir = "src/a") => ({
  key: `${dir}/${name}`,
  kind: "function",
  name,
  filePath: `${dir}/${name}.ts`,
});
const center = sym("center");

function graph(
  direction: "upstream" | "downstream",
  levels: { depth: number; names: string[]; direct?: boolean }[],
): ImpactGraph {
  return {
    target: center as never,
    direction,
    risk: "LOW",
    impactedCount: 0,
    levels: levels.map((l) => ({
      depth: l.depth,
      symbols: l.names.map((n) => ({
        symbol: sym(n) as never,
        via: "calls",
        direct: l.direct ?? l.depth === 1,
      })),
    })),
    affectedFlows: [],
    affectedClusters: [],
    testsCovering: [],
  };
}

describe("layoutImpactColumns", () => {
  it("puts upstream at negative, downstream at positive columns and centre at 0", () => {
    const l = layoutImpactColumns(
      center,
      graph("upstream", [
        { depth: 1, names: ["u1"] },
        { depth: 2, names: ["u2"] },
      ]),
      graph("downstream", [{ depth: 1, names: ["d1"] }]),
    );
    expect(l.columns.map((c) => c.column)).toEqual([-2, -1, 0, 1]);
    const x = (id: string) => l.nodes.find((n) => n.id === id)!.x;
    expect(x("center")).toBe(0);
    expect(x("-1:src/a/u1")).toBe(-340);
    expect(x("1:src/a/d1")).toBe(340);
  });

  it("only draws lines between centre and direct nodes (no invented deeper edges)", () => {
    const l = layoutImpactColumns(
      center,
      null,
      graph("downstream", [
        { depth: 1, names: ["d1"] },
        { depth: 2, names: ["d2a", "d2b"], direct: false },
      ]),
    );
    expect(l.edges).toHaveLength(1);
    expect(l.edges[0]).toMatchObject({
      source: "center",
      target: "1:src/a/d1",
    });
    const up = layoutImpactColumns(
      center,
      graph("upstream", [{ depth: 1, names: ["u"] }]),
      null,
    );
    expect(up.edges[0]).toMatchObject({
      source: "-1:src/a/u",
      target: "center",
    });
  });

  it("sorts by folder then name, stably, regardless of input order", () => {
    const g1 = graph("downstream", [{ depth: 1, names: ["b", "a", "c"] }]);
    const g2 = graph("downstream", [{ depth: 1, names: ["c", "b", "a"] }]);
    const ids = (g: ImpactGraph) =>
      layoutImpactColumns(center, null, g)
        .nodes.filter((n) => n.type === "symbol")
        .map((n) => n.id);
    expect(ids(g1)).toEqual(ids(g2));
    expect(ids(g1)).toEqual(["1:src/a/a", "1:src/a/b", "1:src/a/c"]);
  });

  it('caps a column at 60 with a "+N more" node, expandable', () => {
    const names = Array.from(
      { length: 75 },
      (_, i) => `n${String(i).padStart(3, "0")}`,
    );
    const g = graph("downstream", [{ depth: 1, names }]);
    const l = layoutImpactColumns(center, null, g);
    expect(l.nodes.filter((n) => n.type === "symbol")).toHaveLength(60);
    expect(l.hiddenCountByColumn[1]).toBe(15);
    expect(l.nodes.find((n) => n.type === "more")).toMatchObject({
      hiddenCount: 15,
    });
    const open = layoutImpactColumns(center, null, g, {
      expandedColumns: new Set([1]),
    });
    expect(open.nodes.filter((n) => n.type === "symbol")).toHaveLength(75);
    expect(open.nodes.some((n) => n.type === "more")).toBe(false);
  });

  it("keeps a node present in both directions in each column", () => {
    const l = layoutImpactColumns(
      center,
      graph("upstream", [{ depth: 1, names: ["both"] }]),
      graph("downstream", [{ depth: 1, names: ["both"] }]),
    );
    expect(l.nodes.filter((n) => n.type === "symbol")).toHaveLength(2);
  });

  it("handles 1500 nodes without throwing and is deterministic", () => {
    const names = Array.from({ length: 1500 }, (_, i) => `n${i}`);
    const g = graph("downstream", [{ depth: 1, names }]);
    const a = layoutImpactColumns(center, null, g);
    const b = layoutImpactColumns(center, null, g);
    expect(a).toEqual(b);
    expect(countImpactNodes(g, null)).toBe(1500);
  });

  it("ignores levels with invalid depth", () => {
    const l = layoutImpactColumns(
      center,
      null,
      graph("downstream", [{ depth: 0, names: ["x"] }]),
    );
    expect(l.nodes).toHaveLength(1);
  });
});
