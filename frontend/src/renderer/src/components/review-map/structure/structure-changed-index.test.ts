import { describe, expect, it } from "vitest";
import { buildChangedPathIndex, ROOT_FOLDER } from "./structure-changed-index";

describe("buildChangedPathIndex", () => {
  const idx = buildChangedPathIndex(
    [
      { path: "a/b/one.ts", status: "modified" },
      { path: "a/two.ts", status: "added" },
      { path: "a\\win.ts", status: "modified" },
      { path: "gone.ts", status: "deleted" },
    ],
    ["a/b/one.ts", "gone.ts"],
    ["a/two.ts"],
  );
  it("flags files", () => {
    expect([...idx.fileFlags.get("a/b/one.ts")!].sort()).toEqual([
      "changed",
      "untested",
    ]);
    expect(idx.fileFlags.get("a/two.ts")!.has("violation")).toBe(true);
    expect(idx.fileFlags.has("a/win.ts")).toBe(true);
  });
  it("counts every ancestor folder including the root", () => {
    expect(idx.folderCounts.get("a")).toEqual({
      changed: 3,
      untested: 1,
      violation: 1,
    });
    expect(idx.folderCounts.get("a/b")).toEqual({
      changed: 1,
      untested: 1,
      violation: 0,
    });
    expect(idx.folderCounts.get(ROOT_FOLDER)?.changed).toBe(3);
  });
  it("lists deleted files separately and never marks them", () => {
    expect(idx.deletedFiles).toEqual(["gone.ts"]);
    expect(idx.fileFlags.has("gone.ts")).toBe(false);
  });
});
