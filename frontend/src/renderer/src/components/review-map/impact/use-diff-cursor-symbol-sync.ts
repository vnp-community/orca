/**
 * use-diff-cursor-symbol-sync.ts — FE-CV-TASK-053-07
 *
 * Diff -> graph link: the diff cursor line selects the innermost changed symbol
 * (highlight only; the drawer stays closed and focus stays in the diff).
 *
 * @module components/review-map/impact/use-diff-cursor-symbol-sync
 */

import { useEffect, useMemo } from "react";
import { useAppStore } from "@/store";
import { subscribeDiffCursorLine } from "@/lib/diff-cursor-line-bus";
import {
  buildSymbolLineIndex,
  findInnermostSymbolAtLine,
} from "../symbol-line-index";
import type { SymbolLineEntry } from "../symbol-line-index";

export function useDiffCursorSymbolSync(
  worktreeId: string,
  symbols: readonly SymbolLineEntry[],
): void {
  const index = useMemo(() => buildSymbolLineIndex(symbols), [symbols]);
  useEffect(
    () =>
      subscribeDiffCursorLine((event) => {
        if (event.worktreeId && event.worktreeId !== worktreeId) {
          return;
        }
        const key = findInnermostSymbolAtLine(
          index,
          event.relativePath,
          event.line,
        );
        // No symbol under the cursor: keep the current selection.
        if (key) {
          useAppStore.getState().selectReviewSymbolFromDiff(worktreeId, key);
        }
      }),
    [worktreeId, index],
  );
}
