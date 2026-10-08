import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { translate } from "@/i18n/i18n";
import { useCoveringTests } from "./use-symbol-detail";
import type { SymbolTarget } from "./use-symbol-detail";

type Props = {
  worktreeId: string;
  environmentId: string | null;
  target: SymbolTarget;
  onSelectTest: (test: { name: string; file: string }) => void;
};

/** Mounted only while its section is open, so the `impact includeTests` call is lazy. */
export function SymbolCoveringTests({
  worktreeId,
  environmentId,
  target,
  onSelectTest,
}: Props): React.JSX.Element {
  const q = useCoveringTests(worktreeId, environmentId, target, true);
  if (q.status === "error") {
    return (
      <div
        className="flex items-center gap-2 text-xs text-muted-foreground"
        role="alert"
      >
        <span>
          {translate(
            "auto.components.reviewMap.symbolDetail.testsError",
            "Could not load tests.",
          )}
        </span>
        <Button type="button" size="sm" variant="ghost" onClick={q.refetch}>
          {translate("auto.components.reviewMap.symbolDetail.retry", "Retry")}
        </Button>
      </div>
    );
  }
  if (q.status !== "success" || !q.data) {
    return <Skeleton className="h-8 w-full" />;
  }
  const tests = q.data.testsCovering ?? [];
  if (tests.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          "auto.components.reviewMap.symbolDetail.testsEmpty",
          "No covering test found in the index.",
        )}
      </p>
    );
  }
  return (
    <ul>
      {tests.map((t) => (
        <li key={t.key}>
          <button
            type="button"
            onClick={() => onSelectTest({ name: t.name, file: t.filePath })}
            className="flex w-full min-w-0 flex-col rounded px-1 py-0.5 text-left text-xs hover:bg-accent"
          >
            <span className="truncate">{t.name}</span>
            <span className="truncate text-muted-foreground">{t.filePath}</span>
          </button>
        </li>
      ))}
    </ul>
  );
}
