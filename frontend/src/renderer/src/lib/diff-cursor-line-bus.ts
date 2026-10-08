/**
 * diff-cursor-line-bus.ts — FE-CV-TASK-053-07
 *
 * Tiny pub/sub carrying the diff cursor line to the Review workspace (diff -> graph link).
 * Why a bus with listener counting: DiffViewer is used everywhere, so it must do no work
 * (no Monaco subscription, no timers) unless a Review workspace is actually listening.
 *
 * @module lib/diff-cursor-line-bus
 */

export type DiffCursorLineEvent = {
  worktreeId?: string;
  /** Worktree-relative path, POSIX separators. */
  relativePath: string;
  /** 1-based line of the modified side. */
  line: number;
};

type LineListener = (event: DiffCursorLineEvent) => void;
type CountListener = (hasListeners: boolean) => void;

const lineListeners = new Set<LineListener>();
const countListeners = new Set<CountListener>();
let suppressUntil = 0;

export function hasDiffCursorListeners(): boolean {
  return lineListeners.size > 0;
}

function notifyCount(): void {
  const has = lineListeners.size > 0;
  for (const l of Array.from(countListeners)) {
    l(has);
  }
}

export function subscribeDiffCursorLine(listener: LineListener): () => void {
  lineListeners.add(listener);
  notifyCount();
  return () => {
    lineListeners.delete(listener);
    notifyCount();
  };
}

/** Lets emitters attach their Monaco subscription lazily. Fires immediately with the current state. */
export function onDiffCursorListenersChange(
  listener: CountListener,
): () => void {
  countListeners.add(listener);
  listener(lineListeners.size > 0);
  return () => {
    countListeners.delete(listener);
  };
}

/** Ignore cursor events for `ms` (a programmatic reveal moves the cursor; that must not echo back). */
export function suppressDiffCursorFor(
  ms: number,
  now: number = Date.now(),
): void {
  suppressUntil = Math.max(suppressUntil, now + ms);
}

export function isDiffCursorSuppressed(now: number = Date.now()): boolean {
  return now < suppressUntil;
}

export function emitDiffCursorLine(event: DiffCursorLineEvent): void {
  if (lineListeners.size === 0 || isDiffCursorSuppressed()) {
    return;
  }
  for (const listener of Array.from(lineListeners)) {
    try {
      listener(event);
    } catch (error) {
      // Why: one broken listener must not starve the others.
      console.error("[diff-cursor-line-bus] listener failed", error);
    }
  }
}

export function resetDiffCursorLineBusForTests(): void {
  lineListeners.clear();
  countListeners.clear();
  suppressUntil = 0;
}
