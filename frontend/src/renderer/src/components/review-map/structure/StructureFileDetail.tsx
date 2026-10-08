import { Copy, ExternalLink, FileDiff, X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { translate } from "@/i18n/i18n";
import { OVERLAY_ENCODING, overlayIconFlags } from "../review-overlay-model";
import type { OverlayFlagSet } from "../review-overlay-model";
import type { SymbolRefView } from "../review-wire-types";

type Props = {
  path: string;
  language?: string;
  symbolCount: number;
  loc?: number;
  flags: OverlayFlagSet;
  changedSymbols: readonly SymbolRefView[];
  onClose: () => void;
  onOpenDiff: (path: string) => void;
  onOpenEditor: (path: string) => void;
  /** Opens the Impact lens centred on the symbol. */
  onOpenSymbol: (symbolKey: string) => void;
};

/** Facts about one file from the index; never loads source (that stays in the symbol panel). */
export function StructureFileDetail(p: Props): React.JSX.Element {
  const t = translate;
  return (
    <section
      aria-label={t(
        "auto.components.reviewMap.structure.fileDetail",
        "File details",
      )}
      className="flex flex-col gap-2 border-t p-3 text-xs"
      data-testid="structure-file-detail"
    >
      <div className="flex items-center gap-2">
        <span className="min-w-0 flex-1 truncate font-medium" title={p.path}>
          {p.path}
        </span>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t(
            "auto.components.reviewMap.symbolDetail.copyPath",
            "Copy path",
          )}
          onClick={() => void navigator.clipboard?.writeText(p.path)}
        >
          <Copy className="size-3.5" aria-hidden />
        </Button>
        <Button
          type="button"
          size="icon"
          variant="ghost"
          className="size-6"
          aria-label={t(
            "auto.components.reviewMap.structure.closeDetail",
            "Close file details",
          )}
          onClick={p.onClose}
        >
          <X className="size-3.5" aria-hidden />
        </Button>
      </div>
      <div className="flex flex-wrap items-center gap-2 text-muted-foreground">
        {p.language ? <Badge variant="secondary">{p.language}</Badge> : null}
        <span>
          {t(
            "auto.components.reviewMap.structure.tipSymbols",
            "{{count}} symbols",
            { count: p.symbolCount },
          )}
        </span>
        {p.loc !== undefined ? <span>{p.loc} LOC</span> : null}
        {overlayIconFlags(p.flags).map((flag) => {
          const enc = OVERLAY_ENCODING[flag];
          const Icon = enc.icon;
          return (
            <span key={flag} className="flex items-center gap-1">
              <Icon className={`size-3 ${enc.textClass}`} aria-hidden />
              {t(enc.labelKey, enc.labelFallback)}
            </span>
          );
        })}
      </div>
      {p.changedSymbols.length > 0 ? (
        <div>
          <div className="mb-1 text-muted-foreground">
            {t(
              "auto.components.reviewMap.structure.changedSymbols",
              "Changed symbols in this file",
            )}
          </div>
          <ul>
            {p.changedSymbols.map((s) => (
              <li key={s.key}>
                <button
                  type="button"
                  className="w-full truncate rounded px-1 py-0.5 text-left hover:bg-accent"
                  onClick={() => p.onOpenSymbol(s.key)}
                >
                  {s.name}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      <div className="flex gap-1">
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => p.onOpenDiff(p.path)}
        >
          <FileDiff aria-hidden />
          {t("auto.components.reviewMap.symbolDetail.viewDiff", "View diff")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => p.onOpenEditor(p.path)}
        >
          <ExternalLink aria-hidden />
          {t(
            "auto.components.reviewMap.symbolDetail.openEditor",
            "Open in editor",
          )}
        </Button>
      </div>
    </section>
  );
}
