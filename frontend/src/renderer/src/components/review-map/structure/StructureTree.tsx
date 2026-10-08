import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import { ChevronDown, ChevronRight, File, Folder } from "lucide-react";
import { Button } from "@/components/ui/button";
import { isEditableTarget } from "@/lib/editable-target";
import { translate } from "@/i18n/i18n";
import { OVERLAY_ENCODING } from "../review-overlay-model";
import type { ChangedPathIndex } from "./structure-changed-index";
import {
  folderSymbolCount,
  isNodeRow,
  parentPath,
} from "./structure-tree-model";
import type { StructureRow, StructureTreeState } from "./structure-tree-model";

export const STRUCTURE_ROW_PX = 28;
const OVERSCAN = 12;
const treeDomId = (key: string): string => `st-${key.replace(/[^\w-]/g, "_")}`;

type Props = {
  rows: readonly StructureRow[];
  treeState: StructureTreeState;
  changed: ChangedPathIndex;
  selectedPath: string | null;
  onSelectFile: (path: string) => void;
  /** Enter on a folder: drill into it in the treemap. */
  onEnterFolder: (path: string) => void;
  onToggleFolder: (path: string) => void;
  onExpandFolder: (path: string) => void;
  onCollapseFolder: (path: string) => void;
  onLoadMore: (folder: string) => void;
  onRetry: (folder: string) => void;
};

/** WAI-ARIA tree over a windowed list: only the rows in view exist in the DOM. */
export function StructureTree(props: Props): React.JSX.Element {
  const { rows, treeState, changed, selectedPath } = props;
  const scrollRef = useRef<HTMLDivElement>(null);
  const [activeKey, setActiveKey] = useState<string | null>(null);
  const activeIndex = Math.max(
    0,
    rows.findIndex((r) => r.key === activeKey),
  );
  const active = rows[activeIndex];

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => STRUCTURE_ROW_PX,
    overscan: OVERSCAN,
    initialRect: { width: 320, height: 600 },
    getItemKey: (i) => rows[i]?.key ?? i,
  });

  useEffect(() => {
    if (rows.length > 0 && activeKey === null) {
      setActiveKey(rows[0].key);
    }
  }, [rows, activeKey]);

  const moveTo = (i: number): void => {
    const next = rows[Math.min(rows.length - 1, Math.max(0, i))];
    if (next) {
      setActiveKey(next.key);
      virtualizer.scrollToIndex(rows.indexOf(next));
    }
  };

  const onKeyDown = (e: KeyboardEvent): void => {
    if (
      e.defaultPrevented ||
      e.ctrlKey ||
      e.metaKey ||
      e.altKey ||
      isEditableTarget(e.target)
    ) {
      return;
    }
    if (!active) {
      return;
    }
    const folderRow = active.kind === "folder" ? active : null;
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        return moveTo(activeIndex + 1);
      case "ArrowUp":
        e.preventDefault();
        return moveTo(activeIndex - 1);
      case "Home":
        e.preventDefault();
        return moveTo(0);
      case "End":
        e.preventDefault();
        return moveTo(rows.length - 1);
      case "ArrowRight":
        if (folderRow) {
          e.preventDefault();
          if (!folderRow.expanded) {
            props.onExpandFolder(folderRow.path);
          } else {
            moveTo(activeIndex + 1);
          }
        }
        return;
      case "ArrowLeft": {
        e.preventDefault();
        if (folderRow?.expanded) {
          props.onCollapseFolder(folderRow.path);
        } else {
          const parent = parentPath(active.path);
          const pi = rows.findIndex(
            (r) => r.kind === "folder" && r.path === parent,
          );
          if (pi >= 0) {
            moveTo(pi);
          }
        }
        return;
      }
      case "Enter":
        e.preventDefault();
        if (active.kind === "folder") {
          props.onEnterFolder(active.path);
        } else if (active.kind === "file") {
          props.onSelectFile(active.path);
        } else if (active.kind === "more") {
          props.onLoadMore(active.path);
        } else if (active.kind === "error") {
          props.onRetry(active.path);
        }
        break;
      default:
    }
  };

  return (
    <div
      ref={scrollRef}
      role="tree"
      tabIndex={0}
      aria-label={translate(
        "auto.components.reviewMap.structure.treeLabel",
        "Files and folders",
      )}
      aria-activedescendant={active ? treeDomId(active.key) : undefined}
      onKeyDown={onKeyDown}
      className="h-full min-h-0 overflow-auto outline-none focus-visible:ring-1 focus-visible:ring-ring"
    >
      <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
        {virtualizer.getVirtualItems().map((v) => {
          const row = rows[v.index];
          if (!row) {
            return null;
          }
          return (
            <div
              key={row.key}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                height: v.size,
                transform: `translateY(${v.start}px)`,
              }}
            >
              <TreeRow
                row={row}
                domId={treeDomId(row.key)}
                isActive={row.key === active?.key}
                isSelected={row.path === selectedPath}
                treeState={treeState}
                changed={changed}
                props={props}
                onFocusRow={() => setActiveKey(row.key)}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function TreeRow({
  row,
  domId,
  isActive,
  isSelected,
  treeState,
  changed,
  props,
  onFocusRow,
}: {
  row: StructureRow;
  domId: string;
  isActive: boolean;
  isSelected: boolean;
  treeState: StructureTreeState;
  changed: ChangedPathIndex;
  props: Props;
  onFocusRow: () => void;
}): React.JSX.Element {
  const indent = { paddingLeft: 8 + (row.level - 1) * 14 };
  const base = `flex h-7 items-center gap-1 pr-2 text-xs ${isActive ? "bg-accent" : ""} ${
    isSelected ? "font-medium" : ""
  }`;
  if (!isNodeRow(row)) {
    return (
      <div
        id={domId}
        role="treeitem"
        aria-level={row.level}
        aria-selected={false}
        aria-disabled={row.kind === "loading" || undefined}
        style={indent}
        className={base}
        onClick={onFocusRow}
      >
        {row.kind === "loading" ? (
          <span className="text-muted-foreground">
            {translate(
              "auto.components.reviewMap.structure.loading",
              "Loading...",
            )}
          </span>
        ) : row.kind === "error" ? (
          <>
            <span className="text-muted-foreground">
              {translate(
                "auto.components.reviewMap.structure.folderError",
                "Could not load this folder.",
              )}
            </span>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => props.onRetry(row.path)}
            >
              {translate(
                "auto.components.reviewMap.symbolDetail.retry",
                "Retry",
              )}
            </Button>
          </>
        ) : (
          <>
            <span className="tabular-nums text-muted-foreground">
              {row.total
                ? `${row.shown}/${row.total}`
                : translate(
                    "auto.components.reviewMap.structure.shown",
                    "{{count}} shown",
                    {
                      count: row.shown ?? 0,
                    },
                  )}
            </span>
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={() => props.onLoadMore(row.path)}
            >
              {translate(
                "auto.components.reviewMap.structure.loadMore",
                "Load more",
              )}
            </Button>
          </>
        )}
      </div>
    );
  }
  const isFolder = row.kind === "folder";
  const flags = changed.fileFlags.get(row.path);
  const counts = isFolder ? changed.folderCounts.get(row.path) : undefined;
  const sym = isFolder ? folderSymbolCount(treeState, row.node) : null;
  const Chevron = row.expanded ? ChevronDown : ChevronRight;
  const Icon = isFolder ? Folder : File;
  const ChangedIcon = OVERLAY_ENCODING.changed.icon;
  return (
    <div
      id={domId}
      role="treeitem"
      aria-level={row.level}
      aria-setsize={row.setsize}
      aria-posinset={row.posinset}
      aria-expanded={isFolder ? row.expanded : undefined}
      aria-selected={isSelected}
      data-current={isActive || undefined}
      style={indent}
      className={base}
      onClick={() => {
        onFocusRow();
        if (isFolder) {
          props.onToggleFolder(row.path);
        } else {
          props.onSelectFile(row.path);
        }
      }}
      onDoubleClick={() =>
        isFolder ? props.onEnterFolder(row.path) : undefined
      }
    >
      {isFolder ? (
        <Chevron className="size-3.5 shrink-0" aria-hidden />
      ) : (
        <span className="w-3.5 shrink-0" />
      )}
      <Icon className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
      <span className="min-w-0 flex-1 truncate" title={row.path}>
        {row.name}
      </span>
      {flags?.has("changed") || (counts && counts.changed > 0) ? (
        <span
          className="flex shrink-0 items-center gap-0.5 tabular-nums"
          aria-label={translate(
            "auto.components.reviewMap.structure.changedCount",
            "{{count}} changed",
            {
              count: counts?.changed ?? 1,
            },
          )}
        >
          <ChangedIcon
            className={`size-3 ${OVERLAY_ENCODING.changed.textClass}`}
            aria-hidden
          />
          {counts ? counts.changed : null}
        </span>
      ) : null}
      {sym ? (
        <span className="w-12 shrink-0 text-right tabular-nums text-muted-foreground">
          {sym.approximate ? "≥" : ""}
          {sym.value}
        </span>
      ) : row.node.symbolCount > 0 ? (
        <span className="w-12 shrink-0 text-right tabular-nums text-muted-foreground">
          {row.node.symbolCount}
        </span>
      ) : null}
    </div>
  );
}
