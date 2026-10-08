import type { ReactNode } from "react";
import type { SymbolDetail } from "../../../../../shared/code-intel-graph-types";

export type SymbolDetailActionsRenderer = (args: {
  worktreeId: string;
  symbol: SymbolDetail["symbol"];
}) => ReactNode;

let renderer: SymbolDetailActionsRenderer | null = null;

/** Extension point for other review features (e.g. quality actions) to add panel actions. */
export function setSymbolDetailActionsRenderer(
  next: SymbolDetailActionsRenderer | null,
): void {
  renderer = next;
}

export function SymbolDetailActionsSlot(props: {
  worktreeId: string;
  symbol: SymbolDetail["symbol"];
}): React.JSX.Element | null {
  return renderer ? <>{renderer(props)}</> : null;
}
