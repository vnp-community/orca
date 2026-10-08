import { describe, expect, it } from "vitest";
import {
  assignAreaColors,
  assignLanguageColors,
  deriveStructureArea,
  OVERFLOW_AREA_TOKEN,
  ROOT_AREA,
} from "./structure-area-model";
import { readFileSync } from "node:fs";

describe("deriveStructureArea", () => {
  it("prefers ModuleNode.area, then ChangedFile.area, then the path prefix", () => {
    expect(deriveStructureArea({ id: "x/y.go", area: "custom" }, [])).toBe(
      "custom",
    );
    expect(
      deriveStructureArea({ id: "x/y.go" }, [
        { path: "x/y.go", area: "from-change" },
      ] as never),
    ).toBe("from-change");
    expect(deriveStructureArea({ id: "frontend/src/a.ts" })).toBe("frontend");
  });
  it("maps services to <root>/<name> and handles root files and Windows paths", () => {
    expect(deriveStructureArea("backend-go/services/order/create.go")).toBe(
      "backend-go/order",
    );
    expect(deriveStructureArea("backend-go\\services\\order\\create.go")).toBe(
      "backend-go/order",
    );
    expect(deriveStructureArea("README.md")).toBe(ROOT_AREA);
    expect(deriveStructureArea("docs")).toBe("docs");
  });
});

describe("colour assignment", () => {
  it("is alphabetical, deterministic and overflows to area-6", () => {
    const m = assignAreaColors(["d", "a", "f", "b", "e", "c", "a"]);
    expect(m.get("a")?.tokenVar).toBe("--review-area-1");
    expect(m.get("e")?.tokenVar).toBe("--review-area-5");
    expect(m.get("f")?.tokenVar).toBe(OVERFLOW_AREA_TOKEN);
    expect(assignAreaColors(["b", "a"])).toEqual(assignAreaColors(["a", "b"]));
  });
  it("languages: top 5 by count then alphabetical", () => {
    const m = assignLanguageColors(
      new Map([
        ["go", 10],
        ["ts", 10],
        ["md", 1],
        ["py", 5],
        ["sh", 2],
        ["yaml", 1],
      ]),
    );
    expect(m.get("go")?.tokenVar).toBe("--review-area-1");
    expect(m.get("ts")?.tokenVar).toBe("--review-area-2");
    expect(m.get("yaml")?.tokenVar).toBe(OVERFLOW_AREA_TOKEN);
  });
  it("contains no hex colour in source", () => {
    for (const f of [
      "./structure-area-model.ts",
      "./structure-changed-index.ts",
    ]) {
      expect(readFileSync(new URL(f, import.meta.url), "utf8")).not.toMatch(
        /['"`]#[0-9a-f]{3,8}['"`]/i,
      );
    }
  });
});
