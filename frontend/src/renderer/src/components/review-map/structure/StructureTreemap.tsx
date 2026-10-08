import { useEffect, useMemo, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { translate } from "@/i18n/i18n";
import { overlaySvgProps } from "../review-overlay-model";
import {
  layoutSquarifiedTreemap,
  SMALL_CELL_ID,
} from "./structure-treemap-layout";
import type { TreemapCellMeta } from "./structure-treemap-model";

export const TREEMAP_LABEL_MIN_W = 120;
export const TREEMAP_LABEL_MIN_H = 80;

type Props = {
  cells: readonly TreemapCellMeta[];
  /** Fixed size for tests / SSR; otherwise measured with ResizeObserver. */
  size?: { width: number; height: number };
  onEnterFolder: (path: string) => void;
  onSelectFile: (path: string) => void;
  onOpenTree: () => void;
  onUp: () => void;
};

function fillFor(tokenVar: string | null): string {
  return tokenVar
    ? `color-mix(in srgb, var(${tokenVar}) 28%, var(--card))`
    : "color-mix(in srgb, var(--muted-foreground) 14%, var(--card))";
}

/** The treemap is a picture (role=img); the tree beside it is the keyboard/screen-reader path. */
export function StructureTreemap({
  cells,
  size,
  onEnterFolder,
  onSelectFile,
  onOpenTree,
  onUp,
}: Props): React.JSX.Element {
  const ref = useRef<HTMLDivElement>(null);
  const [measured, setMeasured] = useState({ width: 0, height: 0 });

  useEffect(() => {
    const el = ref.current;
    if (size || !el || typeof ResizeObserver === "undefined") {
      return;
    }
    let raf = 0;
    const ro = new ResizeObserver((entries) => {
      const r = entries[0]?.contentRect;
      if (!r) {
        return;
      }
      // One layout per frame while the panel is being dragged.
      cancelAnimationFrame(raf);
      raf = requestAnimationFrame(() =>
        setMeasured({ width: r.width, height: r.height }),
      );
    });
    ro.observe(el);
    return () => {
      cancelAnimationFrame(raf);
      ro.disconnect();
    };
  }, [size]);

  const { width, height } = size ?? measured;
  const byId = useMemo(() => new Map(cells.map((c) => [c.id, c])), [cells]);
  const layout = useMemo(
    () =>
      layoutSquarifiedTreemap(
        cells.map((c) => ({ id: c.id, value: c.value })),
        { x: 0, y: 0, w: width, h: height },
        { padding: 2 },
      ),
    [cells, width, height],
  );
  const drawn = cells.length - layout.mergedIds.length;
  const label = translate(
    "auto.components.reviewMap.structure.treemapLabel",
    "Treemap of {{count}} items; use the tree for keyboard access.",
    { count: cells.length },
  );

  const onKeyDown = (e: KeyboardEvent): void => {
    if (e.key === "Escape") {
      e.preventDefault();
      onUp();
    }
  };

  return (
    <div
      ref={ref}
      className="relative h-full min-h-0 w-full"
      onKeyDown={onKeyDown}
      tabIndex={-1}
    >
      {width > 0 && height > 0 ? (
        <svg
          role="img"
          aria-label={label}
          width={width}
          height={height}
          data-testid="treemap-svg"
        >
          {layout.cells.map((cell) => {
            if (cell.id === SMALL_CELL_ID) {
              return (
                <g
                  key={cell.id}
                  data-cell={SMALL_CELL_ID}
                  onClick={onOpenTree}
                  className="cursor-pointer"
                >
                  <rect
                    x={cell.x}
                    y={cell.y}
                    width={cell.w}
                    height={cell.h}
                    style={{ fill: fillFor(null) }}
                    stroke="var(--border)"
                  />
                  <text
                    x={cell.x + 6}
                    y={cell.y + 16}
                    className="fill-foreground text-[11px]"
                  >
                    {translate(
                      "auto.components.reviewMap.structure.small",
                      "+{{count}} small",
                      {
                        count: layout.mergedIds.length,
                      },
                    )}
                  </text>
                </g>
              );
            }
            const meta = byId.get(cell.id);
            if (!meta) {
              return null;
            }
            const mark = overlaySvgProps(meta.flags);
            const showLabel =
              cell.w >= TREEMAP_LABEL_MIN_W && cell.h >= TREEMAP_LABEL_MIN_H;
            return (
              <g
                key={cell.id}
                data-cell={cell.id}
                opacity={meta.dimmed ? 0.35 : 1}
                className="cursor-pointer"
                onClick={() =>
                  meta.kind === "folder"
                    ? onEnterFolder(meta.id)
                    : onSelectFile(meta.id)
                }
              >
                <title>
                  {`${meta.id}\n${translate("auto.components.reviewMap.structure.tipSymbols", "{{count}} symbols", { count: meta.symbolCount })}${
                    meta.loc !== undefined ? ` · ${meta.loc} LOC` : ""
                  }${meta.group ? `\n${meta.group}` : ""}${
                    meta.changedCount > 0
                      ? `\n${translate("auto.components.reviewMap.structure.changedCount", "{{count}} changed", { count: meta.changedCount })}`
                      : ""
                  }`}
                </title>
                <rect
                  x={cell.x}
                  y={cell.y}
                  width={cell.w}
                  height={cell.h}
                  style={{ fill: fillFor(meta.tokenVar) }}
                  stroke={mark?.stroke ?? "var(--border)"}
                  strokeWidth={mark?.strokeWidth ?? 1}
                  strokeDasharray={mark?.strokeDasharray}
                />
                {meta.flags.has("changed") ? (
                  <g data-mark="changed">
                    <circle
                      cx={cell.x + 8}
                      cy={cell.y + 8}
                      r={4}
                      style={{ fill: "var(--review-changed)" }}
                    />
                    {meta.kind === "folder" &&
                    meta.changedCount > 0 &&
                    cell.w >= 40 ? (
                      <text
                        x={cell.x + 16}
                        y={cell.y + 12}
                        className="fill-foreground text-[10px]"
                      >
                        {meta.changedCount}
                      </text>
                    ) : null}
                  </g>
                ) : null}
                {showLabel ? (
                  <>
                    <text
                      x={cell.x + 8}
                      y={cell.y + 28}
                      className="fill-foreground text-xs font-medium"
                    >
                      {meta.name}
                    </text>
                    {meta.group ? (
                      <text
                        x={cell.x + 8}
                        y={cell.y + 44}
                        className="fill-muted-foreground text-[11px]"
                      >
                        {meta.group}
                      </text>
                    ) : null}
                  </>
                ) : null}
              </g>
            );
          })}
        </svg>
      ) : null}
      <span className="sr-only" data-testid="treemap-count">
        {drawn}
      </span>
    </div>
  );
}
