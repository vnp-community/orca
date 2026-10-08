import { useCallback, useEffect, useState } from "react";
import {
  ArrowLeft,
  Copy,
  Crosshair,
  ExternalLink,
  FileDiff,
} from "lucide-react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { translate } from "@/i18n/i18n";
import { useAppStore } from "@/store";
import { openReviewFileInEditor } from "@/lib/review-diff-navigation";
import type { ReviewDrawerContentProps } from "../review-lens-registry";
import { AmbiguousSymbolDialog } from "../shell/AmbiguousSymbolDialog";
import { ReviewNoteButton } from "../notes/ReviewNoteButton";
import { SymbolCoveringTests } from "./SymbolCoveringTests";
import { SymbolDetailActionsSlot } from "./symbol-detail-actions-slot";
import { SymbolDetailSection } from "./SymbolDetailSection";
import { SymbolRelatedFlows } from "./SymbolRelatedFlows";
import { SymbolRelationList } from "./SymbolRelationList";
import { SymbolSourcePreview } from "./SymbolSourcePreview";
import {
  ambiguousCandidates,
  symbolTargetId,
  useSymbolDetail,
} from "./use-symbol-detail";
import type { SymbolTarget } from "./use-symbol-detail";

type SectionId = "incoming" | "outgoing" | "flows" | "tests" | "source";
const t = translate;

/** Drawer content for the selected symbol: one `symbol` call, the rest loads on demand. */
export function SymbolDetailPanel({
  worktreeId,
  environmentId,
  selectedSymbolKey,
  indexStatus,
  onOpenDiff,
}: ReviewDrawerContentProps): React.JSX.Element {
  // Navigating between neighbours keeps a local stack; selecting another symbol resets it.
  const [stack, setStack] = useState<SymbolTarget[]>([
    { key: selectedSymbolKey },
  ]);
  useEffect(() => setStack([{ key: selectedSymbolKey }]), [selectedSymbolKey]);
  const target = stack.at(-1)!;
  const [open, setOpen] = useState<ReadonlySet<SectionId>>(
    new Set(["incoming", "outgoing"]),
  );
  const [sourceForbidden, setSourceForbidden] = useState(false);
  const onForbidden = useCallback(() => setSourceForbidden(true), []);

  const q = useSymbolDetail(worktreeId, environmentId, target);
  const toggle = (id: SectionId): void =>
    setOpen((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) {
        next.add(id);
      }
      return next;
    });
  const push = (next: SymbolTarget): void => setStack((s) => [...s, next]);

  if (q.status === "error") {
    if (q.error?.kind === "ambiguous") {
      return (
        <AmbiguousSymbolDialog
          candidates={ambiguousCandidates(q.error)}
          onChoose={(c) => setStack((s) => [...s.slice(0, -1), c])}
          onCancel={() => setStack((s) => (s.length > 1 ? s.slice(0, -1) : s))}
        />
      );
    }
    return (
      <div role="alert" className="flex flex-col gap-2 text-sm">
        <p>
          {q.error?.kind === "not-found"
            ? t(
                "auto.components.reviewMap.symbolDetail.notFound",
                "This symbol was not found in the index; the index may be out of date.",
              )
            : t(
                "auto.components.reviewMap.symbolDetail.loadError",
                "Could not load symbol details.",
              )}
        </p>
        <div className="flex gap-2">
          {stack.length > 1 ? (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => setStack((s) => s.slice(0, -1))}
            >
              {t("auto.components.reviewMap.symbolDetail.back", "Back")}
            </Button>
          ) : null}
          <Button type="button" size="sm" variant="outline" onClick={q.refetch}>
            {t("auto.components.reviewMap.symbolDetail.retry", "Retry")}
          </Button>
        </div>
      </div>
    );
  }
  if (q.status !== "success" || !q.data) {
    return (
      <Skeleton className="h-24 w-full" data-testid="symbol-detail-loading" />
    );
  }

  const { symbol, incoming, outgoing, flows } = q.data;
  const lines =
    symbol.startLine !== undefined
      ? `:${symbol.startLine}${symbol.endLine && symbol.endLine !== symbol.startLine ? `-${symbol.endLine}` : ""}`
      : "";
  const countOf = (g: Record<string, unknown[]>): number =>
    Object.values(g).reduce((n, list) => n + list.length, 0);
  const here: SymbolTarget = symbol.key ? { key: symbol.key } : target;
  // Why: with a stale or overlay index the lines belong to the indexed commit, not to HEAD.
  const indexedCommit = indexStatus?.tools.find((tool) => tool.indexedCommit)?.indexedCommit;
  const indexedAtNote =
    (indexStatus?.overall === "STALE" || indexStatus?.overall === "OVERLAY") && indexedCommit
      ? t(
          "auto.components.reviewMap.symbolDetail.indexLines",
          "Line numbers follow the index at {{commit}}.",
          { commit: indexedCommit.slice(0, 7) },
        )
      : null;

  return (
    <div
      className="flex flex-col gap-2"
      data-testid="symbol-detail"
      key={symbolTargetId(target)}
    >
      <header className="flex flex-col gap-1">
        <div className="flex items-center gap-1">
          {stack.length > 1 ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-6"
              aria-label={t(
                "auto.components.reviewMap.symbolDetail.back",
                "Back",
              )}
              onClick={() => setStack((s) => s.slice(0, -1))}
            >
              <ArrowLeft className="size-3.5" aria-hidden />
            </Button>
          ) : null}
          <h3 className="min-w-0 flex-1 truncate text-sm font-medium">
            {symbol.name}
          </h3>
          <Badge variant="secondary">{symbol.kind}</Badge>
        </div>
        {symbol.qualifiedName ? (
          <p className="truncate text-xs text-muted-foreground">
            {symbol.qualifiedName}
          </p>
        ) : null}
        {symbol.signature ? (
          <p className="truncate font-mono text-xs text-muted-foreground">
            {symbol.signature}
          </p>
        ) : null}
        <p className="truncate text-xs text-muted-foreground">
          {symbol.filePath}
          {lines}
        </p>
        {indexedAtNote ? (
          <p className="text-xs text-muted-foreground">{indexedAtNote}</p>
        ) : null}
      </header>

      <div className="flex flex-wrap gap-1">
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => onOpenDiff(symbol.filePath, symbol.startLine)}
        >
          <FileDiff aria-hidden />
          {t("auto.components.reviewMap.symbolDetail.viewDiff", "View diff")}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => {
            const r = openReviewFileInEditor(worktreeId, symbol);
            if (!r.ok) {
              toast.error(
                t(
                  "auto.components.reviewMap.impact.openPathBlocked",
                  "This path is outside the worktree and was not opened.",
                ),
              );
            }
          }}
        >
          <ExternalLink aria-hidden />
          {t(
            "auto.components.reviewMap.symbolDetail.openEditor",
            "Open in editor",
          )}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => {
            const s = useAppStore.getState();
            s.setReviewImpactFocus(worktreeId, symbol.key);
            s.setReviewLens(worktreeId, "impact");
          }}
        >
          <Crosshair aria-hidden />
          {t(
            "auto.components.reviewMap.symbolDetail.setCenter",
            "Set as center",
          )}
        </Button>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={() => void navigator.clipboard?.writeText(symbol.key)}
        >
          <Copy aria-hidden />
          {t("auto.components.reviewMap.symbolDetail.copyKey", "Copy key")}
        </Button>
        <ReviewNoteButton
          worktreeId={worktreeId}
          hotkey
          anchor={{
            kind: "graph-node",
            lens: "impact",
            nodeKey: symbol.key,
            filePath: symbol.filePath,
            ...(symbol.startLine !== undefined
              ? { startLine: symbol.startLine }
              : {}),
            ...(symbol.endLine !== undefined ? { endLine: symbol.endLine } : {}),
            label: symbol.name,
          }}
        />
        <SymbolDetailActionsSlot worktreeId={worktreeId} symbol={symbol} />
      </div>

      <SymbolDetailSection
        title={t(
          "auto.components.reviewMap.symbolDetail.incoming",
          "Called by",
        )}
        count={countOf(incoming)}
        open={open.has("incoming")}
        onToggle={() => toggle("incoming")}
      >
        <SymbolRelationList
          groups={incoming}
          onSelect={(n) =>
            push(n.key ? { key: n.key } : { name: n.name, file: n.filePath })
          }
        />
      </SymbolDetailSection>
      <SymbolDetailSection
        title={t("auto.components.reviewMap.symbolDetail.outgoing", "Calls")}
        count={countOf(outgoing)}
        open={open.has("outgoing")}
        onToggle={() => toggle("outgoing")}
      >
        <SymbolRelationList
          groups={outgoing}
          onSelect={(n) =>
            push(n.key ? { key: n.key } : { name: n.name, file: n.filePath })
          }
        />
      </SymbolDetailSection>
      <SymbolDetailSection
        title={t(
          "auto.components.reviewMap.symbolDetail.flows",
          "Related flows",
        )}
        count={flows.length}
        open={open.has("flows")}
        onToggle={() => toggle("flows")}
      >
        <SymbolRelatedFlows
          flows={flows}
          onOpenFlow={(flow) => {
            const s = useAppStore.getState();
            s.setReviewDataFlowId(worktreeId, flow.id);
            s.setReviewLens(worktreeId, "dataflow");
          }}
        />
      </SymbolDetailSection>
      <SymbolDetailSection
        title={t(
          "auto.components.reviewMap.symbolDetail.tests",
          "Covering tests",
        )}
        open={open.has("tests")}
        onToggle={() => toggle("tests")}
      >
        <SymbolCoveringTests
          worktreeId={worktreeId}
          environmentId={environmentId}
          target={here}
          onSelectTest={(test) =>
            push(test.file ? { name: test.name, file: test.file } : here)
          }
        />
      </SymbolDetailSection>
      {sourceForbidden ? null : (
        <SymbolDetailSection
          title={t("auto.components.reviewMap.symbolDetail.source", "Source")}
          open={open.has("source")}
          onToggle={() => toggle("source")}
        >
          <SymbolSourcePreview
            worktreeId={worktreeId}
            environmentId={environmentId}
            target={here}
            onForbidden={onForbidden}
          />
        </SymbolDetailSection>
      )}
    </div>
  );
}
