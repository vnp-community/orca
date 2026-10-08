import { translate } from "@/i18n/i18n";
import type { SymbolDetail } from "../../../../../shared/code-intel-graph-types";

type Props = {
  flows: SymbolDetail["flows"];
  onOpenFlow: (flow: SymbolDetail["flows"][number]) => void;
};

export function SymbolRelatedFlows({
  flows,
  onOpenFlow,
}: Props): React.JSX.Element {
  if (flows.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {translate(
          "auto.components.reviewMap.symbolDetail.flowsEmpty",
          "No flow found in the index.",
        )}
      </p>
    );
  }
  return (
    <ul>
      {flows.map((f) => (
        <li key={f.id}>
          <button
            type="button"
            onClick={() => onOpenFlow(f)}
            className="flex w-full items-center justify-between gap-2 rounded px-1 py-0.5 text-left text-xs hover:bg-accent"
          >
            <span className="truncate">{f.label}</span>
            <span className="shrink-0 tabular-nums text-muted-foreground">
              {translate(
                "auto.components.reviewMap.symbolDetail.flowStep",
                "step {{step}}/{{total}}",
                {
                  step: f.step,
                  total: f.stepCount,
                },
              )}
            </span>
          </button>
        </li>
      ))}
    </ul>
  );
}
