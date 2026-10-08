// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const q = vi.hoisted(() => ({
  calls: [] as { method: string; params: unknown; enabled?: boolean }[],
  byMethod: {} as Record<string, unknown>,
  source: {} as unknown,
}));
vi.mock("@/hooks/useCodeIntelQuery", () => ({
  useCodeIntelQuery: (
    _w: string,
    _e: unknown,
    opts: { method: string; params: unknown; enabled?: boolean },
  ) => {
    q.calls.push(opts);
    return (
      q.byMethod[opts.method] ?? {
        status: "idle",
        data: null,
        error: null,
        refetch: vi.fn(),
      }
    );
  },
  defaultCodeIntelCall: vi.fn(async () => q.source),
}));
const storeState = vi.hoisted(() => ({
  setReviewImpactFocus: vi.fn(),
  setReviewLens: vi.fn(),
  setReviewDataFlowId: vi.fn(),
}));
vi.mock("@/store", () => ({ useAppStore: { getState: () => storeState } }));
vi.mock("@/lib/review-diff-navigation", () => ({
  openReviewFileInEditor: vi.fn(() => ({ ok: true })),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
// The note button is store-bound and covered by notes/; here only its anchor matters.
vi.mock("../notes/ReviewNoteButton", () => ({
  ReviewNoteButton: ({ anchor }: { anchor: unknown }) => (
    <span data-testid="note-button" data-anchor={JSON.stringify(anchor)} />
  ),
}));

import { SymbolDetailPanel } from "./SymbolDetailPanel";
import { defaultCodeIntelCall } from "@/hooks/useCodeIntelQuery";
import { SYMBOL_DETAIL_WITH_SOURCE } from "../../../test-support/code-intel-fixtures";

const props = {
  worktreeId: "wt",
  environmentId: null,
  scope: { kind: "branch", baseRef: "main", includeUncommitted: true } as never,
  selectedSymbolKey: "k",
  onClose: vi.fn(),
  onOpenDiff: vi.fn(),
};
const ok = (data: unknown) => ({
  status: "success",
  data,
  error: null,
  refetch: vi.fn(),
});

beforeEach(() => {
  q.calls = [];
  q.byMethod = {
    symbol: ok({
      ...SYMBOL_DETAIL_WITH_SOURCE,
      source: null,
      sourceOmitted: "not_requested",
    }),
  };
  q.source = { ok: true, result: SYMBOL_DETAIL_WITH_SOURCE };
  vi.mocked(defaultCodeIntelCall).mockClear();
});
afterEach(cleanup);

describe("SymbolDetailPanel", () => {
  it("makes exactly one header symbol call with includeSource:false and no source/tests call", () => {
    render(<SymbolDetailPanel {...props} />);
    const symbolCalls = q.calls.filter((c) => c.method === "symbol");
    expect(symbolCalls.length).toBeGreaterThan(0);
    for (const c of symbolCalls) {
      expect(c.params).toMatchObject({ key: "k", includeSource: false });
    }
    expect(q.calls.some((c) => c.method === "impact")).toBe(false);
    expect(defaultCodeIntelCall).not.toHaveBeenCalled();
  });

  it("shows header, callers, and navigates to a neighbour then back", () => {
    render(<SymbolDetailPanel {...props} />);
    expect(screen.getByText("Create")).toBeTruthy();
    fireEvent.click(screen.getByText("TestCreate"));
    expect(q.calls.at(-1)?.params).toMatchObject({
      key: SYMBOL_DETAIL_WITH_SOURCE.incoming.calls[0].key,
    });
    fireEvent.click(screen.getByRole("button", { name: "Back" }));
    expect(q.calls.at(-1)?.params).toMatchObject({ key: "k" });
  });

  it("loads source only when the section opens, with includeSource:true", async () => {
    render(<SymbolDetailPanel {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /Source/ }));
    expect(defaultCodeIntelCall).toHaveBeenCalledTimes(1);
    expect(vi.mocked(defaultCodeIntelCall).mock.calls[0][2]).toMatchObject({
      includeSource: true,
    });
    expect(await screen.findByText("func Create() {}")).toBeTruthy();
  });

  it("hides the Source section when the backend says forbidden", async () => {
    q.source = {
      ok: false,
      error: { kind: "forbidden", code: null, message: "x", retryable: false },
    };
    render(<SymbolDetailPanel {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /Source/ }));
    await vi.waitFor(() =>
      expect(screen.queryByRole("button", { name: /Source/ })).toBeNull(),
    );
    expect(screen.getByText("Create")).toBeTruthy();
  });

  it("lazy tests: impact includeTests only after opening", () => {
    q.byMethod.impact = ok({ testsCovering: [] });
    render(<SymbolDetailPanel {...props} />);
    expect(q.calls.some((c) => c.method === "impact")).toBe(false);
    fireEvent.click(screen.getByRole("button", { name: /Covering tests/ }));
    const c = q.calls.find((x) => x.method === "impact");
    expect(c?.params).toMatchObject({ includeTests: true, depth: 1 });
    expect(
      screen.getByText(/No covering test found in the index/),
    ).toBeTruthy();
  });

  it("not-found shows the stale-index message and keeps the panel alive", () => {
    q.byMethod.symbol = {
      status: "error",
      data: null,
      error: { kind: "not-found" },
      refetch: vi.fn(),
    };
    render(<SymbolDetailPanel {...props} />);
    expect(screen.getByRole("alert").textContent).toMatch(
      /not found in the index/,
    );
  });

  it("set as center writes the impact focus and switches lens", () => {
    render(<SymbolDetailPanel {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /Set as center/ }));
    expect(storeState.setReviewImpactFocus).toHaveBeenCalled();
    expect(storeState.setReviewLens).toHaveBeenCalledWith("wt", "impact");
  });

  it("opens a related flow in the flows lens by its id", () => {
    render(<SymbolDetailPanel {...props} />);
    fireEvent.click(screen.getByRole("button", { name: /Related flows/ }));
    fireEvent.click(screen.getByRole("button", { name: /Create order/ }));
    expect(storeState.setReviewDataFlowId).toHaveBeenCalledWith("wt", "flow-1");
    expect(storeState.setReviewLens).toHaveBeenCalledWith("wt", "dataflow");
  });

  it("offers a review note anchored to the symbol in the impact lens", () => {
    render(<SymbolDetailPanel {...props} />);
    const anchor = JSON.parse(
      screen.getByTestId("note-button").getAttribute("data-anchor") ?? "{}",
    );
    expect(anchor).toMatchObject({ kind: "graph-node", lens: "impact" });
    expect(anchor.nodeKey).toBe(SYMBOL_DETAIL_WITH_SOURCE.symbol.key);
  });

  it("notes which commit the line numbers follow when the index is stale or overlay", () => {
    const status = (overall: string) =>
      ({ overall, tools: [{ indexedCommit: "abcdef1234567890" }] }) as never;
    const { rerender } = render(
      <SymbolDetailPanel {...props} indexStatus={status("STALE")} />,
    );
    expect(
      screen.getByText("Line numbers follow the index at abcdef1."),
    ).toBeTruthy();
    rerender(<SymbolDetailPanel {...props} indexStatus={status("OVERLAY")} />);
    expect(screen.getByText(/follow the index at abcdef1/)).toBeTruthy();
    rerender(<SymbolDetailPanel {...props} indexStatus={status("READY")} />);
    expect(screen.queryByText(/follow the index at/)).toBeNull();
  });
});
