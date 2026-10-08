import { Button } from "@/components/ui/button";
import { translate } from "@/i18n/i18n";
import { computeOverlayFlags } from "../review-overlay-model";
import type { OverlayImpactInput } from "../review-overlay-model";
import type { ChangeOverlayView } from "../review-wire-types";
import type { ImpactLayout } from "./impact-column-layout";
import { ImpactNodeCard } from "./ImpactNodeCard";
import type { ImpactNodeActions } from "./ImpactNodeCard";

type Props = {
  layout: ImpactLayout;
  overlay: ChangeOverlayView;
  impact: OverlayImpactInput;
  selectedKey: string | null;
  actions: ImpactNodeActions;
  onExpandColumn: (column: number) => void;
};

function columnTitle(column: number): string {
  if (column === 0) {
    return translate(
      "auto.components.reviewMap.impact.columnCenter",
      "Selected symbol",
    );
  }
  return column < 0
    ? translate(
        "auto.components.reviewMap.impact.columnUp",
        "Upstream, level {{level}}",
        {
          level: -column,
        },
      )
    : translate(
        "auto.components.reviewMap.impact.columnDown",
        "Downstream, level {{level}}",
        {
          level: column,
        },
      );
}

/** Accessible alternative to the canvas: the same columns as plain lists. */
export function ImpactColumnsList({
  layout,
  overlay,
  impact,
  selectedKey,
  actions,
  onExpandColumn,
}: Props): React.JSX.Element {
  return (
    <div
      className="flex h-full min-h-0 flex-col gap-4 overflow-auto p-3"
      data-testid="impact-list"
    >
      {layout.columns.map(({ column }) => {
        const nodes = layout.nodes.filter((n) => n.column === column);
        return (
          <section key={column} aria-label={columnTitle(column)}>
            <h3 className="mb-1 text-xs font-medium text-muted-foreground">
              {columnTitle(column)}
            </h3>
            <ul className="flex flex-col gap-1">
              {nodes.map((n) =>
                n.type === "more" ? (
                  <li key={n.id}>
                    <Button
                      type="button"
                      size="sm"
                      variant="ghost"
                      onClick={() => onExpandColumn(column)}
                    >
                      {translate(
                        "auto.components.reviewMap.impact.more",
                        "+{{count}} more",
                        {
                          count: n.hiddenCount,
                        },
                      )}
                    </Button>
                  </li>
                ) : (
                  <li key={n.id}>
                    <ImpactNodeCard
                      symbol={n.symbol}
                      isCenter={n.type === "center"}
                      direct={n.type === "symbol" ? n.direct : undefined}
                      via={n.type === "symbol" ? n.via : undefined}
                      selected={selectedKey === n.symbol.key}
                      flags={computeOverlayFlags(
                        { symbolKey: n.symbol.key, file: n.symbol.filePath },
                        overlay,
                        impact,
                      )}
                      actions={actions}
                    />
                  </li>
                ),
              )}
            </ul>
          </section>
        );
      })}
    </div>
  );
}
