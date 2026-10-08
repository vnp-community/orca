// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type Call = {
  method: string;
  enabled?: boolean;
  params: Record<string, unknown>;
};
const h = vi.hoisted(() => ({
  calls: [] as Call[],
  result: (_c: Call): unknown => ({
    status: "idle",
    data: null,
    error: null,
    refetch: () => {},
    truncated: false,
  }),
}));
vi.mock("@/hooks/useCodeIntelQuery", () => ({
  useCodeIntelQuery: (_w: string, _e: unknown, opts: Call) => {
    h.calls.push(opts);
    return opts.enabled === false
      ? {
          status: "idle",
          data: null,
          error: null,
          refetch: () => {},
          truncated: false,
        }
      : h.result(opts);
  },
  defaultCodeIntelCall: vi.fn(),
}));
const storeState = vi.hoisted(() => ({
  reviewUiByWorktree: {} as Record<string, { impactFocusKey?: string | null }>,
  setReviewImpactFocus: vi.fn(),
}));
vi.mock("@/store", () => ({
  useAppStore: Object.assign(
    (sel: (s: unknown) => unknown) => sel(storeState),
    {
      getState: () => storeState,
    },
  ),
}));
vi.mock("@/lib/review-diff-navigation", () => ({
  openReviewFileInEditor: vi.fn(() => ({ ok: true })),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("./ImpactGraphCanvas", () => ({
  default: (p: { layout: { nodes: unknown[]; edges: unknown[] } }) => (
    <div
      data-testid="mock-canvas"
      data-nodes={p.layout.nodes.length}
      data-edges={p.layout.edges.length}
    />
  ),
}));

import { TooltipProvider } from "@/components/ui/tooltip";
import ImpactLensRaw from "./ImpactLens";
import { makeOverlay } from "../review-test-data";

const ImpactLens = (p: React.ComponentProps<typeof ImpactLensRaw>) => (
  <TooltipProvider>
    <ImpactLensRaw {...p} />
  </TooltipProvider>
);
const sym = (key: string, file = "src/a.ts") => ({
  key,
  kind: "function",
  name: key,
  filePath: file,
  startLine: 3,
});
const graph = (direction: string, names: string[], direct = true) => ({
  target: sym("c"),
  direction,
  risk: "HIGH",
  impactedCount: names.length,
  levels: [
    {
      depth: 1,
      symbols: names.map((n) => ({ symbol: sym(n), via: "calls", direct })),
    },
  ],
  affectedFlows: [],
  affectedClusters: [],
  testsCovering: [],
});
const ok = (data: unknown) => ({
  status: "success",
  data,
  error: null,
  refetch: () => {},
  truncated: false,
});

function baseProps(over: Record<string, unknown> = {}) {
  return {
    worktreeId: "wt",
    environmentId: null,
    scope: {
      kind: "branch",
      baseRef: "main",
      includeUncommitted: true,
    } as never,
    overlay: makeOverlay({
      changedSymbols: [
        { symbol: sym("c"), changeKind: "modified", tested: "no" },
      ],
    }),
    selectedSymbolKey: "c",
    chipFilter: null,
    onSelectSymbol: vi.fn(),
    onOpenDiff: vi.fn(),
    requestSymbolChoice: vi.fn(async () => null),
    ...over,
  };
}

beforeEach(() => {
  h.calls = [];
  h.result = (c) =>
    ok(
      graph(
        String(c.params.direction),
        c.params.direction === "upstream" ? ["u1"] : ["d1", "d2"],
      ),
    );
  storeState.reviewUiByWorktree = {};
});
afterEach(cleanup);

describe("ImpactLens", () => {
  it("asks to pick a changed symbol when there is no centre and sends no query", () => {
    render(<ImpactLens {...baseProps({ selectedSymbolKey: null })} />);
    expect(screen.getByTestId("impact-empty")).toBeTruthy();
    expect(h.calls.every((c) => c.enabled === false)).toBe(true);
  });

  it("queries both directions in parallel with depth 2, includeTests:false, target by key", () => {
    render(<ImpactLens {...baseProps()} />);
    const live = h.calls.filter((c) => c.enabled !== false);
    expect([...new Set(live.map((c) => c.params.direction))].sort()).toEqual([
      "downstream",
      "upstream",
    ]);
    for (const c of live) {
      expect(c.params).toMatchObject({
        target: { key: "c" },
        depth: 2,
        includeTests: false,
      });
    }
  });

  it("direction toggle disables the other query; depth change re-queries; depth stays within 1..3", () => {
    render(<ImpactLens {...baseProps()} />);
    h.calls = [];
    fireEvent.click(screen.getByRole("radio", { name: "Callers" }));
    const live = h.calls.filter((c) => c.enabled !== false);
    expect(new Set(live.map((c) => c.params.direction))).toEqual(
      new Set(["upstream"]),
    );
    fireEvent.click(screen.getByRole("radio", { name: "3" }));
    expect(h.calls.at(-1)?.params.depth).toBe(3);
    for (const c of h.calls) {
      expect(c.params.depth as number).toBeGreaterThanOrEqual(1);
      expect(c.params.depth as number).toBeLessThanOrEqual(3);
    }
  });

  it("draws lines only for direct nodes and always shows the no-edge note", () => {
    h.result = (c) => ok(graph(String(c.params.direction), ["x", "y"], false));
    render(<ImpactLens {...baseProps()} />);
    return screen.findByTestId("mock-canvas").then((el) => {
      expect(el.getAttribute("data-edges")).toBe("0");
      expect(screen.getByTestId("impact-no-edges-note")).toBeTruthy();
    });
  });

  it("list view renders columns and selecting a node calls onSelectSymbol", () => {
    const props = baseProps();
    render(<ImpactLens {...props} />);
    fireEvent.click(screen.getByRole("radio", { name: "List" }));
    expect(screen.getByTestId("impact-list")).toBeTruthy();
    const node = screen
      .getAllByTestId("impact-node")
      .find((n) => n.getAttribute("data-symbol-key") === "d1")!;
    fireEvent.click(node);
    expect(props.onSelectSymbol).toHaveBeenCalledWith("d1");
    fireEvent.keyDown(node, { key: "Enter" });
    expect(props.onOpenDiff).toHaveBeenCalledWith("src/a.ts", 3);
  });

  it('shows "none found" wording (not "unaffected") for an empty result', () => {
    h.result = (c) => ok(graph(String(c.params.direction), []));
    render(<ImpactLens {...baseProps()} />);
    expect(screen.getByTestId("impact-none").textContent).toMatch(
      /No dependencies found in the index/,
    );
  });

  it("shows a per-query error with retry", () => {
    const refetch = vi.fn();
    h.result = (c) =>
      c.params.direction === "upstream"
        ? {
            status: "error",
            data: null,
            error: { kind: "timeout" },
            refetch,
            truncated: false,
          }
        : ok(graph("downstream", ["d1"]));
    render(<ImpactLens {...baseProps()} />);
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(refetch).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("radio", { name: "List" }));
    expect(screen.getAllByTestId("impact-node").length).toBeGreaterThan(0);
  });

  it("UNKNOWN risk reads as not enough data", () => {
    h.result = (c) =>
      ok({ ...graph(String(c.params.direction), ["a"]), risk: "UNKNOWN" });
    render(<ImpactLens {...baseProps()} />);
    expect(screen.getByTestId("impact-risk").textContent).toMatch(
      /Not enough data/,
    );
  });

  it("opens the ambiguity dialog flow and re-queries by name+file", async () => {
    const choose = vi.fn(async () => ({ name: "Foo", file: "src/foo.ts" }));
    h.result = (c) =>
      c.params.target && "key" in (c.params.target as object)
        ? {
            status: "error",
            data: null,
            error: {
              kind: "ambiguous",
              data: { candidates: [{ name: "Foo", filePath: "src/foo.ts" }] },
            },
            refetch: () => {},
            truncated: false,
          }
        : ok(graph(String(c.params.direction), ["a"]));
    render(<ImpactLens {...baseProps({ requestSymbolChoice: choose })} />);
    await vi.waitFor(() =>
      expect(
        h.calls.some(
          (c) => (c.params.target as { name?: string }).name === "Foo",
        ),
      ).toBe(true),
    );
    expect(choose).toHaveBeenCalled();
  });
});
