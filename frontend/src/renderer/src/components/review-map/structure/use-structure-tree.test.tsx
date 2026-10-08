// @vitest-environment happy-dom
import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { CodeIntelCallFn } from "@/hooks/useCodeIntelQuery";
import {
  STRUCTURE_MAX_CONCURRENT,
  useStructureTree,
} from "./use-structure-tree";

const folder = (id: string) => ({ id, kind: "folder", symbolCount: 0 });
const file = (id: string) => ({ id, kind: "file", symbolCount: 1 });

function deferred() {
  let resolve!: (v: unknown) => void;
  const p = new Promise((r) => (resolve = r));
  return { p, resolve };
}

function fake(responses: Record<string, unknown>) {
  const calls: Record<string, unknown>[] = [];
  let inFlight = 0;
  let maxInFlight = 0;
  const gates: Record<string, ReturnType<typeof deferred>> = {};
  const fn: CodeIntelCallFn = async (_w, _m, params) => {
    calls.push(params);
    inFlight++;
    maxInFlight = Math.max(maxInFlight, inFlight);
    const key = `${params.path ?? ""}|${params.pageToken ?? ""}`;
    if (gates[key]) {
      await gates[key].p;
    }
    inFlight--;
    const r = responses[key];
    return r instanceof Error
      ? {
          ok: false,
          error: {
            kind: "tool-failed",
            code: null,
            message: "x",
            retryable: false,
          },
        }
      : {
          ok: true,
          result: (r as { result: unknown }).result,
          meta: (r as { meta?: unknown }).meta as never,
        };
  };
  return { fn, calls, gates, maxInFlight: () => maxInFlight };
}

const mount = async (f: CodeIntelCallFn) => {
  const hook = renderHook(() =>
    useStructureTree({ worktreeId: "w", environmentId: null, callFn: f }),
  );
  await act(async () => {
    await Promise.resolve();
  });
  return hook;
};
const flush = () =>
  act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });

describe("useStructureTree", () => {
  it("loads the root with depth 2 and no path", async () => {
    const f = fake({
      "|": { result: { nodes: [folder("src"), file("a.ts")], edges: [] } },
    });
    const { result } = await mount(f.fn);
    await flush();
    expect(f.calls[0]).toEqual({ depth: 2 });
    expect(result.current.rootStatus).toBe("ready");
    expect([...result.current.state.nodes.keys()].sort()).toEqual([
      "a.ts",
      "src",
    ]);
  });

  it("requests a folder once with depth 1 even when toggled quickly", async () => {
    const f = fake({
      "|": { result: { nodes: [folder("src")], edges: [] } },
      "src|": { result: { nodes: [file("src/a.ts")], edges: [] } },
    });
    const { result } = await mount(f.fn);
    await flush();
    act(() => {
      result.current.expandFolder("src");
      result.current.collapseFolder("src");
      result.current.expandFolder("src");
    });
    await flush();
    expect(f.calls.filter((c) => c.path === "src")).toEqual([
      { depth: 1, path: "src" },
    ]);
    expect(result.current.state.nodes.has("src/a.ts")).toBe(true);
  });

  it("never exceeds 2 requests in flight", async () => {
    const f = fake({
      "|": { result: { nodes: ["a", "b", "c", "d"].map(folder), edges: [] } },
      ...Object.fromEntries(
        ["a", "b", "c", "d"].map((p) => [
          `${p}|`,
          { result: { nodes: [], edges: [] } },
        ]),
      ),
    });
    for (const p of ["a", "b", "c", "d"]) {
      f.gates[`${p}|`] = deferred();
    }
    const { result } = await mount(f.fn);
    await flush();
    act(() =>
      ["a", "b", "c", "d"].forEach((p) => result.current.expandFolder(p)),
    );
    await flush();
    expect(f.maxInFlight()).toBeLessThanOrEqual(STRUCTURE_MAX_CONCURRENT);
    for (const p of ["a", "b", "c", "d"]) {
      f.gates[`${p}|`].resolve(null);
      await flush();
    }
    expect(f.calls.filter((c) => c.path).length).toBe(4);
  });

  it("pages with the envelope token and reports totals", async () => {
    const f = fake({
      "|": {
        result: { nodes: [file("a.ts")], edges: [] },
        meta: { nextPageToken: "p2", totalCount: 2 },
      },
      "|p2": {
        result: { nodes: [file("b.ts")], edges: [] },
        meta: { totalCount: 2 },
      },
    });
    const { result } = await mount(f.fn);
    await flush();
    expect(result.current.state.folders.get("")).toMatchObject({
      nextPageToken: "p2",
      totalCount: 2,
      loaded: 1,
    });
    act(() => result.current.loadMore(""));
    await flush();
    expect(f.calls[1]).toEqual({ depth: 2, pageToken: "p2" });
    expect(result.current.state.folders.get("")).toMatchObject({
      loaded: 2,
      nextPageToken: undefined,
    });
  });

  it("one folder failing leaves loaded data intact and can be retried", async () => {
    const f = fake({
      "|": { result: { nodes: [folder("bad"), file("ok.ts")], edges: [] } },
      "bad|": new Error("x") as never,
    });
    const { result } = await mount(f.fn);
    await flush();
    act(() => result.current.expandFolder("bad"));
    await flush();
    expect(result.current.state.folders.get("bad")?.status).toBe("error");
    expect(result.current.state.nodes.has("ok.ts")).toBe(true);
    act(() => result.current.retry("bad"));
    await flush();
    expect(f.calls.filter((c) => c.path === "bad")).toHaveLength(2);
  });

  it("never sends unsafe paths and stops at 5000 nodes", async () => {
    const big = Array.from({ length: 5100 }, (_, i) => file(`f${i}.ts`));
    const f = fake({
      "|": {
        result: { nodes: big, edges: [] },
        meta: { nextPageToken: "more" },
      },
    });
    const { result } = await mount(f.fn);
    await flush();
    expect(result.current.state.nodes.size).toBe(5000);
    expect(result.current.capReached).toBe(true);
    expect(result.current.state.folders.get("")?.nextPageToken).toBeUndefined();
    act(() => result.current.expandFolder("../etc"));
    await flush();
    expect(f.calls.some((c) => c.path === "../etc")).toBe(false);
  });
});
