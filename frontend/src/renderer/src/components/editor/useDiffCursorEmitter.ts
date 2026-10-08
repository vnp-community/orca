/**
 * useDiffCursorEmitter.ts — FE-CV-TASK-053-07
 *
 * Publishes the modified-side cursor line to the diff cursor bus (debounced). Attaches the
 * Monaco listener only while somebody listens, so ordinary diffs pay nothing.
 *
 * @module components/editor/useDiffCursorEmitter
 */

import { useEffect } from "react";
import type { editor } from "monaco-editor";
import {
  emitDiffCursorLine,
  onDiffCursorListenersChange,
} from "@/lib/diff-cursor-line-bus";

export const DIFF_CURSOR_DEBOUNCE_MS = 150;

export function useDiffCursorEmitter(
  modifiedEditor: editor.ICodeEditor | null,
  worktreeId: string | undefined,
  relativePath: string,
): void {
  useEffect(() => {
    if (!modifiedEditor) {
      return;
    }
    let cursorSub: { dispose: () => void } | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const detach = (): void => {
      cursorSub?.dispose();
      cursorSub = null;
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
    };
    const stopWatching = onDiffCursorListenersChange((hasListeners) => {
      if (!hasListeners) {
        detach();
        return;
      }
      if (cursorSub) {
        return;
      }
      cursorSub = modifiedEditor.onDidChangeCursorPosition((e) => {
        if (timer !== null) {
          clearTimeout(timer);
        }
        const line = e.position.lineNumber;
        timer = setTimeout(() => {
          timer = null;
          emitDiffCursorLine({ worktreeId, relativePath, line });
        }, DIFF_CURSOR_DEBOUNCE_MS);
      });
    });
    return () => {
      stopWatching();
      detach();
    };
  }, [modifiedEditor, worktreeId, relativePath]);
}
