/**
 * structure-treemap-model.ts — FE-CV-TASK-054-05
 *
 * Turns the children of the current folder into treemap items with value, colour token and
 * overlay marks. Pure so the SVG component stays a thin renderer.
 *
 * @module components/review-map/structure/structure-treemap-model
 */

import type { ModuleNode } from "../../../../../shared/code-intel-graph-types";
import type { OverlayFlag } from "../review-overlay-model";
import {
  assignAreaColors,
  assignLanguageColors,
  deriveStructureArea,
} from "./structure-area-model";
import type { ChangedPathIndex } from "./structure-changed-index";
import { baseName, folderSymbolCount } from "./structure-tree-model";
import type { StructureTreeState } from "./structure-tree-model";

export type TreemapSizeBy = "symbols" | "loc";
export type TreemapColorBy = "area" | "language";

export type TreemapCellMeta = {
  id: string;
  name: string;
  kind: "file" | "folder";
  value: number;
  /** Area or language label shown in the legend and tooltips. */
  group: string;
  tokenVar: string | null;
  flags: ReadonlySet<OverlayFlag>;
  changedCount: number;
  symbolCount: number;
  loc?: number;
  dimmed: boolean;
};

export function treemapValue(
  node: ModuleNode,
  sizeBy: TreemapSizeBy,
  state: StructureTreeState,
): number {
  if (sizeBy === "loc") {
    return node.loc ?? 0;
  }
  return node.kind === "folder"
    ? folderSymbolCount(state, node).value
    : node.symbolCount;
}

/** "Lines" needs `loc`; it is disabled when no visible node carries it. */
export function hasLocData(nodes: readonly ModuleNode[]): boolean {
  return nodes.some((n) => typeof n.loc === "number");
}

export function buildTreemapCells(args: {
  nodes: readonly ModuleNode[];
  state: StructureTreeState;
  changed: ChangedPathIndex;
  changedFiles: readonly { path: string; area?: string }[];
  sizeBy: TreemapSizeBy;
  colorBy: TreemapColorBy;
  /** Chip filter: files kept; others are dimmed. null = no filter. */
  keepFiles: ReadonlySet<string> | null;
}): { cells: TreemapCellMeta[]; groups: Map<string, string | null> } {
  const { nodes, state, changed, sizeBy, colorBy, keepFiles } = args;
  const groupOf = (n: ModuleNode): string =>
    colorBy === "area"
      ? deriveStructureArea(n, args.changedFiles)
      : n.kind === "folder"
        ? ""
        : (n.language ?? "unknown");
  const groupNames = nodes.map(groupOf).filter((g) => g !== "");
  let assignment: Map<string, { tokenVar: string }>;
  if (colorBy === "area") {
    assignment = assignAreaColors(groupNames);
  } else {
    const counts = new Map<string, number>();
    for (const g of groupNames) {
      counts.set(g, (counts.get(g) ?? 0) + 1);
    }
    assignment = assignLanguageColors(counts);
  }
  const groups = new Map<string, string | null>();
  const cells = nodes.map((n): TreemapCellMeta => {
    const group = groupOf(n);
    const tokenVar = group ? (assignment.get(group)?.tokenVar ?? null) : null;
    if (group) {
      groups.set(group, tokenVar);
    }
    const counts =
      n.kind === "folder" ? changed.folderCounts.get(n.id) : undefined;
    const flags = new Set<OverlayFlag>(changed.fileFlags.get(n.id) ?? []);
    if (counts) {
      if (counts.changed > 0) {
        flags.add("changed");
      }
      if (counts.untested > 0) {
        flags.add("untested");
      }
      if (counts.violation > 0) {
        flags.add("violation");
      }
    }
    const keep =
      !keepFiles ||
      (n.kind === "file"
        ? keepFiles.has(n.id)
        : [...keepFiles].some((p) => p.startsWith(`${n.id}/`)));
    return {
      id: n.id,
      name: baseName(n.id),
      kind: n.kind,
      value: treemapValue(n, sizeBy, state),
      group,
      tokenVar,
      flags,
      changedCount: counts?.changed ?? (flags.has("changed") ? 1 : 0),
      symbolCount:
        n.kind === "folder" ? folderSymbolCount(state, n).value : n.symbolCount,
      loc: n.loc,
      dimmed: !keep,
    };
  });
  return { cells, groups };
}
