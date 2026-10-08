import { useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@/components/ui/resizable";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { usePerceivedLoadingStage } from "@/hooks/usePerceivedLoadingStage";
import { translate } from "@/i18n/i18n";
import { useAppStore } from "@/store";
import { openReviewFileInEditor } from "@/lib/review-diff-navigation";
import { reviewChipPredicate } from "../review-chip-filter";
import type { ReviewLensProps } from "../review-lens-registry";
import { overlayFlagsWithData } from "../review-overlay-model";
import { ReviewLoadingStage } from "../shell/ReviewLoadingStage";
import { buildChangedPathIndex } from "./structure-changed-index";
import { normalizeStructurePath } from "./structure-area-model";
import { buildTreemapCells, hasLocData } from "./structure-treemap-model";
import type { TreemapColorBy, TreemapSizeBy } from "./structure-treemap-model";
import {
  childrenOf,
  flattenStructureRows,
  nearestLoadedParent,
  ROOT_PATH,
  STRUCTURE_MAX_NODES,
} from "./structure-tree-model";
import { StructureFileDetail } from "./StructureFileDetail";
import { StructureToolbar } from "./StructureToolbar";
import { StructureTree } from "./StructureTree";
import { StructureTreemap } from "./StructureTreemap";
import { useStructureTree } from "./use-structure-tree";

export const STRUCTURE_NARROW_PX = 560;
const t = translate;

/** Structure lens: treemap of the current folder plus a virtualized tree, overlay on top. */
export default function StructureLens(
  props: ReviewLensProps,
): React.JSX.Element {
  const {
    worktreeId,
    environmentId,
    overlay,
    chipFilter,
    onSelectSymbol,
    onOpenDiff,
  } = props;
  const tree = useStructureTree({ worktreeId, environmentId });
  const [currentPath, setCurrentPath] = useState(ROOT_PATH);
  const [sizeBy, setSizeBy] = useState<TreemapSizeBy>("symbols");
  const [colorBy, setColorBy] = useState<TreemapColorBy>("area");
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [pane, setPane] = useState<"treemap" | "tree">("treemap");
  const [narrow, setNarrow] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const el = rootRef.current;
    if (!el || typeof ResizeObserver === "undefined") {
      return;
    }
    const ro = new ResizeObserver((e) =>
      setNarrow((e[0]?.contentRect.width ?? 1000) < STRUCTURE_NARROW_PX),
    );
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const changed = useMemo(
    () =>
      buildChangedPathIndex(
        overlay.changedFiles,
        overlay.uncoveredSymbols.map((s) => s.filePath),
        overlay.violations.map((v) => v.file),
      ),
    [overlay],
  );
  const keepFiles = useMemo(() => {
    if (!chipFilter) {
      return null;
    }
    const pred = reviewChipPredicate(overlay, chipFilter);
    return pred ? new Set([...pred.files].map(normalizeStructurePath)) : null;
  }, [chipFilter, overlay]);

  const rows = useMemo(
    () => flattenStructureRows(tree.state, tree.expanded, keepFiles),
    [tree.state, tree.expanded, keepFiles],
  );
  const children = useMemo(
    () => childrenOf(tree.state, currentPath),
    [tree.state, currentPath],
  );
  const { cells, groups } = useMemo(
    () =>
      buildTreemapCells({
        nodes: children,
        state: tree.state,
        changed,
        changedFiles: overlay.changedFiles as { path: string; area?: string }[],
        sizeBy,
        colorBy,
        keepFiles,
      }),
    [
      children,
      tree.state,
      changed,
      overlay.changedFiles,
      sizeBy,
      colorBy,
      keepFiles,
    ],
  );
  const locAvailable = hasLocData(children);
  useEffect(() => {
    if (sizeBy === "loc" && !locAvailable) {
      setSizeBy("symbols");
    }
  }, [sizeBy, locAvailable]);

  const rootLoad = tree.state.folders.get(ROOT_PATH);
  const stage = usePerceivedLoadingStage(
    tree.rootStatus === "loading" || tree.rootStatus === "idle",
    {
      remote: environmentId !== null,
    },
  );

  const enterFolder = (path: string): void => {
    setCurrentPath(path);
    tree.expandFolder(path);
  };
  const openEditor = (path: string): void => {
    if (!openReviewFileInEditor(worktreeId, { filePath: path }).ok) {
      toast.error(
        t(
          "auto.components.reviewMap.impact.openPathBlocked",
          "This path is outside the worktree and was not opened.",
        ),
      );
    }
  };
  const selected = selectedFile
    ? tree.state.nodes.get(selectedFile)
    : undefined;

  if (tree.rootStatus === "idle" || tree.rootStatus === "loading") {
    return (
      <div ref={rootRef} className="h-full">
        <ReviewLoadingStage stage={stage} />
      </div>
    );
  }
  if (tree.rootStatus === "error") {
    return (
      <div
        ref={rootRef}
        role="alert"
        className="flex flex-col items-start gap-2 p-6 text-sm"
      >
        <p>
          {t(
            "auto.components.reviewMap.structure.rootError",
            "Could not load the repository structure.",
          )}
        </p>
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={() => tree.retry(ROOT_PATH)}
        >
          {t("auto.components.reviewMap.symbolDetail.retry", "Retry")}
        </Button>
      </div>
    );
  }

  const treePane = (
    <StructureTree
      rows={rows}
      treeState={tree.state}
      changed={changed}
      selectedPath={selectedFile}
      onSelectFile={setSelectedFile}
      onEnterFolder={enterFolder}
      onToggleFolder={tree.toggleFolder}
      onExpandFolder={tree.expandFolder}
      onCollapseFolder={tree.collapseFolder}
      onLoadMore={tree.loadMore}
      onRetry={tree.retry}
    />
  );
  const treemapPane =
    cells.length === 0 ? (
      <div
        className="flex flex-col items-start gap-2 p-6 text-sm text-muted-foreground"
        data-testid="structure-empty"
      >
        <p>
          {t(
            "auto.components.reviewMap.structure.empty",
            "The index has no structure information for this path.",
          )}
        </p>
        {currentPath ? (
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={() =>
              setCurrentPath(nearestLoadedParent(tree.state, currentPath))
            }
          >
            {t("auto.components.reviewMap.structure.up", "Up one level")}
          </Button>
        ) : null}
      </div>
    ) : (
      <StructureTreemap
        cells={cells}
        onEnterFolder={enterFolder}
        onSelectFile={setSelectedFile}
        onOpenTree={() => setPane("tree")}
        onUp={() =>
          currentPath &&
          setCurrentPath(nearestLoadedParent(tree.state, currentPath))
        }
      />
    );

  return (
    <div
      ref={rootRef}
      className="flex h-full min-h-0 flex-col"
      data-testid="structure-lens"
    >
      <StructureToolbar
        currentPath={currentPath}
        sizeBy={sizeBy}
        colorBy={colorBy}
        locAvailable={locAvailable}
        legendFlags={overlayFlagsWithData(overlay, { hasImpact: false })}
        groups={groups}
        onSizeBy={setSizeBy}
        onColorBy={setColorBy}
        onNavigate={(p) =>
          p === ROOT_PATH ? setCurrentPath(ROOT_PATH) : enterFolder(p)
        }
      />
      <div className="flex flex-col gap-0.5 px-3 py-1 text-xs text-muted-foreground">
        {changed.deletedFiles.length > 0 ? (
          <p>
            {t(
              "auto.components.reviewMap.structure.deleted",
              "{{count}} deleted files are not shown",
              { count: changed.deletedFiles.length },
            )}
          </p>
        ) : null}
        {tree.capReached ? (
          <p role="status">
            {t(
              "auto.components.reviewMap.structure.capReached",
              "Stopped loading at {{count}} nodes. Open a folder to see more.",
              { count: STRUCTURE_MAX_NODES },
            )}
          </p>
        ) : null}
        {rootLoad?.totalCount && rootLoad.nextPageToken ? (
          <p>{`${rootLoad.loaded}/${rootLoad.totalCount}`}</p>
        ) : null}
      </div>
      {narrow ? (
        <>
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            value={pane}
            onValueChange={(v) => v && setPane(v as "treemap" | "tree")}
            className="mx-3 mb-1 w-fit"
          >
            <ToggleGroupItem value="treemap">
              {t("auto.components.reviewMap.structure.paneTreemap", "Treemap")}
            </ToggleGroupItem>
            <ToggleGroupItem value="tree">
              {t("auto.components.reviewMap.structure.paneTree", "Tree")}
            </ToggleGroupItem>
          </ToggleGroup>
          <div className="min-h-0 flex-1">
            {pane === "treemap" ? treemapPane : treePane}
          </div>
        </>
      ) : (
        <ResizablePanelGroup orientation="vertical" className="min-h-0 flex-1">
          <ResizablePanel id="treemap" minSize="25">
            {treemapPane}
          </ResizablePanel>
          <ResizableHandle />
          <ResizablePanel id="tree" minSize="20">
            {treePane}
          </ResizablePanel>
        </ResizablePanelGroup>
      )}
      {selected && selectedFile ? (
        <StructureFileDetail
          path={selectedFile}
          language={selected.language}
          symbolCount={selected.symbolCount}
          loc={selected.loc}
          flags={changed.fileFlags.get(selectedFile) ?? new Set()}
          changedSymbols={overlay.changedSymbols
            .filter(
              (c) => normalizeStructurePath(c.symbol.filePath) === selectedFile,
            )
            .map((c) => c.symbol)}
          onClose={() => setSelectedFile(null)}
          onOpenDiff={(p) => onOpenDiff(p)}
          onOpenEditor={openEditor}
          onOpenSymbol={(key) => {
            const s = useAppStore.getState();
            s.setReviewImpactFocus(worktreeId, key);
            s.setReviewLens(worktreeId, "impact");
            onSelectSymbol(key);
          }}
        />
      ) : null}
    </div>
  );
}
