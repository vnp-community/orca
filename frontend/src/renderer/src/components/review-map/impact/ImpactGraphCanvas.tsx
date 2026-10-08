import { useMemo } from "react";
import { Background, Controls, MiniMap, ReactFlow } from "@xyflow/react";
import type { Edge, Node } from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useDocumentColorMode } from "@/hooks/useDocumentColorMode";
import { usePrefersReducedMotion } from "@/hooks/usePrefersReducedMotion";
import { computeOverlayFlags } from "../review-overlay-model";
import type { OverlayImpactInput } from "../review-overlay-model";
import type { ChangeOverlayView } from "../review-wire-types";
import type { ImpactLayout } from "./impact-column-layout";
import { ImpactSymbolNode } from "./ImpactSymbolNode";
import type { ImpactFlowNodeData } from "./ImpactSymbolNode";
import type { ImpactNodeActions } from "./ImpactNodeCard";

export const IMPACT_MINIMAP_AFTER = 80;
const NODE_TYPES = { impact: ImpactSymbolNode };

type Props = {
  layout: ImpactLayout;
  overlay: ChangeOverlayView;
  impact: OverlayImpactInput;
  selectedKey: string | null;
  actions: ImpactNodeActions;
  onExpandColumn: (column: number) => void;
};

/** Lazy-loaded (default export) so xyflow stays out of the Review tab's first chunk. */
export default function ImpactGraphCanvas({
  layout,
  overlay,
  impact,
  selectedKey,
  actions,
  onExpandColumn,
}: Props): React.JSX.Element {
  const colorMode = useDocumentColorMode();
  const reduceMotion = usePrefersReducedMotion();
  const { nodes, edges } = useMemo(() => {
    const flowNodes: Node[] = layout.nodes.map((n) => {
      const symbol = n.type === "more" ? undefined : n.symbol;
      const data: ImpactFlowNodeData = {
        kind: n.type,
        symbol,
        flags: symbol
          ? computeOverlayFlags(
              { symbolKey: symbol.key, file: symbol.filePath },
              overlay,
              impact,
            )
          : new Set(),
        selected: symbol ? selectedKey === symbol.key : false,
        direct: n.type === "symbol" ? n.direct : undefined,
        via: n.type === "symbol" ? n.via : undefined,
        hiddenCount: n.type === "more" ? n.hiddenCount : undefined,
        column: n.column,
        actions,
        onExpand: onExpandColumn,
      };
      return {
        id: n.id,
        type: "impact",
        position: { x: n.x, y: n.y },
        data,
        draggable: false,
      };
    });
    const flowEdges: Edge[] = layout.edges.map((e) => ({
      id: e.id,
      source: e.source,
      target: e.target,
      style: { stroke: "var(--muted-foreground)" },
    }));
    return { nodes: flowNodes, edges: flowEdges };
  }, [layout, overlay, impact, selectedKey, actions, onExpandColumn]);

  return (
    <div className="h-full min-h-0 w-full" data-testid="impact-canvas">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        colorMode={colorMode}
        fitView
        fitViewOptions={{ padding: 0.2, duration: reduceMotion ? 0 : 200 }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        onlyRenderVisibleElements
        proOptions={{ hideAttribution: true }}
      >
        <Background gap={16} />
        <Controls showInteractive={false} />
        {nodes.length > IMPACT_MINIMAP_AFTER ? (
          <MiniMap nodeColor="var(--muted-foreground)" />
        ) : null}
      </ReactFlow>
    </div>
  );
}
