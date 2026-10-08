/**
 * use-diff-line-reveal.ts — FE-CV-TASK-053-05
 *
 * Scrolls a diff pane to a requested line once Monaco is mounted. A request carries a nonce
 * so only a new nonce re-applies; a request made before mount waits for the editor.
 *
 * @module components/editor/use-diff-line-reveal
 */

import { useEffect, useRef } from "react";
import type { editor } from "monaco-editor";
import { suppressDiffCursorFor } from "@/lib/diff-cursor-line-bus";
import type { DiffReviewReveal } from "./diff-viewer-props";
import { useDiffCursorEmitter } from "./useDiffCursorEmitter";

export type DiffLineRevealArgs = {
  /** Truthy once Monaco has mounted (the modified editor state in DiffViewer). */
  mountedSignal: unknown;
  resolveEditor: (side: "original" | "modified") => editor.ICodeEditor | null;
  line?: number;
  side?: "original" | "modified";
  nonce?: number;
  onApplied?: (nonce: number) => void;
};

export const DIFF_REVEAL_CURSOR_SUPPRESS_MS = 500;

export function useDiffLineReveal({
  mountedSignal,
  resolveEditor,
  line,
  side = "modified",
  nonce,
  onApplied,
}: DiffLineRevealArgs): void {
  const appliedNonceRef = useRef<number | null>(null);
  const resolveRef = useRef(resolveEditor);
  resolveRef.current = resolveEditor;
  const onAppliedRef = useRef(onApplied);
  onAppliedRef.current = onApplied;

  useEffect(() => {
    if (line === undefined || nonce === undefined || !mountedSignal) {
      return;
    }
    if (appliedNonceRef.current === nonce) {
      return;
    }
    let rafId: number | null = requestAnimationFrame(() => {
      rafId = null;
      const target = resolveRef.current(side);
      const model = target?.getModel();
      if (!target || !model) {
        return;
      }
      const lineNumber = Math.min(
        Math.max(1, Math.floor(line)),
        Math.max(1, model.getLineCount()),
      );
      suppressDiffCursorFor(DIFF_REVEAL_CURSOR_SUPPRESS_MS);
      target.revealLineInCenter(lineNumber);
      target.setPosition({ lineNumber, column: 1 });
      appliedNonceRef.current = nonce;
      onAppliedRef.current?.(nonce);
    });
    return () => {
      if (rafId !== null) {
        cancelAnimationFrame(rafId);
      }
    };
  }, [mountedSignal, line, side, nonce]);
}

/** DiffViewer wiring for the Review links: line reveal (lens -> diff) and cursor emit (diff -> lens). */
export function useDiffReviewLinks(args: {
  diffEditorRef: { current: editor.IStandaloneDiffEditor | null };
  modifiedEditor: editor.ICodeEditor | null;
  reveal?: DiffReviewReveal;
  worktreeId?: string;
  relativePath: string;
}): void {
  const { diffEditorRef, modifiedEditor, reveal } = args;
  useDiffCursorEmitter(modifiedEditor, args.worktreeId, args.relativePath);
  useDiffLineReveal({
    mountedSignal: modifiedEditor,
    resolveEditor: (side) =>
      (side === "original"
        ? diffEditorRef.current?.getOriginalEditor()
        : diffEditorRef.current?.getModifiedEditor()) ?? null,
    line: reveal?.line,
    side: reveal?.side,
    nonce: reveal?.nonce,
    onApplied: reveal?.onApplied,
  });
}
