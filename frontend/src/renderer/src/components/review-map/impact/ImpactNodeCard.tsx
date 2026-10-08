import { Crosshair, ExternalLink, FileDiff, Copy } from "lucide-react";
import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuItem,
  ContextMenuTrigger,
} from "@/components/ui/context-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { translate } from "@/i18n/i18n";
import {
  OVERLAY_ENCODING,
  overlayClassNames,
  overlayIconFlags,
} from "../review-overlay-model";
import type { OverlayFlagSet } from "../review-overlay-model";
import type { SymbolRefView } from "../review-wire-types";

export type ImpactNodeActions = {
  onSelect: (symbolKey: string) => void;
  onOpenDiff: (path: string, line?: number) => void;
  onSetCenter: (symbolKey: string) => void;
  onOpenEditor: (symbol: SymbolRefView) => void;
};

type Props = {
  symbol: SymbolRefView;
  flags: OverlayFlagSet;
  selected: boolean;
  isCenter?: boolean;
  direct?: boolean;
  via?: string;
  actions: ImpactNodeActions;
};

/** One symbol box, shared by the graph canvas and the list view. */
export function ImpactNodeCard({
  symbol,
  flags,
  selected,
  isCenter,
  direct,
  via,
  actions,
}: Props): React.JSX.Element {
  const line = symbol.startLine ? `:${symbol.startLine}` : "";
  return (
    <ContextMenu>
      <ContextMenuTrigger asChild>
        <div
          role="button"
          tabIndex={0}
          aria-pressed={selected}
          data-testid="impact-node"
          data-symbol-key={symbol.key}
          data-center={isCenter ? "true" : undefined}
          title={
            via
              ? `${symbol.filePath}${line} · ${via}`
              : `${symbol.filePath}${line}`
          }
          onClick={() => actions.onSelect(symbol.key)}
          onDoubleClick={() =>
            actions.onOpenDiff(symbol.filePath, symbol.startLine)
          }
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              actions.onOpenDiff(symbol.filePath, symbol.startLine);
            } else if (e.key === " ") {
              e.preventDefault();
              actions.onSelect(symbol.key);
            }
          }}
          className={`flex w-full min-w-0 flex-col rounded-md bg-card px-2 py-1 text-left text-xs ${
            overlayClassNames(flags) || "border"
          } ${selected ? "ring-2 ring-ring" : ""} ${isCenter ? "font-medium" : ""}`}
        >
          <span className="flex min-w-0 items-center gap-1">
            <span className="truncate">{symbol.name}</span>
            <TooltipProvider>
              {overlayIconFlags(flags).map((flag) => {
                const enc = OVERLAY_ENCODING[flag];
                const Icon = enc.icon;
                const label = translate(enc.labelKey, enc.labelFallback);
                return (
                  <Tooltip key={flag}>
                    <TooltipTrigger asChild>
                      <span
                        role="img"
                        aria-label={label}
                        data-flag={flag}
                        className="shrink-0"
                      >
                        <Icon className={`size-3 ${enc.textClass}`} />
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>{label}</TooltipContent>
                  </Tooltip>
                );
              })}
            </TooltipProvider>
            {direct ? (
              <span className="ml-auto shrink-0 text-muted-foreground">
                {translate("auto.components.reviewMap.impact.direct", "direct")}
              </span>
            ) : null}
          </span>
          <span className="truncate text-muted-foreground">
            {symbol.kind} · {symbol.filePath}
            {line}
          </span>
        </div>
      </ContextMenuTrigger>
      <ContextMenuContent>
        <ContextMenuItem
          onSelect={() => actions.onOpenDiff(symbol.filePath, symbol.startLine)}
        >
          <FileDiff aria-hidden />
          {translate(
            "auto.components.reviewMap.symbolDetail.viewDiff",
            "View diff",
          )}
        </ContextMenuItem>
        <ContextMenuItem onSelect={() => actions.onSetCenter(symbol.key)}>
          <Crosshair aria-hidden />
          {translate(
            "auto.components.reviewMap.symbolDetail.setCenter",
            "Set as center",
          )}
        </ContextMenuItem>
        <ContextMenuItem onSelect={() => actions.onOpenEditor(symbol)}>
          <ExternalLink aria-hidden />
          {translate(
            "auto.components.reviewMap.symbolDetail.openEditor",
            "Open in editor",
          )}
        </ContextMenuItem>
        <ContextMenuItem
          onSelect={() => void navigator.clipboard?.writeText(symbol.key)}
        >
          <Copy aria-hidden />
          {translate(
            "auto.components.reviewMap.symbolDetail.copyKey",
            "Copy key",
          )}
        </ContextMenuItem>
      </ContextMenuContent>
    </ContextMenu>
  );
}
