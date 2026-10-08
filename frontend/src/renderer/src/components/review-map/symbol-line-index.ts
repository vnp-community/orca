/**
 * symbol-line-index.ts — FE-CV-TASK-053-07
 *
 * Per-file interval index from symbol line ranges, so a diff cursor line maps to the
 * innermost symbol. Lines are 1-based as delivered (PQ-20); no +/-1 adjustment here.
 *
 * @module components/review-map/symbol-line-index
 */

import { normalizeRuntimePathSeparators } from "../../../../shared/cross-platform-path";

export type SymbolLineEntry = {
  key: string;
  filePath: string;
  startLine?: number;
  endLine?: number;
};

export type SymbolLineIndex = ReadonlyMap<
  string,
  { key: string; start: number; end: number }[]
>;

function norm(path: string): string {
  return normalizeRuntimePathSeparators(path).replace(/^\.\//, "");
}

export function buildSymbolLineIndex(
  entries: readonly SymbolLineEntry[],
): SymbolLineIndex {
  const index = new Map<
    string,
    { key: string; start: number; end: number }[]
  >();
  const seen = new Set<string>();
  for (const e of entries) {
    if (!e.startLine || e.startLine < 1) {
      continue;
    }
    const end = e.endLine && e.endLine >= e.startLine ? e.endLine : e.startLine;
    const path = norm(e.filePath);
    const dedupe = `${path}\0${e.key}`;
    if (seen.has(dedupe)) {
      continue;
    }
    seen.add(dedupe);
    const list = index.get(path) ?? [];
    list.push({ key: e.key, start: e.startLine, end });
    index.set(path, list);
  }
  return index;
}

/** Narrowest range containing `line`; ties resolve by key so the answer is stable. */
export function findInnermostSymbolAtLine(
  index: SymbolLineIndex,
  relativePath: string,
  line: number,
): string | null {
  const list = index.get(norm(relativePath));
  if (!list) {
    return null;
  }
  let best: { key: string; size: number } | null = null;
  for (const r of list) {
    if (line < r.start || line > r.end) {
      continue;
    }
    const size = r.end - r.start;
    if (!best || size < best.size || (size === best.size && r.key < best.key)) {
      best = { key: r.key, size };
    }
  }
  return best?.key ?? null;
}
