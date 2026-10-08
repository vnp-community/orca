/**
 * useDiffPopoverTracking.ts
 *
 * Keeps the diff-comment popover glued to its line while the modified editor scrolls,
 * resizes or relayouts. Extracted from DiffViewer to keep that file under the line cap.
 *
 * @module components/editor/useDiffPopoverTracking
 */

import { useEffect } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";
import type { editor } from "monaco-editor";
import { monaco } from "@/lib/monaco-setup";
import {
  getDiffCommentPopoverLeft,
  getDiffCommentPopoverTop,
} from "../diff-comments/diff-comment-popover-position";

export type DiffCommentPopoverState = {
  lineNumber: number;
  startLine?: number;
  top: number;
  left?: number;
  lineHeight: number;
};

export function useDiffPopoverTracking(args: {
  modifiedEditor: editor.ICodeEditor | null;
  popover: DiffCommentPopoverState | null;
  setPopover: Dispatch<SetStateAction<DiffCommentPopoverState | null>>;
  diffBodyRef: RefObject<HTMLDivElement | null>;
}): void {
  const { modifiedEditor, popover, setPopover, diffBodyRef } = args;
  useEffect(() => {
    if (!modifiedEditor || !popover) {
      return;
    }
    const update = (): void => {
      const lineHeight = modifiedEditor.getOption(
        monaco.editor.EditorOption.lineHeight,
      );
      const top = getDiffCommentPopoverTop(
        modifiedEditor,
        popover.lineNumber,
        lineHeight,
      );
      if (top == null) {
        setPopover(null);
        return;
      }
      const left = getDiffCommentPopoverLeft(
        modifiedEditor,
        diffBodyRef.current,
      );
      setPopover((prev) =>
        prev
          ? { ...prev, top, left: left == null ? prev.left : left, lineHeight }
          : prev,
      );
    };
    const scrollSub = modifiedEditor.onDidScrollChange(update);
    const contentSub = modifiedEditor.onDidContentSizeChange(update);
    const layoutSub = modifiedEditor.onDidLayoutChange(update);
    return () => {
      scrollSub.dispose();
      contentSub.dispose();
      layoutSub.dispose();
    };
    // Why: depend on popover.lineNumber (not the whole popover object) so the
    // effect doesn't re-subscribe on every top update it dispatches.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [modifiedEditor, popover?.lineNumber]);
}
