import { Checkbox } from "@/components/ui/checkbox";
import { translate } from "@/i18n/i18n";
import { basename, dirname } from "@/lib/path";
import { readingReasonLabel } from "../reading-reason-labels";
import { OVERLAY_ENCODING, type OverlayFlag } from "../review-overlay-model";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ReadingOrderItem } from "../reading-order-model";

const STATUS_LETTER: Record<ReadingOrderItem["fileStatus"], string> = {
  added: "A",
  modified: "M",
  deleted: "D",
  renamed: "R",
  copied: "C",
  untracked: "U",
  unknown: "?",
};

// Git decoration tokens are reserved for git status, which is exactly what A/M/D mean here.
const STATUS_COLOR: Partial<Record<ReadingOrderItem["fileStatus"], string>> = {
  added: "var(--git-decoration-added)",
  modified: "var(--git-decoration-modified)",
  deleted: "var(--git-decoration-deleted)",
  renamed: "var(--git-decoration-renamed)",
  untracked: "var(--git-decoration-untracked)",
  copied: "var(--git-decoration-copied)",
};

type Props = {
  item: ReadingOrderItem;
  seen: boolean;
  active: boolean;
  domId: string;
  /** Overlay marks (untested/violation); icon + text label, never colour alone. */
  overlayFlags?: readonly OverlayFlag[];
  onSelect: () => void;
  onToggleSeen: () => void;
  onOpenDiff: () => void;
};

export function ReadingOrderRow({
  item,
  seen,
  active,
  domId,
  overlayFlags,
  onSelect,
  onToggleSeen,
  onOpenDiff,
}: Props): React.JSX.Element {
  const dir = dirname(item.file);
  return (
    <div
      id={domId}
      role="option"
      aria-selected={active}
      data-row="step"
      data-seen={seen}
      onClick={onSelect}
      onDoubleClick={onOpenDiff}
      className={`flex h-10 items-center gap-2 px-3 text-xs ${active ? "bg-accent" : ""}`}
    >
      <Checkbox
        tabIndex={-1}
        checked={seen}
        onCheckedChange={onToggleSeen}
        onClick={(e) => e.stopPropagation()}
        aria-label={translate(
          "auto.components.reviewMap.readingOrder.markFile",
          "Mark {{file}} as read",
          { file: item.file },
        )}
      />
      <span className="w-5 shrink-0 text-right text-muted-foreground">
        {item.n}
      </span>
      <span
        className="w-3 shrink-0 font-mono"
        style={{ color: STATUS_COLOR[item.fileStatus] }}
        aria-label={item.fileStatus}
      >
        {STATUS_LETTER[item.fileStatus]}
      </span>
      <div className="min-w-0 flex-1">
        <div className={`truncate ${seen ? "text-muted-foreground" : ""}`}>
          {basename(item.file)}
          {item.symbols.length > 0 ? (
            <span className="ml-1 text-muted-foreground">
              · {item.symbols.length}
            </span>
          ) : null}
        </div>
        <div className="truncate text-muted-foreground">
          {item.cycleGroup
            ? translate(
                "auto.components.reviewMap.readingOrder.cycle",
                "Dependency cycle",
              )
            : item.dependsOn.length > 0
              ? translate(
                  "auto.components.reviewMap.readingOrder.after",
                  "Read after earlier steps",
                )
              : readingReasonLabel(item.reason)}
          {dir && dir !== "." ? ` · ${dir}` : ""}
        </div>
      </div>
      {overlayFlags && overlayFlags.length > 0 ? (
        <TooltipProvider>
          <span
            className="flex shrink-0 items-center gap-1"
            data-testid="reading-row-flags"
          >
            {overlayFlags.map((flag) => {
              const enc = OVERLAY_ENCODING[flag];
              const Icon = enc.icon;
              const label = translate(enc.labelKey, enc.labelFallback);
              return (
                <Tooltip key={flag}>
                  <TooltipTrigger asChild>
                    <span role="img" aria-label={label} data-flag={flag}>
                      <Icon className={`size-3.5 ${enc.textClass}`} />
                    </span>
                  </TooltipTrigger>
                  <TooltipContent>
                    {translate(enc.descriptionKey, enc.descriptionFallback)}
                  </TooltipContent>
                </Tooltip>
              );
            })}
          </span>
        </TooltipProvider>
      ) : null}
      <button
        type="button"
        tabIndex={-1}
        onClick={(e) => {
          e.stopPropagation();
          onOpenDiff();
        }}
        className="shrink-0 rounded px-1 text-muted-foreground hover:bg-accent"
      >
        {translate(
          "auto.components.reviewMap.readingOrder.viewDiff",
          "View diff",
        )}
      </button>
    </div>
  );
}
