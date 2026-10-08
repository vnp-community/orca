import { ChevronRight } from "lucide-react";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { translate } from "@/i18n/i18n";
import { ReviewOverlayLegend } from "../ReviewOverlayLegend";
import type { OverlayFlag } from "../review-overlay-model";
import type { TreemapColorBy, TreemapSizeBy } from "./structure-treemap-model";

type Props = {
  currentPath: string;
  sizeBy: TreemapSizeBy;
  colorBy: TreemapColorBy;
  locAvailable: boolean;
  legendFlags: readonly OverlayFlag[];
  /** Color legend: group label -> token var (null = neutral). Always shown as text. */
  groups: ReadonlyMap<string, string | null>;
  onSizeBy: (v: TreemapSizeBy) => void;
  onColorBy: (v: TreemapColorBy) => void;
  onNavigate: (path: string) => void;
};

export function StructureToolbar(p: Props): React.JSX.Element {
  const t = translate;
  const segments = p.currentPath ? p.currentPath.split("/") : [];
  return (
    <div className="flex flex-col gap-2 border-b px-3 py-2">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={p.sizeBy}
          onValueChange={(v) => v && p.onSizeBy(v as TreemapSizeBy)}
          aria-label={t(
            "auto.components.reviewMap.structure.sizeBy",
            "Size by",
          )}
        >
          <ToggleGroupItem value="symbols">
            {t("auto.components.reviewMap.structure.sizeSymbols", "Symbols")}
          </ToggleGroupItem>
          <ToggleGroupItem
            value="loc"
            disabled={!p.locAvailable}
            title={
              p.locAvailable
                ? undefined
                : t(
                    "auto.components.reviewMap.structure.noLoc",
                    "The index has no line counts for this folder.",
                  )
            }
          >
            {t("auto.components.reviewMap.structure.sizeLoc", "Lines")}
          </ToggleGroupItem>
        </ToggleGroup>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          value={p.colorBy}
          onValueChange={(v) => v && p.onColorBy(v as TreemapColorBy)}
          aria-label={t(
            "auto.components.reviewMap.structure.colorBy",
            "Color by",
          )}
        >
          <ToggleGroupItem value="area">
            {t("auto.components.reviewMap.structure.colorArea", "Area")}
          </ToggleGroupItem>
          <ToggleGroupItem value="language">
            {t("auto.components.reviewMap.structure.colorLanguage", "Language")}
          </ToggleGroupItem>
        </ToggleGroup>
        <ReviewOverlayLegend flags={p.legendFlags} className="ml-auto" />
      </div>
      <nav
        aria-label={t("auto.components.reviewMap.structure.breadcrumb", "Path")}
        className="flex flex-wrap items-center gap-0.5 text-xs"
      >
        <button
          type="button"
          className="rounded px-1 hover:bg-accent"
          onClick={() => p.onNavigate("")}
        >
          {t("auto.components.reviewMap.structure.root", "Repository")}
        </button>
        {segments.map((seg, i) => {
          const path = segments.slice(0, i + 1).join("/");
          return (
            <span key={path} className="flex items-center gap-0.5">
              <ChevronRight
                className="size-3 text-muted-foreground"
                aria-hidden
              />
              <button
                type="button"
                aria-current={i === segments.length - 1 ? "page" : undefined}
                className="rounded px-1 hover:bg-accent"
                onClick={() => p.onNavigate(path)}
              >
                {seg}
              </button>
            </span>
          );
        })}
      </nav>
      {p.groups.size > 0 ? (
        <ul
          className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground"
          aria-label={t(
            "auto.components.reviewMap.structure.colorLegend",
            "Colors",
          )}
        >
          {[...p.groups.entries()].map(([label, tokenVar]) => (
            <li key={label} className="flex items-center gap-1">
              <span
                aria-hidden="true"
                className="inline-block size-3 rounded-sm border"
                style={{
                  background: tokenVar
                    ? `color-mix(in srgb, var(${tokenVar}) 28%, var(--card))`
                    : "var(--muted)",
                }}
              />
              {label}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
