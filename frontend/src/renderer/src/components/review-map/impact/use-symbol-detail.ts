/**
 * use-symbol-detail.ts — FE-CV-TASK-053-04
 *
 * Data for the symbol panel. The header call always sends includeSource:false (the contract
 * default is true); source is fetched only when its section opens and is kept in component
 * state, not in the shared query cache, so code text does not outlive the panel.
 *
 * @module components/review-map/impact/use-symbol-detail
 */

import { useCallback, useEffect, useRef, useState } from "react";
import {
  useCodeIntelQuery,
  defaultCodeIntelCall,
} from "@/hooks/useCodeIntelQuery";
import type { CodeIntelQueryError } from "@/hooks/useCodeIntelQuery";
import type {
  ImpactGraph,
  SymbolDetail,
} from "../../../../../shared/code-intel-graph-types";
import type { AmbiguousSymbolCandidate } from "../shell/AmbiguousSymbolDialog";

export type SymbolTarget = { key: string } | { name: string; file: string };

export function symbolTargetId(t: SymbolTarget): string {
  return "key" in t ? `k:${t.key}` : `n:${t.name}@${t.file}`;
}

export function useSymbolDetail(
  worktreeId: string,
  environmentId: string | null,
  target: SymbolTarget | null,
) {
  return useCodeIntelQuery<SymbolDetail>(worktreeId, environmentId, {
    method: "symbol",
    enabled: target !== null,
    params: { ...target, includeSource: false },
  });
}

/** Lazy `impact includeTests` call: only runs once the "Covering tests" section is opened. */
export function useCoveringTests(
  worktreeId: string,
  environmentId: string | null,
  target: SymbolTarget | null,
  enabled: boolean,
) {
  return useCodeIntelQuery<ImpactGraph>(worktreeId, environmentId, {
    method: "impact",
    enabled: enabled && target !== null,
    params: {
      target: target ?? {},
      direction: "upstream",
      depth: 1,
      includeTests: true,
    },
  });
}

export type SymbolSourceState =
  | { status: "idle" }
  | { status: "loading" }
  | {
      status: "ready";
      source: SymbolDetail["source"];
      omitted: SymbolDetail["sourceOmitted"];
    }
  | { status: "error"; error: CodeIntelQueryError };

export function useSymbolSource(
  worktreeId: string,
  environmentId: string | null,
  target: SymbolTarget | null,
  open: boolean,
): { state: SymbolSourceState; retry: () => void } {
  const [state, setState] = useState<SymbolSourceState>({ status: "idle" });
  const [attempt, setAttempt] = useState(0);
  const targetId = target ? symbolTargetId(target) : null;
  const targetRef = useRef(target);
  targetRef.current = target;

  useEffect(() => {
    if (!open || !targetRef.current) {
      // Why: closing drops the text immediately.
      setState({ status: "idle" });
      return;
    }
    const ctrl = new AbortController();
    setState({ status: "loading" });
    void defaultCodeIntelCall(
      worktreeId,
      "codeIntel.symbol",
      { ...targetRef.current, includeSource: true },
      ctrl.signal,
      environmentId,
    ).then(
      (outcome) => {
        if (ctrl.signal.aborted) {
          return;
        }
        if (!outcome.ok) {
          setState({ status: "error", error: outcome.error });
          return;
        }
        const detail = outcome.result as Partial<SymbolDetail> | null;
        setState({
          status: "ready",
          source: detail?.source ?? null,
          omitted: detail?.sourceOmitted ?? null,
        });
      },
      () => {
        if (!ctrl.signal.aborted) {
          setState({
            status: "error",
            error: {
              kind: "unknown",
              code: null,
              message: "Unexpected error",
              retryable: false,
            },
          });
        }
      },
    );
    return () => {
      ctrl.abort();
      setState({ status: "idle" });
    };
  }, [open, worktreeId, environmentId, targetId, attempt]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  return { state, retry };
}

/** Candidates of an `ambiguous` error, tolerant of missing/foreign shapes. */
export function ambiguousCandidates(
  error: CodeIntelQueryError,
): AmbiguousSymbolCandidate[] {
  const raw = error.data?.candidates;
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.flatMap((c): AmbiguousSymbolCandidate[] => {
    if (typeof c !== "object" || c === null) {
      return [];
    }
    const r = c as Record<string, unknown>;
    if (typeof r.name !== "string" || typeof r.filePath !== "string") {
      return [];
    }
    return [
      {
        key: typeof r.key === "string" ? r.key : undefined,
        uid: typeof r.uid === "string" ? r.uid : "",
        name: r.name,
        kind: typeof r.kind === "string" ? r.kind : "",
        filePath: r.filePath,
        line: typeof r.line === "number" ? r.line : 0,
      },
    ];
  });
}
