/**
 * structure-area-model.ts — FE-CV-TASK-054-02
 *
 * Which "area" a node belongs to and which colour token it gets. Colours are assigned by
 * alphabetical order of the areas currently visible, so the mapping is explainable and
 * identical for everyone; text labels always accompany colour.
 *
 * @module components/review-map/structure/structure-area-model
 */

import { normalizeRuntimePathSeparators } from "../../../../../shared/cross-platform-path";

export const ROOT_AREA = "(root)";
export const AREA_COLOR_TOKENS = [
  "--review-area-1",
  "--review-area-2",
  "--review-area-3",
  "--review-area-4",
  "--review-area-5",
] as const;
export const OVERFLOW_AREA_TOKEN = "--review-area-6";

export function normalizeStructurePath(path: string): string {
  return normalizeRuntimePathSeparators(path)
    .replace(/^\.\//, "")
    .replace(/\/+$/, "");
}

function areaFromPrefix(path: string): string {
  const parts = normalizeStructurePath(path).split("/").filter(Boolean);
  if (parts.length <= 1 && !path.endsWith("/")) {
    // A top-level file belongs to the root; a top-level folder is its own area.
    return parts.length === 1 && !parts[0].includes(".") ? parts[0] : ROOT_AREA;
  }
  // backend-go/services/<name>/... => backend-go/<name>
  if (parts[1] === "services" && parts.length >= 3) {
    return `${parts[0]}/${parts[2]}`;
  }
  return parts[0];
}

/** Priority: ModuleNode.area, then ChangedFile.area for that path, then the path prefix. */
export function deriveStructureArea(
  node: { id: string; area?: string } | string,
  changedFiles: readonly { path: string; area?: string }[] = [],
): string {
  const id = normalizeStructurePath(typeof node === "string" ? node : node.id);
  if (typeof node !== "string" && node.area) {
    return node.area;
  }
  const changed = changedFiles.find(
    (f) => normalizeStructurePath(f.path) === id && f.area,
  );
  return changed?.area ?? areaFromPrefix(id);
}

export type ColorAssignment = { tokenVar: string; index: number };

/** Alphabetical order of visible areas -> area-1..5; the rest share area-6. */
export function assignAreaColors(
  visibleAreas: Iterable<string>,
): Map<string, ColorAssignment> {
  const sorted = [...new Set(visibleAreas)].sort((a, b) =>
    a < b ? -1 : a > b ? 1 : 0,
  );
  return new Map(
    sorted.map((area, i) => [
      area,
      i < AREA_COLOR_TOKENS.length
        ? { tokenVar: AREA_COLOR_TOKENS[i], index: i }
        : { tokenVar: OVERFLOW_AREA_TOKEN, index: AREA_COLOR_TOKENS.length },
    ]),
  );
}

/** Top-5 languages by count (ties alphabetical) get a token; the rest share area-6. */
export function assignLanguageColors(
  counts: ReadonlyMap<string, number>,
): Map<string, ColorAssignment> {
  const ranked = [...counts.entries()].sort(
    (a, b) => b[1] - a[1] || (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : 0),
  );
  return new Map(
    ranked.map(([lang], i) => [
      lang,
      i < AREA_COLOR_TOKENS.length
        ? { tokenVar: AREA_COLOR_TOKENS[i], index: i }
        : { tokenVar: OVERFLOW_AREA_TOKEN, index: AREA_COLOR_TOKENS.length },
    ]),
  );
}
