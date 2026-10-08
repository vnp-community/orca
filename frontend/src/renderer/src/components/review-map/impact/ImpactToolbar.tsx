import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { translate } from "@/i18n/i18n";
import { ReviewOverlayLegend } from "../ReviewOverlayLegend";
import type { OverlayFlag } from "../review-overlay-model";

export type ImpactDirection = "both" | "upstream" | "downstream";
export type ImpactView = "graph" | "list";
export const IMPACT_DEPTH_MIN = 1;
export const IMPACT_DEPTH_MAX = 3;

type Props = {
  direction: ImpactDirection;
  depth: number;
  view: ImpactView;
  legendFlags: readonly OverlayFlag[];
  onDirection: (d: ImpactDirection) => void;
  onDepth: (d: number) => void;
  onView: (v: ImpactView) => void;
};

export function ImpactToolbar(p: Props): React.JSX.Element {
  const t = translate;
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b px-3 py-2">
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={p.direction}
        onValueChange={(v) => v && p.onDirection(v as ImpactDirection)}
        aria-label={t(
          "auto.components.reviewMap.impact.direction",
          "Direction",
        )}
      >
        <ToggleGroupItem value="upstream">
          {t("auto.components.reviewMap.impact.upstream", "Callers")}
        </ToggleGroupItem>
        <ToggleGroupItem value="both">
          {t("auto.components.reviewMap.impact.both", "Both")}
        </ToggleGroupItem>
        <ToggleGroupItem value="downstream">
          {t("auto.components.reviewMap.impact.downstream", "Dependencies")}
        </ToggleGroupItem>
      </ToggleGroup>
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={String(p.depth)}
        onValueChange={(v) => v && p.onDepth(Number(v))}
        aria-label={t("auto.components.reviewMap.impact.depth", "Depth")}
      >
        {[1, 2, 3].map((d) => (
          <ToggleGroupItem key={d} value={String(d)}>
            {d}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        value={p.view}
        onValueChange={(v) => v && p.onView(v as ImpactView)}
        aria-label={t("auto.components.reviewMap.impact.view", "View")}
      >
        <ToggleGroupItem value="graph">
          {t("auto.components.reviewMap.impact.viewGraph", "Graph")}
        </ToggleGroupItem>
        <ToggleGroupItem value="list">
          {t("auto.components.reviewMap.impact.viewList", "List")}
        </ToggleGroupItem>
      </ToggleGroup>
      <ReviewOverlayLegend flags={p.legendFlags} className="ml-auto" />
    </div>
  );
}
