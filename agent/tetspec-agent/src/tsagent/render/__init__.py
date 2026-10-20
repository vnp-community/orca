"""MOD-Render: render the three 29119-3 md documents from the test KG (FR-009, MECH-07).

Fixed path `<TSAGENT_SPECS_ROOT>/<feature>/0x-*.md`. Rendered files have their hash recorded in `rendered_file`; a file on disk whose
hash differs from the recorded one (hand edit) is not overwritten unless `force`. Rendering an old version never writes to the fixed path.

The documents themselves are Vietnamese.
"""
import hashlib
import os
import tempfile

from ..config import check_feature
from ..errors import HandEditedError
from ..log import get_logger
from ..ontology import ontology
from ..store import ops, source
from ..store import tso as store_tso
from ..validate.tso_rules import Ctx, validate
from .docs import BUILDERS, View

log = get_logger("tsagent.render")


def sha(text):
    return hashlib.sha256(text.encode("utf-8")).hexdigest()


NONE_NOTE = "> Tài liệu sinh bằng luật, chưa qua LLM (`llm: none`)."  # document text (Vietnamese)


def front_matter(feature, kind, meta, body):
    lines = ["---", f"spec: TSD-{feature}-{kind}", f"feature: {feature}", f"docKind: {kind}", f"version: {meta['version']}",
             f"status: {meta['status']}", f"supersedes: {meta['version'] - 1 if meta['version'] > 1 else 'none'}",
             f"baseline: {meta.get('baseline')}",
             f"changes: [{', '.join(meta.get('changedBy') or [])}]",
             f"llm: {meta.get('llm') or 'unknown'}", f"llm_model: \"{meta.get('llmModel') or ''}\"",
             f"contentHash: {sha(body)}", "---", ""]
    return "\n".join(lines)


def llm_of(db, feature, baseline):
    """LLM mode and model of the latest run that produced the version (by source baseline); else the latest run of the feature."""
    r = db.one("SELECT mode, model FROM run WHERE feature = ? AND base_baseline_id = ? AND status <> 'Failed' "
               "ORDER BY created_at DESC, rowid DESC LIMIT 1", (feature, baseline))
    if r is None:
        r = db.one("SELECT mode, model FROM run WHERE feature = ? AND status <> 'Failed' ORDER BY created_at DESC, rowid DESC LIMIT 1",
                   (feature,))
    return (r["mode"], r["model"] or "") if r is not None else (None, "")


def _with_none_note(body):
    """Mode `none`: add a warning line right below the first heading."""
    lines = body.split("\n")
    for i, line in enumerate(lines):
        if line.startswith("# "):
            return "\n".join(lines[:i + 1] + ["", NONE_NOTE] + lines[i + 1:])
    return NONE_NOTE + "\n\n" + body


def build(db, cfg, feature, version=None):
    """Return {kind: (file name, md content)} and the version metadata."""
    check_feature(feature)
    g = store_tso.load_spec(db, feature, version)
    pkg = source.load_source(db, feature, g.meta["baseline"])
    review = {c: (lc or {}).get("review_status") for c, lc in g.meta["lifecycle"].items()}
    problems = [p for p in ops.check(db, feature) if p["rule"] in ("T16", "T19", "T20", "R17")]
    rep = validate(Ctx(g, pkg, g.meta["baseline"], auto_approve=cfg.auto_approve, review=review,
                       extra={"versioning_problems": problems}), "GS")
    g.meta.update({"report": rep.to_dict(), "history": [h for h in store_tso.history(db, feature) if h["version"] <= g.meta["version"]],
                   "primaryDoc": pkg.meta.get("primaryDoc"), "ontologyVersion": ontology().version})
    mode, model = llm_of(db, feature, g.meta["baseline"])
    g.meta.update({"llm": mode, "llmModel": model if mode not in (None, "none") else ""})
    v = View(g, pkg, g.meta)
    out = {}
    for kind, (name, fn) in BUILDERS.items():
        body = fn(v).rstrip("\n") + "\n"
        if mode == "none":
            body = _with_none_note(body)
        out[kind] = (name, front_matter(feature, kind, g.meta, body) + body)
    return out, g.meta


def _atomic_write(path, text):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=os.path.dirname(path), suffix=".tmp")
    with os.fdopen(fd, "w", encoding="utf-8", newline="\n") as fh:
        fh.write(text)
    os.replace(tmp, path)


def render(db, cfg, feature, version=None, out_dir=None, force=False):
    """Write the three documents. An old (non-latest) version needs `out_dir`; then `rendered_file` is untouched. Returns the files."""
    docs, meta = build(db, cfg, feature, version)
    latest = store_tso.versions(db, feature)[-1]["version"]
    fixed = out_dir is None
    if fixed and meta["version"] != latest:
        raise ValueError("rendering an old version needs out_dir (or use build() to get the content)")
    root = os.path.join(cfg.specs_root, feature) if fixed else out_dir
    written = []
    with db.tx():
        for kind, (name, text) in sorted(docs.items()):
            path = os.path.join(root, name)
            if fixed:
                rec = db.one("SELECT path, sha FROM rendered_file WHERE feature = ? AND kind = ?", (feature, kind))
                if os.path.exists(path) and not force:
                    with open(path, encoding="utf-8") as fh:
                        cur = fh.read()
                    if cur != text and (rec is None or sha(cur) != rec["sha"]):
                        raise HandEditedError(f"{path} was edited by hand (differs from the recorded render); use --force to overwrite")
            _atomic_write(path, text)
            if fixed:
                db.x("INSERT INTO rendered_file(feature, kind, path, sha, version) VALUES (?, ?, ?, ?, ?) "
                     "ON CONFLICT(feature, kind) DO UPDATE SET path = excluded.path, sha = excluded.sha, version = excluded.version",
                     (feature, kind, path, sha(text), meta["version"]))
            written.append(path)
    log.info("render %s v%s: %d files", feature, meta["version"], len(written))
    return {"version": meta["version"], "files": written, "gate": meta["report"]["passed"]}


__all__ = ["build", "render"]
