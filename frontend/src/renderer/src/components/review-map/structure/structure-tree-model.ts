/**
 * structure-tree-model.ts — FE-CV-TASK-054-03
 *
 * Pure model of the lazily loaded structure tree: path guard, children lookup and the
 * flattened row list (including loading / error / "load more" rows) that the virtualized
 * tree renders.
 *
 * @module components/review-map/structure/structure-tree-model
 */

import type { ModuleNode } from "../../../../../shared/code-intel-graph-types";
import { normalizeStructurePath } from "./structure-area-model";

export const ROOT_PATH = "";
export const STRUCTURE_MAX_NODES = 5000;
export const STRUCTURE_ROOT_DEPTH = 2;
export const STRUCTURE_FOLDER_DEPTH = 1;

export type FolderLoad = {
  status: "loading" | "ready" | "error";
  nextPageToken?: string;
  /** Total children the backend reports (0/undefined = unknown). */
  totalCount?: number;
  /** Children loaded from this folder's own responses. */
  loaded: number;
  errorMessage?: string;
  loadingMore?: boolean;
};

export type StructureTreeState = {
  nodes: ReadonlyMap<string, ModuleNode>;
  folders: ReadonlyMap<string, FolderLoad>;
};

export type StructureNodeRow = {
  kind: "folder" | "file";
  key: string;
  path: string;
  name: string;
  level: number;
  expanded: boolean;
  setsize: number;
  posinset: number;
  node: ModuleNode;
};

export type StructureStatusRow = {
  kind: "loading" | "error" | "more";
  key: string;
  path: string;
  level: number;
  shown?: number;
  total?: number;
  message?: string;
};

export type StructureRow = StructureNodeRow | StructureStatusRow;

export function isNodeRow(row: StructureRow): row is StructureNodeRow {
  return row.kind === "folder" || row.kind === "file";
}

/** Relative POSIX path with no traversal; anything else is never sent to the backend. */
export function isSafeStructurePath(path: string): boolean {
  if (path === ROOT_PATH) {
    return true;
  }
  if (
    path.includes("\0") ||
    path.includes("\\") ||
    path.startsWith("/") ||
    /^[A-Za-z]:/.test(path)
  ) {
    return false;
  }
  return !path.split("/").some((s) => s === ".." || s === "." || s === "");
}

export function parentPath(path: string): string {
  const i = path.lastIndexOf("/");
  return i < 0 ? ROOT_PATH : path.slice(0, i);
}

/** Nearest ancestor that exists as a returned folder (the root when none does). */
export function nearestLoadedParent(
  state: StructureTreeState,
  path: string,
): string {
  let parent = parentPath(path);
  while (parent !== ROOT_PATH && state.nodes.get(parent)?.kind !== "folder") {
    parent = parentPath(parent);
  }
  return parent;
}

export function baseName(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1);
}

export function normalizeModuleNodes(
  nodes: readonly ModuleNode[],
): ModuleNode[] {
  return nodes
    .map((n) => ({ ...n, id: normalizeStructurePath(n.id) }))
    .filter((n) => n.id !== "" && isSafeStructurePath(n.id));
}

const childIndexCache = new WeakMap<
  ReadonlyMap<string, ModuleNode>,
  Map<string, ModuleNode[]>
>();

function compareNodes(a: ModuleNode, b: ModuleNode): number {
  return a.kind !== b.kind
    ? a.kind === "folder"
      ? -1
      : 1
    : a.id < b.id
      ? -1
      : a.id > b.id
        ? 1
        : 0;
}

/**
 * Parent -> children, built once per node map. A node whose direct parent folder was not
 * returned hangs under its nearest returned ancestor (or the root), so a sparse response
 * still shows up instead of vanishing.
 */
function childIndex(
  nodes: ReadonlyMap<string, ModuleNode>,
): Map<string, ModuleNode[]> {
  const cached = childIndexCache.get(nodes);
  if (cached) {
    return cached;
  }
  const index = new Map<string, ModuleNode[]>();
  for (const node of nodes.values()) {
    let parent = parentPath(node.id);
    while (parent !== ROOT_PATH && nodes.get(parent)?.kind !== "folder") {
      parent = parentPath(parent);
    }
    const list = index.get(parent) ?? [];
    list.push(node);
    index.set(parent, list);
  }
  for (const list of index.values()) {
    list.sort(compareNodes);
  }
  childIndexCache.set(nodes, index);
  return index;
}

export function childrenOf(
  state: StructureTreeState,
  folder: string,
): ModuleNode[] {
  return childIndex(state.nodes).get(folder) ?? [];
}

/** A folder's own symbolCount when the backend gave one, else the sum of loaded children ("at least"). */
export function folderSymbolCount(
  state: StructureTreeState,
  folder: ModuleNode,
): { value: number; approximate: boolean } {
  if (folder.symbolCount > 0) {
    return { value: folder.symbolCount, approximate: false };
  }
  const kids = childrenOf(state, folder.id);
  const f = state.folders.get(folder.id);
  return {
    value: kids.reduce((n, k) => n + k.symbolCount, 0),
    approximate: !f || f.status !== "ready" || Boolean(f.nextPageToken),
  };
}

/** Keep a branch only when it holds a file in `keep` (chip filter); null = no filter. */
function branchVisible(
  node: ModuleNode,
  keep: ReadonlySet<string> | null,
): boolean {
  if (!keep) {
    return true;
  }
  if (node.kind === "file") {
    return keep.has(node.id);
  }
  const prefix = `${node.id}/`;
  for (const p of keep) {
    if (p.startsWith(prefix)) {
      return true;
    }
  }
  return false;
}

export function flattenStructureRows(
  state: StructureTreeState,
  expanded: ReadonlySet<string>,
  keepFiles: ReadonlySet<string> | null = null,
): StructureRow[] {
  const rows: StructureRow[] = [];
  const walk = (folder: string, level: number): void => {
    const kids = childrenOf(state, folder).filter((n) =>
      branchVisible(n, keepFiles),
    );
    kids.forEach((node, i) => {
      const isOpen = node.kind === "folder" && expanded.has(node.id);
      rows.push({
        kind: node.kind,
        key: node.id,
        path: node.id,
        name: baseName(node.id),
        level,
        expanded: isOpen,
        setsize: kids.length,
        posinset: i + 1,
        node,
      });
      if (isOpen) {
        walk(node.id, level + 1);
      }
    });
    const load = state.folders.get(folder);
    if (!load) {
      return;
    }
    if (load.status === "loading" && !load.loadingMore) {
      rows.push({
        kind: "loading",
        key: `${folder}\0loading`,
        path: folder,
        level,
      });
    } else if (load.status === "error") {
      rows.push({
        kind: "error",
        key: `${folder}\0error`,
        path: folder,
        level,
        message: load.errorMessage,
      });
    } else if (load.nextPageToken) {
      rows.push({
        kind: "more",
        key: `${folder}\0more`,
        path: folder,
        level,
        shown: load.loaded,
        total:
          load.totalCount && load.totalCount > 0 ? load.totalCount : undefined,
      });
    }
  };
  walk(ROOT_PATH, 1);
  return rows;
}
