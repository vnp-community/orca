/**
 * impact-lens-model.ts — FE-CV-TASK-053-03
 *
 * Pure helpers of the Impact lens: centre choice, depth guard, affected-set derivation.
 *
 * @module components/review-map/impact/impact-lens-model
 */

import type {
  ImpactGraph,
  RiskLevel,
} from "../../../../../shared/code-intel-graph-types";
import type { ChangeOverlayView, SymbolRefView } from "../review-wire-types";
import { countImpactNodes } from "./impact-column-layout";
import { IMPACT_DEPTH_MAX, IMPACT_DEPTH_MIN } from "./ImpactToolbar";

/** The contract rejects (does not clamp) depth outside 1..3, so never send anything else. */
export function clampImpactDepth(depth: number): number {
  if (!Number.isFinite(depth)) {
    return 2;
  }
  return Math.min(
    IMPACT_DEPTH_MAX,
    Math.max(IMPACT_DEPTH_MIN, Math.round(depth)),
  );
}

/** Explicit focus wins; otherwise follow the selection if it is a changed symbol. */
export function resolveImpactCenter(
  focusKey: string | null,
  selectedKey: string | null,
  overlay: Pick<ChangeOverlayView, "changedSymbols">,
): string | null {
  if (focusKey) {
    return focusKey;
  }
  if (
    selectedKey &&
    overlay.changedSymbols.some((c) => c.symbol.key === selectedKey)
  ) {
    return selectedKey;
  }
  return null;
}

export function findChangedSymbol(
  overlay: Pick<ChangeOverlayView, "changedSymbols">,
  key: string,
): SymbolRefView | null {
  return (
    overlay.changedSymbols.find((c) => c.symbol.key === key)?.symbol ?? null
  );
}

/** Symbols reached by impact queries that are not themselves changed. */
export function collectAffectedKeys(
  graphs: readonly (ImpactGraph | null)[],
  overlay: Pick<ChangeOverlayView, "changedSymbols">,
): Set<string> {
  const changed = new Set(overlay.changedSymbols.map((c) => c.symbol.key));
  const out = new Set<string>();
  for (const g of graphs) {
    for (const level of g?.levels ?? []) {
      for (const s of level.symbols) {
        if (!changed.has(s.symbol.key)) {
          out.add(s.symbol.key);
        }
      }
    }
  }
  return out;
}

export const IMPACT_LIST_DEFAULT_AFTER = 300;

export function defaultImpactView(
  ...graphs: (ImpactGraph | null)[]
): "graph" | "list" {
  return countImpactNodes(...graphs) > IMPACT_LIST_DEFAULT_AFTER
    ? "list"
    : "graph";
}

export function describeRisk(
  risk: RiskLevel | undefined,
): { id: string; fallback: string } | null {
  switch (risk) {
    case "LOW":
      return { id: "low", fallback: "Low risk" };
    case "MEDIUM":
      return { id: "medium", fallback: "Medium risk" };
    case "HIGH":
      return { id: "high", fallback: "High risk" };
    case "CRITICAL":
      return { id: "critical", fallback: "Critical risk" };
    case "UNKNOWN":
      return { id: "unknown", fallback: "Not enough data" };
    default:
      return null;
  }
}
