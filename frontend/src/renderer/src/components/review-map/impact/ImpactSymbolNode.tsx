import { Handle, Position } from "@xyflow/react";
import type { NodeProps } from "@xyflow/react";
import { Button } from "@/components/ui/button";
import { translate } from "@/i18n/i18n";
import { ImpactNodeCard } from "./ImpactNodeCard";
import type { ImpactNodeActions } from "./ImpactNodeCard";
import type { OverlayFlagSet } from "../review-overlay-model";
import type { SymbolRefView } from "../review-wire-types";

export type ImpactFlowNodeData = {
  kind: "center" | "symbol" | "more";
  symbol?: SymbolRefView;
  flags: OverlayFlagSet;
  selected: boolean;
  direct?: boolean;
  via?: string;
  hiddenCount?: number;
  column: number;
  actions: ImpactNodeActions;
  onExpand: (column: number) => void;
  [key: string]: unknown;
};

export function ImpactSymbolNode({ data }: NodeProps): React.JSX.Element {
  const d = data as ImpactFlowNodeData;
  return (
    <div style={{ width: 260 }}>
      <Handle type="target" position={Position.Left} isConnectable={false} />
      {d.kind === "more" ? (
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => d.onExpand(d.column)}
        >
          {translate(
            "auto.components.reviewMap.impact.more",
            "+{{count}} more",
            {
              count: d.hiddenCount ?? 0,
            },
          )}
        </Button>
      ) : (
        <ImpactNodeCard
          symbol={d.symbol!}
          flags={d.flags}
          selected={d.selected}
          isCenter={d.kind === "center"}
          direct={d.direct}
          via={d.via}
          actions={d.actions}
        />
      )}
      <Handle type="source" position={Position.Right} isConnectable={false} />
    </div>
  );
}
