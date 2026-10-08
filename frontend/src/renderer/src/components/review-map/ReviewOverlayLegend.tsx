import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { translate } from "@/i18n/i18n";
import { OVERLAY_ENCODING } from "./review-overlay-model";
import type { OverlayFlag } from "./review-overlay-model";

type Props = {
  /** Only flags with data are listed (see overlayFlagsWithData). */
  flags: readonly OverlayFlag[];
  className?: string;
};

/** Legend built from OVERLAY_ENCODING itself so it can never disagree with the marks. */
export function ReviewOverlayLegend({
  flags,
  className,
}: Props): React.JSX.Element | null {
  if (flags.length === 0) {
    return null;
  }
  // Why: own provider so the legend also works when mounted outside the app shell (tests, previews).
  return (
    <TooltipProvider>
      <ul
        className={`flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground ${className ?? ""}`}
        aria-label={translate(
          "auto.components.reviewMap.overlay.legend.title",
          "Legend",
        )}
      >
        {flags.map((flag) => {
          const enc = OVERLAY_ENCODING[flag];
          const Icon = enc.icon;
          return (
            <li key={flag}>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="inline-flex items-center gap-1">
                    <span
                      aria-hidden="true"
                      className={`inline-flex size-4 items-center justify-center rounded-sm ${enc.borderClass} ${
                        enc.strokeWidth >= 2 ? "border-2" : "border"
                      } ${enc.dashed ? "border-dashed" : ""}`}
                    >
                      <Icon className={`size-3 ${enc.textClass}`} />
                    </span>
                    <span>{translate(enc.labelKey, enc.labelFallback)}</span>
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  {translate(enc.descriptionKey, enc.descriptionFallback)}
                </TooltipContent>
              </Tooltip>
            </li>
          );
        })}
      </ul>
    </TooltipProvider>
  );
}
