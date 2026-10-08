/**
 * review-diff-navigation.ts — FE-CV-TASK-053-06
 *
 * Opens a diff (or the plain editor) at a symbol's line for the Review lenses. No git
 * command is issued here: it only reuses the diff openers and already-loaded change lists.
 *
 * @module lib/review-diff-navigation
 */

import { detectLanguage } from "@/lib/language-detect";
import { activateAndRevealWorktree } from "@/lib/worktree-activation";
import { useAppStore } from "@/store";
import { findWorktreeById } from "@/store/slices/worktree-helpers";
import { resolveAnnotationPathInsideWorktree } from "../components/editor/check-annotation-path";
import type { ReviewScope } from "../components/review-map/review-scope-model";

export type ReviewDiffOpenResult =
  | {
      ok: true;
      opened: "branch-diff" | "working-diff" | "file";
      notInChanges: boolean;
    }
  | {
      ok: false;
      reason: "worktree-not-found" | "path-not-allowed" | "scope-unsupported";
    };

export type ReviewSymbolLocation = { filePath: string; startLine?: number };

let revealNonce = 0;
let pendingFrames: number[] = [];

function cancelPendingFrames(): void {
  for (const id of pendingFrames) {
    cancelAnimationFrame(id);
  }
  pendingFrames = [];
}

// Why: opening can replace the active tab and mount Monaco asynchronously; two frames
// (as for annotation jumps) let the destination own layout before we request the line.
function afterTwoFrames(fn: () => void): void {
  cancelPendingFrames();
  const outer = requestAnimationFrame(() => {
    const inner = requestAnimationFrame(() => {
      pendingFrames = [];
      fn();
    });
    pendingFrames.push(inner);
  });
  pendingFrames.push(outer);
}

export function openReviewDiffAtSymbol(
  worktreeId: string,
  ref: ReviewSymbolLocation,
  scope: ReviewScope | null,
  options: { line?: number } = {},
): ReviewDiffOpenResult {
  const store = useAppStore.getState();
  const worktree = findWorktreeById(store.worktreesByRepo, worktreeId);
  if (!worktree) {
    return { ok: false, reason: "worktree-not-found" };
  }
  const resolved = resolveAnnotationPathInsideWorktree(
    worktree.path,
    ref.filePath,
  );
  if (!resolved) {
    return { ok: false, reason: "path-not-allowed" };
  }
  const { absolutePath, relativePath } = resolved;
  const language = detectLanguage(relativePath);
  const line = options.line ?? ref.startLine;

  const branchEntry = (store.gitBranchChangesByWorktree[worktreeId] ?? []).find(
    (e) => e.path === relativePath,
  );
  const summary = store.gitBranchCompareSummaryByWorktree[worktreeId];
  const workingEntry = (store.gitStatusByWorktree[worktreeId] ?? []).find(
    (e) => e.path === relativePath,
  );

  activateAndRevealWorktree(worktreeId);

  let opened: "branch-diff" | "working-diff" | "file";
  if (branchEntry && summary && summary.status === "ready") {
    // Why: for range/hostedReview scopes the branch compare is a stand-in (O-13, unverified).
    store.openBranchDiff(
      worktreeId,
      worktree.path,
      branchEntry,
      summary,
      language,
    );
    opened = "branch-diff";
  } else if (
    workingEntry &&
    scope?.kind !== "range" &&
    scope?.kind !== "hostedReview"
  ) {
    store.openDiff(worktreeId, absolutePath, relativePath, language, false);
    opened = "working-diff";
  } else if (scope?.kind === "range" || scope?.kind === "hostedReview") {
    if (!workingEntry) {
      return { ok: false, reason: "scope-unsupported" };
    }
    store.openDiff(worktreeId, absolutePath, relativePath, language, false);
    opened = "working-diff";
  } else {
    store.openFile({
      filePath: absolutePath,
      relativePath,
      worktreeId,
      language,
      mode: "edit",
    });
    opened = "file";
  }

  const fileId = useAppStore.getState().activeFileId;
  if (opened === "file") {
    if (line && line > 0) {
      afterTwoFrames(() =>
        useAppStore
          .getState()
          .setPendingEditorReveal({
            filePath: absolutePath,
            line,
            column: 1,
            matchLength: 0,
          }),
      );
    }
  } else if (fileId && line && line > 0) {
    afterTwoFrames(() =>
      useAppStore
        .getState()
        .setPendingDiffReveal({
          fileId,
          line,
          side: "modified",
          nonce: ++revealNonce,
        }),
    );
  }
  return { ok: true, opened, notInChanges: opened === "file" };
}

/** Plain editor at the line; shares the path guard so it can never leave the worktree. */
export function openReviewFileInEditor(
  worktreeId: string,
  ref: ReviewSymbolLocation,
  options: { line?: number } = {},
): ReviewDiffOpenResult {
  const store = useAppStore.getState();
  const worktree = findWorktreeById(store.worktreesByRepo, worktreeId);
  if (!worktree) {
    return { ok: false, reason: "worktree-not-found" };
  }
  const resolved = resolveAnnotationPathInsideWorktree(
    worktree.path,
    ref.filePath,
  );
  if (!resolved) {
    return { ok: false, reason: "path-not-allowed" };
  }
  const line = options.line ?? ref.startLine;
  activateAndRevealWorktree(worktreeId);
  store.openFile(
    {
      filePath: resolved.absolutePath,
      relativePath: resolved.relativePath,
      worktreeId,
      language: detectLanguage(resolved.relativePath),
      mode: "edit",
    },
    { forceContentReload: true },
  );
  store.setPendingEditorReveal(null);
  if (line && line > 0) {
    afterTwoFrames(() =>
      useAppStore.getState().setPendingEditorReveal({
        filePath: resolved.absolutePath,
        line,
        column: 1,
        matchLength: 0,
      }),
    );
  }
  return { ok: true, opened: "file", notInChanges: false };
}
