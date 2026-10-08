// @vitest-environment happy-dom
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";

const backend = vi.hoisted(() => ({
  calls: [] as Record<string, unknown>[],
  respond: (_p: Record<string, unknown>): unknown => ({ nodes: [], edges: [] }),
}));
vi.mock("@/hooks/useCodeIntelQuery", () => ({
  defaultCodeIntelCall: vi.fn(
    async (_w: string, _m: string, params: Record<string, unknown>) => {
      backend.calls.push(params);
      const r = backend.respond(params);
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
        : { ok: true, result: r, meta: null };
    },
  ),
}));
const storeState = vi.hoisted(() => ({
  setReviewImpactFocus: vi.fn(),
  setReviewLens: vi.fn(),
}));
vi.mock("@/store", () => ({ useAppStore: { getState: () => storeState } }));
vi.mock("@/lib/review-diff-navigation", () => ({
  openReviewFileInEditor: vi.fn(() => ({ ok: true })),
}));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));

import { TooltipProvider } from "@/components/ui/tooltip";
import StructureLensRaw from "./StructureLens";
import { MODULE_GRAPH_PAGE_1 } from "../../../test-support/code-intel-fixtures";
import { makeOverlay } from "../review-test-data";

const StructureLens = (p: React.ComponentProps<typeof StructureLensRaw>) => (
  <TooltipProvider>
    <StructureLensRaw {...p} />
  </TooltipProvider>
);
const height = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  "offsetHeight",
);
beforeAll(() =>
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    value: 600,
  }),
);
afterAll(
  () =>
    height &&
    Object.defineProperty(HTMLElement.prototype, "offsetHeight", height),
);

function props(over: Record<string, unknown> = {}) {
  return {
    worktreeId: "wt",
    environmentId: null,
    scope: {
      kind: "branch",
      baseRef: "main",
      includeUncommitted: true,
    } as never,
    overlay: makeOverlay({
      changedFiles: [{ path: "services/order/create.go", status: "modified" }],
      changedSymbols: [
        {
          symbol: {
            key: "k1",
            kind: "function",
            name: "Create",
            filePath: "services/order/create.go",
          },
          changeKind: "modified",
          tested: "no",
        },
      ],
    }),
    selectedSymbolKey: null,
    chipFilter: null,
    onSelectSymbol: vi.fn(),
    onOpenDiff: vi.fn(),
    requestSymbolChoice: vi.fn(async () => null),
    ...over,
  };
}
const flush = () =>
  act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });

beforeEach(() => {
  backend.calls = [];
  backend.respond = (p) =>
    p.path === "services/order"
      ? {
          nodes: [
            {
              id: "services/order/create.go",
              kind: "file",
              language: "go",
              symbolCount: 3,
              loc: 42,
            },
          ],
          edges: [],
        }
      : MODULE_GRAPH_PAGE_1;
});
afterEach(cleanup);

describe("StructureLens", () => {
  it("loads the root (depth 2), shows treemap + tree, and the changed mark", async () => {
    render(<StructureLens {...props()} />);
    await flush();
    expect(backend.calls[0]).toEqual({ depth: 2 });
    expect(screen.getAllByRole("treeitem").length).toBeGreaterThan(0);
    expect(screen.getByRole("tree")).toBeTruthy();
  });

  it("expanding a folder loads it once with depth 1; selecting a file shows details", async () => {
    render(<StructureLens {...props()} />);
    await flush();
    const folder = screen
      .getAllByRole("treeitem")
      .find((e) => e.getAttribute("aria-expanded") !== null)!;
    fireEvent.click(folder);
    await flush();
    expect(backend.calls.filter((c) => c.path === "services/order")).toEqual([
      { depth: 1, path: "services/order" },
    ]);
    const file = screen
      .getAllByRole("treeitem")
      .find((e) => e.textContent?.includes("create.go"))!;
    fireEvent.click(file);
    const detail = screen.getByTestId("structure-file-detail");
    expect(detail.textContent).toContain("services/order/create.go");
    expect(detail.textContent).toContain("go");
  });

  it("a changed symbol in the file opens the impact lens centred on it", async () => {
    const p = props();
    render(<StructureLens {...p} />);
    await flush();
    fireEvent.click(screen.getAllByRole("treeitem")[0]);
    await flush();
    fireEvent.click(
      screen
        .getAllByRole("treeitem")
        .find((e) => e.textContent?.includes("create.go"))!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(storeState.setReviewImpactFocus).toHaveBeenCalledWith("wt", "k1");
    expect(storeState.setReviewLens).toHaveBeenCalledWith("wt", "impact");
    expect(p.onSelectSymbol).toHaveBeenCalledWith("k1");
  });

  it("shows a retry when the root fails", async () => {
    backend.respond = () => new Error("x");
    render(<StructureLens {...props()} />);
    await flush();
    expect(screen.getByRole("alert")).toBeTruthy();
    backend.respond = () => MODULE_GRAPH_PAGE_1;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await flush();
    expect(screen.getByRole("tree")).toBeTruthy();
  });

  it('shows the empty state with "up one level" when an entered folder has nothing', async () => {
    backend.respond = (p) =>
      p.path
        ? { nodes: [], edges: [] }
        : { nodes: [MODULE_GRAPH_PAGE_1.nodes[0]], edges: [] };
    render(<StructureLens {...props()} />);
    await flush();
    fireEvent.keyDown(screen.getByRole("tree"), { key: "Enter" });
    await flush();
    expect(screen.getByTestId("structure-empty").textContent).toMatch(
      /no structure information/,
    );
    fireEvent.click(screen.getByRole("button", { name: "Up one level" }));
    expect(screen.queryByTestId("structure-empty")).toBeNull();
  });
});
