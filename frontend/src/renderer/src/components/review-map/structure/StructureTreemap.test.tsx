// @vitest-environment happy-dom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { readFileSync } from "node:fs";
import { TooltipProvider } from "@/components/ui/tooltip";
import { StructureTreemap } from "./StructureTreemap";
import { StructureToolbar } from "./StructureToolbar";
import type { TreemapCellMeta } from "./structure-treemap-model";

afterEach(cleanup);

const cell = (
  id: string,
  over: Partial<TreemapCellMeta> = {},
): TreemapCellMeta => ({
  id,
  name: id.split("/").pop()!,
  kind: "file",
  value: 10,
  group: "svc",
  tokenVar: "--review-area-1",
  flags: new Set(),
  changedCount: 0,
  symbolCount: 10,
  dimmed: false,
  ...over,
});
const handlers = () => ({
  onEnterFolder: vi.fn(),
  onSelectFile: vi.fn(),
  onOpenTree: vi.fn(),
  onUp: vi.fn(),
});

describe("StructureTreemap", () => {
  it("draws one group per cell, labels big cells with name and area, and is role=img", () => {
    const h = handlers();
    render(
      <StructureTreemap
        cells={[
          cell("a/big.go", { value: 100 }),
          cell("a/two.go", { value: 1 }),
        ]}
        size={{ width: 800, height: 400 }}
        {...h}
      />,
    );
    expect(screen.getByRole("img").getAttribute("aria-label")).toMatch(
      /Treemap of 2 items/,
    );
    expect(document.querySelectorAll("[data-cell]").length).toBe(2);
    expect(screen.getAllByText("big.go").length).toBe(1);
    expect(screen.getAllByText("svc").length).toBeGreaterThan(0);
  });
  it('clicking a folder enters it, a file selects it, "+N small" opens the tree; Esc goes up', () => {
    const h = handlers();
    const cells = [
      cell("f", { kind: "folder", value: 1000 }),
      cell("a.go", { value: 900 }),
    ];
    for (let i = 0; i < 30; i++) {
      cells.push(cell(`tiny${i}.go`, { value: 1 }));
    }
    const { container } = render(
      <StructureTreemap
        cells={cells}
        size={{ width: 300, height: 200 }}
        {...h}
      />,
    );
    fireEvent.click(container.querySelector('[data-cell="f"]')!);
    expect(h.onEnterFolder).toHaveBeenCalledWith("f");
    fireEvent.click(container.querySelector('[data-cell="a.go"]')!);
    expect(h.onSelectFile).toHaveBeenCalledWith("a.go");
    fireEvent.click(container.querySelector('[data-cell="__small__"]')!);
    expect(h.onOpenTree).toHaveBeenCalled();
    fireEvent.keyDown(container.firstChild as Element, { key: "Escape" });
    expect(h.onUp).toHaveBeenCalled();
  });
  it("marks changed cells with an overlay stroke token and a dot, never a hex colour", () => {
    const h = handlers();
    const { container } = render(
      <StructureTreemap
        cells={[cell("c.go", { flags: new Set(["changed", "untested"]) })]}
        size={{ width: 400, height: 300 }}
        {...h}
      />,
    );
    const rect = container.querySelector('[data-cell="c.go"] rect')!;
    expect(rect.getAttribute("stroke")).toBe("var(--review-changed)");
    expect(rect.getAttribute("stroke-dasharray")).toBeTruthy();
    expect(container.querySelector('[data-mark="changed"]')).toBeTruthy();
    expect(container.innerHTML).not.toMatch(/#[0-9a-f]{3,8}\b/i);
  });
  it("source files contain no hex colour literals", () => {
    for (const f of [
      "./StructureTreemap.tsx",
      "./StructureToolbar.tsx",
      "./StructureTree.tsx",
    ]) {
      expect(readFileSync(new URL(f, import.meta.url), "utf8")).not.toMatch(
        /['"`]#[0-9a-f]{3,8}['"`]/i,
      );
    }
  });
});

describe("StructureToolbar", () => {
  const props = {
    currentPath: "a/b",
    sizeBy: "symbols" as const,
    colorBy: "area" as const,
    locAvailable: false,
    legendFlags: ["changed"] as const,
    groups: new Map([["svc", "--review-area-1"]]),
    onSizeBy: vi.fn(),
    onColorBy: vi.fn(),
    onNavigate: vi.fn(),
  };
  it("disables Lines without loc, has breadcrumbs, text legend", () => {
    render(
      <TooltipProvider>
        <StructureToolbar {...props} />
      </TooltipProvider>,
    );
    expect(
      (screen.getByRole("radio", { name: "Lines" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "a" }));
    expect(props.onNavigate).toHaveBeenCalledWith("a");
    fireEvent.click(screen.getByRole("button", { name: "Repository" }));
    expect(props.onNavigate).toHaveBeenCalledWith("");
    expect(screen.getByText("svc")).toBeTruthy();
  });
});
