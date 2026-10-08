/**
 * use-structure-tree.ts — FE-CV-TASK-054-03
 *
 * Folder-at-a-time loader for the Structure lens: root `depth:2`, each opened folder
 * `depth:1`, at most 2 requests in flight, each folder requested once, 5 000 loaded nodes max.
 * Why not useCodeIntelPagedQuery: the number of folders (hence queries) is dynamic and hooks
 * cannot be created per folder; this keeps the same envelope/paging semantics by hand.
 *
 * @module components/review-map/structure/use-structure-tree
 */

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { defaultCodeIntelCall } from "@/hooks/useCodeIntelQuery";
import type { CodeIntelCallFn } from "@/hooks/useCodeIntelQuery";
import type {
  ModuleGraph,
  ModuleNode,
} from "../../../../../shared/code-intel-graph-types";
import {
  isSafeStructurePath,
  normalizeModuleNodes,
  parentPath,
  ROOT_PATH,
  STRUCTURE_FOLDER_DEPTH,
  STRUCTURE_MAX_NODES,
  STRUCTURE_ROOT_DEPTH,
} from "./structure-tree-model";
import type { FolderLoad, StructureTreeState } from "./structure-tree-model";

export const STRUCTURE_MAX_CONCURRENT = 2;

type Task = { path: string; pageToken?: string };

export type UseStructureTreeResult = {
  state: StructureTreeState;
  expanded: ReadonlySet<string>;
  /** True once the 5 000-node cap stopped further loading. */
  capReached: boolean;
  rootStatus: FolderLoad["status"] | "idle";
  toggleFolder: (path: string) => void;
  expandFolder: (path: string) => void;
  collapseFolder: (path: string) => void;
  loadMore: (path: string) => void;
  retry: (path: string) => void;
};

function errorMessage(error: { message?: string; kind?: string }): string {
  return error.kind ?? error.message ?? "error";
}

export function useStructureTree(args: {
  worktreeId: string;
  environmentId: string | null;
  enabled?: boolean;
  callFn?: CodeIntelCallFn;
}): UseStructureTreeResult {
  const { worktreeId, environmentId, enabled = true } = args;
  const callRef = useRef(args.callFn ?? defaultCodeIntelCall);
  callRef.current = args.callFn ?? defaultCodeIntelCall;

  const nodesRef = useRef(new Map<string, ModuleNode>());
  const foldersRef = useRef(new Map<string, FolderLoad>());
  const requestedRef = useRef(new Set<string>());
  const queueRef = useRef<Task[]>([]);
  const runningRef = useRef(0);
  const capRef = useRef(false);
  const abortRef = useRef<AbortController | null>(null);
  const [version, setVersion] = useState(0);
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set());
  const bump = useCallback(() => setVersion((v) => v + 1), []);

  const pump = useCallback(() => {
    const ctrl = abortRef.current;
    if (!ctrl) {
      return;
    }
    while (
      runningRef.current < STRUCTURE_MAX_CONCURRENT &&
      queueRef.current.length > 0
    ) {
      const task = queueRef.current.shift()!;
      runningRef.current++;
      const prev = foldersRef.current.get(task.path);
      foldersRef.current.set(task.path, {
        status: "loading",
        loaded: prev?.loaded ?? 0,
        loadingMore: Boolean(task.pageToken),
        nextPageToken: task.pageToken ? prev?.nextPageToken : undefined,
        totalCount: prev?.totalCount,
      });
      bump();
      const params: Record<string, unknown> = {
        depth:
          task.path === ROOT_PATH
            ? STRUCTURE_ROOT_DEPTH
            : STRUCTURE_FOLDER_DEPTH,
        ...(task.path !== ROOT_PATH ? { path: task.path } : {}),
        ...(task.pageToken ? { pageToken: task.pageToken } : {}),
      };
      void callRef
        .current(
          worktreeId,
          "codeIntel.structure",
          params,
          ctrl.signal,
          environmentId,
        )
        .then((outcome) => {
          if (ctrl.signal.aborted) {
            return;
          }
          if (!outcome.ok) {
            foldersRef.current.set(task.path, {
              status: "error",
              loaded: prev?.loaded ?? 0,
              errorMessage: errorMessage(outcome.error),
            });
            return;
          }
          const graph = outcome.result as Partial<ModuleGraph> | null;
          const incoming = normalizeModuleNodes(graph?.nodes ?? []);
          let directChildren = 0;
          for (const node of incoming) {
            if (!nodesRef.current.has(node.id)) {
              if (nodesRef.current.size >= STRUCTURE_MAX_NODES) {
                capRef.current = true;
                continue;
              }
              nodesRef.current.set(node.id, node);
            }
            if (parentPath(node.id) === task.path) {
              directChildren++;
            }
          }
          const token = capRef.current
            ? undefined
            : outcome.meta?.nextPageToken;
          const total = outcome.meta?.totalCount;
          foldersRef.current.set(task.path, {
            status: "ready",
            loaded: (task.pageToken ? (prev?.loaded ?? 0) : 0) + directChildren,
            nextPageToken: token,
            totalCount: total && total > 0 ? total : undefined,
          });
        })
        .catch(() => {
          if (!ctrl.signal.aborted) {
            foldersRef.current.set(task.path, {
              status: "error",
              loaded: 0,
              errorMessage: "unknown",
            });
          }
        })
        .finally(() => {
          runningRef.current--;
          if (!ctrl.signal.aborted) {
            bump();
            pump();
          }
        });
    }
  }, [worktreeId, environmentId, bump]);

  const request = useCallback(
    (path: string, pageToken?: string) => {
      if (!isSafeStructurePath(path) || capRef.current) {
        return;
      }
      if (!pageToken) {
        if (requestedRef.current.has(path)) {
          return;
        }
        requestedRef.current.add(path);
      }
      queueRef.current.push({ path, pageToken });
      pump();
    },
    [pump],
  );

  useEffect(() => {
    if (!enabled) {
      return;
    }
    const ctrl = new AbortController();
    abortRef.current = ctrl;
    nodesRef.current = new Map();
    foldersRef.current = new Map();
    requestedRef.current = new Set();
    queueRef.current = [];
    runningRef.current = 0;
    capRef.current = false;
    setExpanded(new Set());
    request(ROOT_PATH);
    return () => {
      ctrl.abort();
      abortRef.current = null;
    };
  }, [enabled, worktreeId, environmentId, request]);

  const expandFolder = useCallback(
    (path: string) => {
      setExpanded((prev) => (prev.has(path) ? prev : new Set(prev).add(path)));
      request(path);
    },
    [request],
  );
  const collapseFolder = useCallback(
    (path: string) =>
      setExpanded((prev) => {
        if (!prev.has(path)) {
          return prev;
        }
        const next = new Set(prev);
        next.delete(path);
        return next;
      }),
    [],
  );
  const toggleFolder = useCallback(
    (path: string) =>
      expanded.has(path) ? collapseFolder(path) : expandFolder(path),
    [expanded, collapseFolder, expandFolder],
  );
  const loadMore = useCallback(
    (path: string) => {
      const token = foldersRef.current.get(path)?.nextPageToken;
      const f = foldersRef.current.get(path);
      if (token && f?.status === "ready") {
        request(path, token);
      }
    },
    [request],
  );
  const retry = useCallback(
    (path: string) => {
      if (foldersRef.current.get(path)?.status === "error") {
        requestedRef.current.delete(path);
        request(path);
      }
    },
    [request],
  );

  const state = useMemo<StructureTreeState>(
    () => ({
      nodes: new Map(nodesRef.current),
      folders: new Map(foldersRef.current),
    }),
    // version is the change signal for the ref-held maps.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [version],
  );
  return {
    state,
    expanded,
    capReached: capRef.current,
    rootStatus: state.folders.get(ROOT_PATH)?.status ?? "idle",
    toggleFolder,
    expandFolder,
    collapseFolder,
    loadMore,
    retry,
  };
}
