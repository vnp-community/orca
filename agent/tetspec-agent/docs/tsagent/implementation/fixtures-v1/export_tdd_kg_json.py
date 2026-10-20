"""Xuất KG TDD của minipay-demo thành gói `tdd-kg/v1` (ONTOLOGY-TDD mục 8).

python scripts/export_tdd_kg_json.py --demo tmp/minipay-demo --cr CR-001 --patch <tdd_patch.json> --kind slice --out <file>
--kind full: toàn bộ KG nền + mọi patch đã duyệt; --kind slice: lát cắt của CR (nút của patch + ngữ cảnh, testScope=context).
"""
import argparse
import datetime
import hashlib
import json
import pathlib
import sys

BASE_KEYS = ("label", "version", "status", "sourceDoc", "sourceSection", "owner", "description", "note", "testScope")


def to_node(n):
    out = {"code": n["code"], "class": n["class"]}
    props = {}
    for k, v in n.items():
        if k in ("code", "class") or v is None or v == "":
            continue
        if k in BASE_KEYS:
            out[k] = v
        elif k == "primary":
            out["primary"] = bool(v)
        elif k == "steps" and isinstance(v, str):
            # tdd-kg/v1 yêu cầu mảng (luật I7); KG minipay lưu các bước thành chuỗi "a -> b -> c"
            props[k] = [s.strip() for s in v.split("->") if s.strip()]
        else:
            props[k] = v
    out["props"] = props
    return out


def ontology_decl(demo):
    sys.path.insert(0, str(demo))
    from tdd_kg.ontology import CLASSES, RELATIONS
    classes = [{"name": k, "parent": v["parent"], "required": v["required"], "attrs": v["attrs"], "doc": v["doc"]}
               for k, v in sorted(CLASSES.items())]
    relations = [{"name": k, "domain": v["domain"], "range": v["range"], "doc": v["doc"]} for k, v in sorted(RELATIONS.items())]
    return {"classes": classes, "relations": relations}


def main(argv=None):
    ap = argparse.ArgumentParser()
    ap.add_argument("--demo", required=True)
    ap.add_argument("--cr", default="CR-001")
    ap.add_argument("--patch", help="patch của CR (bắt buộc với --kind slice)")
    ap.add_argument("--feature", default=None)
    ap.add_argument("--kind", choices=["slice", "full"], default="slice")
    ap.add_argument("--out", required=True)
    a = ap.parse_args(argv)
    demo = pathlib.Path(a.demo).resolve()
    sys.path.insert(0, str(demo))
    from tdd_kg import kg_patch
    from workflow import testspec as ts
    feature = a.feature or (demo / "changes" / a.cr / "feature.txt").read_text().strip()
    supersede = []
    if a.kind == "slice":
        patch = kg_patch.load(pathlib.Path(a.patch))
        nodes, edges = ts.merged_kg(a.cr, patch)
        sl, sl_edges, _ = ts.build_slice(a.cr, patch, nodes, edges, title=a.cr)
        supersede = [{"old": s["old"], "new": s["new"]} for s in patch.get("supersede", [])
                     if s.get("old") in sl and s.get("new") in sl]
    else:
        patches = [kg_patch.load(p) for p in kg_patch.approved_patches()]
        data, errs = kg_patch.merge(kg_patch.load(kg_patch.BASE_DATA), patches)
        if errs:
            raise SystemExit("KG gộp có lỗi: " + "; ".join(errs[:3]))
        sl = {n["code"]: n for n in data["nodes"]}
        sl_edges = [tuple(e[:3]) + ((e[3] if len(e) > 3 else {}),) for e in data["edges"]]
        for p in patches:
            supersede += [{"old": s["old"], "new": s["new"]} for s in p.get("supersede", [])]
    edges_out, seen = [], set()
    for e in sl_edges:
        s, r, d = e[:3]
        if (s, r, d) in seen:  # KG nền và patch có thể khai cùng một cạnh
            continue
        seen.add((s, r, d))
        props = e[3] if len(e) > 3 and isinstance(e[3], dict) else {}
        edges_out.append([s, r, d, props])
    nodes_out = [to_node(sl[c]) for c in sorted(sl)]
    body = {"nodes": nodes_out, "edges": sorted(edges_out, key=lambda x: (x[0], x[1], x[2]))}
    pkg = {
        "format": "tdd-kg/v1",
        "ontologyVersion": "tdd-core-1+ext-0",
        "feature": feature,
        "package": {"kind": a.kind, "cr": a.cr if a.kind == "slice" else None,
                    "generatedAt": datetime.datetime.now(datetime.UTC).strftime("%Y-%m-%dT%H:%M:%SZ"),
                    "sourceHash": "sha256:" + hashlib.sha256(json.dumps(body, sort_keys=True, ensure_ascii=False).encode()).hexdigest()},
        "ontology": ontology_decl(demo),
        **body,
        "supersede": supersede,
    }
    pathlib.Path(a.out).write_text(json.dumps(pkg, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    print(f"{a.out}: {len(nodes_out)} nút, {len(edges_out)} cạnh, {len(supersede)} supersede")


if __name__ == "__main__":
    main()
