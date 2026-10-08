/**
 * impact-column-layout.ts — FE-CV-TASK-053-02
 *
 * Pure column layout for the Impact lens. The v7 ImpactGraph has NO edges (PQ-19(5)), so
 * nodes are laid out by depth: column -d = upstream level d, 0 = the centre, +d = downstream
 * level d. Only centre<->direct nodes get a line; nothing is inferred between deeper levels.
 *
 * @module components/review-map/impact/impact-column-layout
 */

import type {
  ImpactGraph,
  ImpactSymbol,
} from "../../../../../shared/code-intel-graph-types";
import type { SymbolRefView } from "../review-wire-types";

export type ImpactLayoutOptions = {
  rowHeight: number;
  columnWidth: number;
  columnGap: number;
  maxPerColumn: number;
  /** Columns whose hidden nodes were expanded by the user. */
  expandedColumns?: ReadonlySet<number>;
};

export const DEFAULT_IMPACT_LAYOUT: ImpactLayoutOptions = {
  rowHeight: 56,
  columnWidth: 260,
  columnGap: 80,
  maxPerColumn: 60,
};

export type ImpactLayoutNode =
  | {
      id: string;
      type: "center";
      column: 0;
      x: number;
      y: number;
      symbol: SymbolRefView;
    }
  | {
      id: string;
      type: "symbol";
      column: number;
      x: number;
      y: number;
      symbol: SymbolRefView;
      direct: boolean;
      via: string;
      confidence?: number;
    }
  | {
      id: string;
      type: "more";
      column: number;
      x: number;
      y: number;
      hiddenCount: number;
    };

export type ImpactLayoutEdge = { id: string; source: string; target: string };

export type ImpactLayout = {
  nodes: ImpactLayoutNode[];
  edges: ImpactLayoutEdge[];
  columns: { column: number; x: number; count: number }[];
  hiddenCountByColumn: Record<number, number>;
};

export const CENTER_NODE_ID = "center";

/** Last two directory segments of the path, used to keep sibling files together. */
function folderKey(filePath: string): string {
  const parts = filePath.replace(/\\/g, "/").split("/");
  parts.pop();
  return parts.slice(-2).join("/");
}

function compareSymbols(a: ImpactSymbol, b: ImpactSymbol): number {
  const fa = folderKey(a.symbol.filePath);
  const fb = folderKey(b.symbol.filePath);
  if (fa !== fb) {
    return fa < fb ? -1 : 1;
  }
  if (a.symbol.name !== b.symbol.name) {
    return a.symbol.name < b.symbol.name ? -1 : 1;
  }
  return a.symbol.key < b.symbol.key ? -1 : a.symbol.key > b.symbol.key ? 1 : 0;
}

function groupByColumn(
  graph: ImpactGraph | null,
  sign: 1 | -1,
  into: Map<number, ImpactSymbol[]>,
): void {
  if (!graph) {
    return;
  }
  for (const level of graph.levels) {
    if (!Number.isFinite(level.depth) || level.depth < 1) {
      continue;
    }
    const column = sign * Math.floor(level.depth);
    const list = into.get(column) ?? [];
    list.push(...level.symbols);
    into.set(column, list);
  }
}

export function layoutImpactColumns(
  center: SymbolRefView,
  upstream: ImpactGraph | null,
  downstream: ImpactGraph | null,
  options: Partial<ImpactLayoutOptions> = {},
): ImpactLayout {
  const o = { ...DEFAULT_IMPACT_LAYOUT, ...options };
  const stepX = o.columnWidth + o.columnGap;
  const byColumn = new Map<number, ImpactSymbol[]>();
  groupByColumn(upstream, -1, byColumn);
  groupByColumn(downstream, 1, byColumn);

  const nodes: ImpactLayoutNode[] = [
    {
      id: CENTER_NODE_ID,
      type: "center",
      column: 0,
      x: 0,
      y: 0,
      symbol: center,
    },
  ];
  const edges: ImpactLayoutEdge[] = [];
  const columns: ImpactLayout["columns"] = [{ column: 0, x: 0, count: 1 }];
  const hiddenCountByColumn: Record<number, number> = {};

  for (const column of [...byColumn.keys()].sort((a, b) => a - b)) {
    const sorted = [...(byColumn.get(column) ?? [])].sort(compareSymbols);
    const expanded = o.expandedColumns?.has(column) === true;
    const visible = expanded ? sorted : sorted.slice(0, o.maxPerColumn);
    const hidden = sorted.length - visible.length;
    const rows = visible.length + (hidden > 0 ? 1 : 0);
    const top = (-(rows - 1) * o.rowHeight) / 2;
    const x = column * stepX;
    visible.forEach((entry, i) => {
      const id = `${column}:${entry.symbol.key}`;
      nodes.push({
        id,
        type: "symbol",
        column,
        x,
        y: top + i * o.rowHeight,
        symbol: entry.symbol,
        direct: entry.direct,
        via: entry.via,
        confidence: entry.confidence,
      });
      if (entry.direct) {
        // Callers (upstream) point at the centre; downstream dependencies are pointed at by it.
        edges.push(
          column < 0
            ? { id: `e:${id}`, source: id, target: CENTER_NODE_ID }
            : { id: `e:${id}`, source: CENTER_NODE_ID, target: id },
        );
      }
    });
    if (hidden > 0) {
      hiddenCountByColumn[column] = hidden;
      nodes.push({
        id: `${column}:more`,
        type: "more",
        column,
        x,
        y: top + visible.length * o.rowHeight,
        hiddenCount: hidden,
      });
    }
    columns.push({ column, x, count: sorted.length });
  }
  columns.sort((a, b) => a.column - b.column);
  return { nodes, edges, columns, hiddenCountByColumn };
}

/** Total symbols across both directions, used to choose graph vs list by default. */
export function countImpactNodes(...graphs: (ImpactGraph | null)[]): number {
  let n = 0;
  for (const g of graphs) {
    for (const level of g?.levels ?? []) {
      n += level.symbols.length;
    }
  }
  return n;
}
