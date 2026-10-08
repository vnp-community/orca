// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeAll,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import type { ModuleNode } from "../../../../../shared/code-intel-graph-types";
import { buildChangedPathIndex } from "./structure-changed-index";
import { flattenStructureRows } from "./structure-tree-model";
import type { StructureTreeState } from "./structure-tree-model";
import { StructureTree } from "./StructureTree";

afterEach(cleanup);

// happy-dom has no layout: give the scroll container a viewport for the virtualizer.
const heightDescriptor = Object.getOwnPropertyDescriptor(
  HTMLElement.prototype,
  "offsetHeight",
);
beforeAll(() => {
  Object.defineProperty(HTMLElement.prototype, "offsetHeight", {
    configurable: true,
    value: 600,
  });
});
afterAll(() => {
  if (heightDescriptor) {
    Object.defineProperty(
      HTMLElement.prototype,
      "offsetHeight",
      heightDescriptor,
    );
  }
});

const folder = (id: string): ModuleNode => ({
  id,
  kind: "folder",
  symbolCount: 0,
});
const file = (id: string): ModuleNode => ({ id, kind: "file", symbolCount: 2 });

function setup(
  nodes: ModuleNode[],
  expanded: string[] = [],
  folders: StructureTreeState["folders"] = new Map(),
) {
  const treeState: StructureTreeState = {
    nodes: new Map(nodes.map((n) => [n.id, n])),
    folders,
  };
  const handlers = {
    onSelectFile: vi.fn(),
    onEnterFolder: vi.fn(),
    onToggleFolder: vi.fn(),
    onExpandFolder: vi.fn(),
    onCollapseFolder: vi.fn(),
    onLoadMore: vi.fn(),
    onRetry: vi.fn(),
  };
  const rows = flattenStructureRows(treeState, new Set(expanded));
  const changed = buildChangedPathIndex(
    [{ path: "src/a.ts", status: "modified" }],
    [],
    [],
  );
  render(
    <StructureTree
      rows={rows}
      treeState={treeState}
      changed={changed}
      selectedPath={null}
      {...handlers}
    />,
  );
  return { handlers, tree: screen.getByRole("tree") };
}

describe("StructureTree", () => {
  const nodes = [
    folder("src"),
    file("src/a.ts"),
    file("src/b.ts"),
    file("README.md"),
  ];

  it("exposes aria-level/expanded/setsize/posinset", () => {
    setup(nodes, ["src"]);
    const src = screen.getAllByRole("treeitem")[0];
    expect(src.getAttribute("aria-expanded")).toBe("true");
    expect(src.getAttribute("aria-level")).toBe("1");
    expect(src.getAttribute("aria-setsize")).toBe("2");
    expect(src.getAttribute("aria-posinset")).toBe("1");
    const a = screen.getAllByRole("treeitem")[1];
    expect(a.getAttribute("aria-level")).toBe("2");
  });

  it("arrow keys move the active descendant; Home/End jump", () => {
    const { tree } = setup(nodes, ["src"]);
    const first = tree.getAttribute("aria-activedescendant");
    fireEvent.keyDown(tree, { key: "ArrowDown" });
    expect(tree.getAttribute("aria-activedescendant")).not.toBe(first);
    fireEvent.keyDown(tree, { key: "End" });
    expect(tree.getAttribute("aria-activedescendant")).toBe("st-README_md");
    fireEvent.keyDown(tree, { key: "Home" });
    expect(tree.getAttribute("aria-activedescendant")).toBe("st-src");
  });

  it("ArrowRight expands a closed folder, ArrowLeft collapses an open one or goes to the parent", () => {
    const closed = setup(nodes);
    fireEvent.keyDown(closed.tree, { key: "ArrowRight" });
    expect(closed.handlers.onExpandFolder).toHaveBeenCalledWith("src");
    cleanup();
    const open = setup(nodes, ["src"]);
    fireEvent.keyDown(open.tree, { key: "ArrowLeft" });
    expect(open.handlers.onCollapseFolder).toHaveBeenCalledWith("src");
    fireEvent.keyDown(open.tree, { key: "ArrowDown" });
    fireEvent.keyDown(open.tree, { key: "ArrowLeft" });
    expect(open.tree.getAttribute("aria-activedescendant")).toBe("st-src");
  });

  it("Enter enters a folder or selects a file; * is ignored", () => {
    const { tree, handlers } = setup(nodes, ["src"]);
    fireEvent.keyDown(tree, { key: "Enter" });
    expect(handlers.onEnterFolder).toHaveBeenCalledWith("src");
    fireEvent.keyDown(tree, { key: "ArrowDown" });
    fireEvent.keyDown(tree, { key: "Enter" });
    expect(handlers.onSelectFile).toHaveBeenCalledWith("src/a.ts");
    fireEvent.keyDown(tree, { key: "*" });
    expect(handlers.onExpandFolder).not.toHaveBeenCalled();
  });

  it("renders error rows with retry and load-more rows with shown/total", () => {
    const folders: StructureTreeState["folders"] = new Map([
      ["src", { status: "error", loaded: 0 }],
      ["", { status: "ready", loaded: 2, nextPageToken: "t", totalCount: 9 }],
    ]);
    const { handlers } = setup(nodes, ["src"], folders);
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(handlers.onRetry).toHaveBeenCalledWith("src");
    expect(screen.getByText("2/9")).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Load more" }));
    expect(handlers.onLoadMore).toHaveBeenCalledWith("");
  });

  it("renders a window, not 10000 rows", () => {
    const many = Array.from({ length: 10000 }, (_, i) =>
      file(`f${String(i).padStart(5, "0")}.ts`),
    );
    setup(many);
    expect(screen.getAllByRole("treeitem").length).toBeLessThan(100);
  });

  it("marks changed files with an icon and an accessible count", () => {
    setup(nodes, ["src"]);
    expect(screen.getAllByLabelText(/changed/).length).toBeGreaterThan(0);
  });
});
