import { useState } from "react";
import { Button } from "@/components/ui/button";
import { translate } from "@/i18n/i18n";
import type { SymbolNeighbor } from "../../../../../shared/code-intel-graph-types";

export const SYMBOL_RELATION_PAGE = 20;

type Props = {
  groups: Record<string, SymbolNeighbor[]>;
  onSelect: (neighbor: SymbolNeighbor) => void;
};

/** Callers/callees grouped by edge kind; at most 20 per group until "show more". */
export function SymbolRelationList({
  groups,
  onSelect,
}: Props): React.JSX.Element {
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const kinds = Object.keys(groups).filter((k) => (groups[k]?.length ?? 0) > 0);
  if (kinds.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          "auto.components.reviewMap.symbolDetail.relationsEmpty",
          "None found in the index.",
        )}
      </p>
    );
  }
  return (
    <div className="flex flex-col gap-2">
      {kinds.map((kind) => {
        const all = groups[kind];
        const shown = expanded.has(kind)
          ? all
          : all.slice(0, SYMBOL_RELATION_PAGE);
        return (
          <div key={kind}>
            <div className="text-xs text-muted-foreground">
              {kind} · {all.length}
            </div>
            <ul>
              {shown.map((n, i) => (
                <li key={`${n.uid ?? n.key ?? n.name}:${n.filePath}:${i}`}>
                  <button
                    type="button"
                    onClick={() => onSelect(n)}
                    className="flex w-full min-w-0 flex-col rounded px-1 py-0.5 text-left text-xs hover:bg-accent"
                  >
                    <span className="truncate">{n.name}</span>
                    <span className="truncate text-muted-foreground">
                      {n.filePath}
                      {n.line ? `:${n.line}` : ""}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
            {all.length > shown.length ? (
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setExpanded(new Set(expanded).add(kind))}
              >
                {translate(
                  "auto.components.reviewMap.symbolDetail.showMore",
                  "Show {{count}} more",
                  {
                    count: all.length - shown.length,
                  },
                )}
              </Button>
            ) : null}
          </div>
        );
      })}
    </div>
  );
}
