import { useEffect, useMemo } from "react";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { translate } from "@/i18n/i18n";
import { maskSensitiveText } from "../sensitive-text-masking";
import { useSymbolSource } from "./use-symbol-detail";
import type { SymbolTarget } from "./use-symbol-detail";

export const SYMBOL_SOURCE_MAX_LINES = 200;

type Props = {
  worktreeId: string;
  environmentId: string | null;
  target: SymbolTarget;
  /** Told when the backend refuses source (missing read_source) so the section can hide. */
  onForbidden: () => void;
};

const OMITTED_LABEL: Record<string, [string, string]> = {
  gitignored: ["gitignored", "Source omitted: the file is git-ignored."],
  binary: ["binary", "Source omitted: binary file."],
  sensitive_path: ["sensitive", "Source omitted: the path may hold secrets."],
  not_requested: ["notRequested", "Source was not requested."],
};

/** Plain text on purpose (backend strings are never decoded or highlighted); not logged. */
export function SymbolSourcePreview({
  worktreeId,
  environmentId,
  target,
  onForbidden,
}: Props): React.JSX.Element {
  const { state, retry } = useSymbolSource(
    worktreeId,
    environmentId,
    target,
    true,
  );
  const view = useMemo(() => {
    if (state.status !== "ready" || !state.source) {
      return null;
    }
    const masked = maskSensitiveText(state.source.text);
    const lines = masked.text.split("\n");
    return {
      lines: lines.slice(0, SYMBOL_SOURCE_MAX_LINES),
      cut: lines.length > SYMBOL_SOURCE_MAX_LINES || state.source.truncated,
      start: state.source.startLine,
      masked: masked.masked,
    };
  }, [state]);

  const forbidden =
    state.status === "error" && state.error.kind === "forbidden";
  useEffect(() => {
    if (forbidden) {
      onForbidden();
    }
  }, [forbidden, onForbidden]);

  if (state.status === "error") {
    if (forbidden) {
      return <span hidden />;
    }
    return (
      <div
        className="flex items-center gap-2 text-xs text-muted-foreground"
        role="alert"
      >
        <span>
          {translate(
            "auto.components.reviewMap.symbolDetail.sourceError",
            "Could not load source.",
          )}
        </span>
        <Button type="button" size="sm" variant="ghost" onClick={retry}>
          {translate("auto.components.reviewMap.symbolDetail.retry", "Retry")}
        </Button>
      </div>
    );
  }
  if (state.status !== "ready") {
    return <Skeleton className="h-16 w-full" />;
  }
  if (!view) {
    const [id, fallback] = OMITTED_LABEL[state.omitted ?? ""] ?? [
      "unavailable",
      "No source available.",
    ];
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          `auto.components.reviewMap.symbolDetail.omitted.${id}`,
          fallback,
        )}
      </p>
    );
  }
  return (
    <div>
      <pre className="max-h-64 overflow-auto rounded border bg-muted/40 p-2 font-mono text-xs">
        <code>
          {view.lines.map((line, i) => (
            <div key={i} className="flex gap-2">
              <span className="w-8 shrink-0 select-none text-right text-muted-foreground tabular-nums">
                {view.start + i}
              </span>
              <span className="whitespace-pre">{line}</span>
            </div>
          ))}
        </code>
      </pre>
      {view.cut ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {translate(
            "auto.components.reviewMap.symbolDetail.sourceCut",
            "Showing the first {{count}} lines.",
            {
              count: SYMBOL_SOURCE_MAX_LINES,
            },
          )}
        </p>
      ) : null}
      {view.masked ? (
        <p className="mt-1 text-xs text-muted-foreground">
          {translate(
            "auto.components.reviewMap.symbolDetail.sourceMasked",
            "Possible secrets were hidden.",
          )}
        </p>
      ) : null}
    </div>
  );
}
