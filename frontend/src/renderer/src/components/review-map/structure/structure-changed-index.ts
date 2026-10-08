/**
 * structure-changed-index.ts — FE-CV-TASK-054-02
 *
 * Path index of the change overlay for the Structure lens: per-file flags and per-folder
 * counts (every ancestor), plus deleted files, which have no cell to mark.
 *
 * @module components/review-map/structure/structure-changed-index
 */

import type { OverlayFlag } from "../review-overlay-model";
import type { ChangedFileView } from "../review-wire-types";
import { normalizeStructurePath } from "./structure-area-model";

export type FolderFlagCounts = {
  changed: number;
  untested: number;
  violation: number;
};

export type ChangedPathIndex = {
  fileFlags: ReadonlyMap<string, ReadonlySet<OverlayFlag>>;
  folderCounts: ReadonlyMap<string, FolderFlagCounts>;
  deletedFiles: readonly string[];
};

export const ROOT_FOLDER = "";

function ancestors(path: string): string[] {
  const parts = path.split("/");
  parts.pop();
  const out = [ROOT_FOLDER];
  for (let i = 1; i <= parts.length; i++) {
    out.push(parts.slice(0, i).join("/"));
  }
  return out;
}

export function buildChangedPathIndex(
  changedFiles: readonly Pick<ChangedFileView, "path" | "status">[],
  uncoveredFiles: Iterable<string>,
  violationFiles: Iterable<string>,
): ChangedPathIndex {
  const fileFlags = new Map<string, Set<OverlayFlag>>();
  const deleted: string[] = [];
  const add = (path: string, flag: OverlayFlag): void => {
    const set = fileFlags.get(path) ?? new Set<OverlayFlag>();
    set.add(flag);
    fileFlags.set(path, set);
  };
  for (const f of changedFiles) {
    const p = normalizeStructurePath(f.path);
    if (f.status === "deleted") {
      deleted.push(p);
    } else {
      add(p, "changed");
    }
  }
  const deletedSet = new Set(deleted);
  for (const u of uncoveredFiles) {
    const p = normalizeStructurePath(u);
    if (!deletedSet.has(p)) {
      add(p, "untested");
    }
  }
  for (const v of violationFiles) {
    const p = normalizeStructurePath(v);
    if (!deletedSet.has(p)) {
      add(p, "violation");
    }
  }
  const folderCounts = new Map<string, FolderFlagCounts>();
  for (const [path, flags] of fileFlags) {
    for (const folder of ancestors(path)) {
      const c = folderCounts.get(folder) ?? {
        changed: 0,
        untested: 0,
        violation: 0,
      };
      if (flags.has("changed")) {
        c.changed++;
      }
      if (flags.has("untested")) {
        c.untested++;
      }
      if (flags.has("violation")) {
        c.violation++;
      }
      folderCounts.set(folder, c);
    }
  }
  return { fileFlags, folderCounts, deletedFiles: deleted.sort() };
}
