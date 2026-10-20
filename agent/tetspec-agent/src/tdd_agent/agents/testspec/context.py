"""Extra context for the test-spec run (SOLUTION-INTEGRATE-TDD-AGENT section 14).

``[testspec] context_mode`` chooses what the run reads besides the design KG (one mode per run, always from the configuration):

* ``none``     nothing (the default; the run is the one of sections 3-8).
* ``solution`` the Solution document of the request that caused the change: the chosen option, its assumptions, open questions, risks
               and the acceptance criteria of the request (JSON, the shape Orca stores; see ``parse_solution``). An unapproved
               Solution is used too: only a note says so.
* ``plan``     the Plan of that request, an OpenSpec change: ``tasks.md`` (tasks with Serves / Code / Verify / Done when), the scenarios
               of ``specs/**/spec.md``, ``proposal.md`` and ``design.md``.

This module reads and normalises a document into a ``Bundle`` of items and writes documents sent over MCP. The projection puts the items
into the tdd-kg/v1 package as ordinary nodes (``Projector._inject_context``), so tsagent grounds, cites and validates them like any other
source. Nothing here calls a model or the network.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
from dataclasses import dataclass, field
from pathlib import Path

from ...errors import TddAgentError

MODES = ("none", "plan", "solution")
LABEL = {"solution": "SOLUTION", "plan": "PLAN"}
# An element code named in free text: FR-NOTE-03, BRL-02, ADR-02 (letters first, then dash-separated parts).
CODE_IN_TEXT = re.compile(r"(?<![A-Za-z0-9_\-])([A-Z][A-Z0-9]{0,7}(?:-[A-Za-z0-9]+)+)(?![A-Za-z0-9_])")
SEVERITY = {"low": "Low", "medium": "Medium", "high": "High", "critical": "High"}
MAX_APPROACH = 1200
MAX_TEXT = 600
# Limits of a document sent over MCP (TDD INT-FR-24).
MAX_FILE_BYTES = 256 * 1024
MAX_FILES = 40
MAX_TOTAL_BYTES = 2 * 1024 * 1024
ALLOWED_NAME = re.compile(r"^(?:solution\.json|tasks\.md|proposal\.md|design\.md|specs/[A-Za-z0-9._-]+/spec\.md)$")
REF_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$")


@dataclass
class Item:
    kind: str                 # decision | constraint | assumption | open_question | risk | acceptance
    key: str                  # id inside the source document (opt-1, A-2, Q-1, T1.1, SC1, D1 ...)
    text: str
    props: dict = field(default_factory=dict)
    anchors: list[str] = field(default_factory=list)   # element codes the item names itself


@dataclass
class Bundle:
    mode: str
    path: str                 # relative to the repository root
    sha256: str
    ref: str                  # short reference used in node codes (REQ-2)
    items: list[Item]
    hints: list[str]          # element codes named anywhere in the document, in order of appearance
    notes: list[str] = field(default_factory=list)
    snapshot: str = ""        # design KG snapshot the document says it was written on ("" when it does not say)

    @property
    def label(self) -> str:
        return LABEL[self.mode]

    def meta(self) -> dict:
        return {"mode": self.mode, "path": self.path, "ref": self.ref, "sha256": self.sha256}


# ------------------------------------------------------------------ helpers
def slug(s: str) -> str:
    return re.sub(r"[^A-Za-z0-9]+", "-", str(s)).strip("-") or "X"


def clip(s: object, n: int = MAX_TEXT) -> str:
    t = re.sub(r"\s+", " ", str(s or "")).strip()
    return t if len(t) <= n else t[: n - 1] + "…"


def codes_in(*texts: object) -> list[str]:
    out: list[str] = []
    for t in texts:
        for m in CODE_IN_TEXT.finditer(str(t or "")):
            if m.group(1) not in out:
                out.append(m.group(1))
    return out


def _invalid(msg: str) -> TddAgentError:
    return TddAgentError(msg, code="TESTSPEC_CONTEXT_INVALID")


def _missing(msg: str) -> TddAgentError:
    return TddAgentError(msg, code="TESTSPEC_CONTEXT_MISSING")


def _inside(root: Path, p: Path) -> Path:
    r, q = Path(root).resolve(), p.resolve()
    try:
        q.relative_to(r)
    except ValueError:
        raise TddAgentError(f"context path {p} is outside the repository", code="TESTSPEC_CONTEXT_PATH") from None
    return q


def context_params(cfg, params: dict | None) -> tuple[str, str]:
    """(mode, path): the mode comes from ``[testspec] context_mode`` only; a run may override the path (``params.context_path``)."""
    ts = cfg.section("testspec")
    p = params or {}
    return ts["context_mode"], str(p.get("context_path") or ts["context_path"] or "").strip()


def resolve_source(cfg, mode: str, rel: str) -> Path:
    """The file the mode reads first: ``solution.json`` (a file, or a folder holding it) or ``tasks.md`` (a folder holding it, or the file)."""
    if not rel:
        raise _missing(f"[testspec] context_mode = {mode!r} needs a document: set [testspec] context_path (or params.context_path of the run)")
    p = _inside(cfg.repo_root, Path(cfg.repo_root) / rel)
    if mode == "solution":
        if p.is_dir():
            p = p / "solution.json"
        if not p.is_file():
            raise _missing(f"solution document not found: {rel} (a JSON file, or a folder with solution.json)")
        return p
    tasks = p if p.is_file() else p / "tasks.md"
    if not tasks.is_file():
        raise _missing(f"plan not found: {rel} (an OpenSpec change folder with tasks.md, or the tasks.md file)")
    return tasks


def _spec_files(tasks: Path) -> list[Path]:
    d = tasks.parent / "specs"
    return sorted(d.glob("**/spec.md")) if d.is_dir() else []


def plan_files(tasks: Path) -> list[Path]:
    """Every file of an OpenSpec change this module reads, in a fixed order."""
    return [tasks, *[f for f in (tasks.parent / "proposal.md", tasks.parent / "design.md") if f.is_file()], *_spec_files(tasks)]


def _digest(mode: str, src: Path) -> str:
    h = hashlib.sha256()
    for f in [src] if mode == "solution" else plan_files(src):
        h.update(f.name.encode())
        h.update(f.read_bytes())
    return h.hexdigest()


def fingerprint(cfg, params: dict | None = None) -> dict:
    """Part of the input hash of GT and TS1: a different mode, path or document content runs the test specs again."""
    mode, rel = context_params(cfg, params)
    if mode == "none":
        return {"mode": "none"}
    try:
        src = resolve_source(cfg, mode, rel)
    except TddAgentError as exc:     # reported by the gate itself; the hash only has to differ from a good state
        return {"mode": mode, "path": rel, "error": exc.code}
    return {"mode": mode, "path": rel, "sha256": _digest(mode, src)}


def load(cfg, params: dict | None = None) -> Bundle | None:
    """The bundle of the configured mode, or None when ``context_mode`` is ``none``."""
    mode, rel = context_params(cfg, params)
    if mode == "none":
        return None
    return read_bundle(cfg, mode, rel, str((params or {}).get("context_ref") or "").strip())


def read_bundle(cfg, mode: str, rel: str, ref: str = "") -> Bundle:
    """Read and normalise the document of ``mode`` at ``rel`` (relative to the repository), whatever ``[testspec] context_mode`` says."""
    src = resolve_source(cfg, mode, rel)
    sha = _digest(mode, src)
    shown = str(src.resolve().relative_to(Path(cfg.repo_root).resolve()))
    if mode == "solution":
        try:
            obj = json.loads(src.read_text(encoding="utf-8"))
        except (OSError, ValueError) as exc:
            raise _invalid(f"{shown}: not a JSON document ({exc})") from None
        return parse_solution(obj, shown, sha, ref)
    return parse_plan(src, shown, sha, ref)


# ------------------------------------------------------------------ documents sent over MCP
def check_ref(ref: object) -> str:
    if not isinstance(ref, str) or not REF_RE.match(ref):
        raise TddAgentError("context.ref must be 1 to 64 characters of letters, digits, '.', '_' or '-'", code="TESTSPEC_CONTEXT_FILE")
    return ref


def _payload_files(kind: str, payload: dict) -> dict[str, str]:
    """Name -> text of the files of a document sent over MCP, after every check (nothing is written yet)."""
    document, files = payload.get("document"), payload.get("files")
    if (document is None) == (files is None):
        raise TddAgentError("context needs exactly one of 'document' (solution) or 'files' (plan)", code="TESTSPEC_CONTEXT_FILE")
    if kind == "solution":
        if files is not None or not isinstance(document, dict):
            raise TddAgentError("context.kind = 'solution' takes 'document': the Solution as a JSON object", code="TESTSPEC_CONTEXT_FILE")
        out = {"solution.json": json.dumps(document, ensure_ascii=False, indent=1, sort_keys=True) + "\n"}
    else:
        if document is not None or not isinstance(files, dict):
            raise TddAgentError("context.kind = 'plan' takes 'files': an object of file name -> text", code="TESTSPEC_CONTEXT_FILE")
        out = {}
        for name, text in files.items():
            if not isinstance(name, str) or not ALLOWED_NAME.match(name) or name == "solution.json" or ".." in name.split("/"):
                raise TddAgentError(f"file name not allowed: {name!r} (tasks.md, proposal.md, design.md, specs/<name>/spec.md)", code="TESTSPEC_CONTEXT_FILE")
            if not isinstance(text, str):
                raise TddAgentError(f"{name}: the content must be text", code="TESTSPEC_CONTEXT_FILE")
            out[name] = text
        if "tasks.md" not in out:
            raise TddAgentError("a plan needs tasks.md", code="TESTSPEC_CONTEXT_FILE")
    if len(out) > MAX_FILES:
        raise TddAgentError(f"too many files: {len(out)} (at most {MAX_FILES})", code="TESTSPEC_CONTEXT_TOO_LARGE")
    total = 0
    for name, text in out.items():
        n = len(text.encode("utf-8"))
        total += n
        if n > MAX_FILE_BYTES:
            raise TddAgentError(f"{name} is {n} bytes (at most {MAX_FILE_BYTES})", code="TESTSPEC_CONTEXT_TOO_LARGE")
    if total > MAX_TOTAL_BYTES:
        raise TddAgentError(f"the documents are {total} bytes (at most {MAX_TOTAL_BYTES})", code="TESTSPEC_CONTEXT_TOO_LARGE")
    return out


def write_inline(cfg, kind: str, ref: str, payload: dict) -> dict:
    """Write a document sent over MCP to ``<work_dir>/context/<kind>-<ref>/`` and return ``{path, files}`` (path relative to the repository).

    Checks first, writes after: a refused call (or a document that does not read) leaves the folder as it was. The files are written to a
    temporary folder, read back with the reader of the mode, and only then replace the folder of the same ``ref`` in two renames, so a
    reader never sees half a document."""
    if kind not in ("solution", "plan"):
        raise TddAgentError("context.kind must be 'solution' or 'plan'", code="TESTSPEC_CONTEXT_FILE")
    ref = check_ref(ref)
    files = _payload_files(kind, payload)
    base = Path(cfg.work_dir) / "context"
    target = base / f"{kind}-{ref}"
    try:
        _inside(cfg.repo_root, target)
    except TddAgentError:
        raise TddAgentError(f"documents sent over MCP are written under {base}, which is outside the repository {cfg.repo_root}: "
                            "keep [paths] work_dir inside the repository", code="TESTSPEC_CONTEXT_PATH") from None
    base.mkdir(parents=True, exist_ok=True)
    tmp = base / f".tmp-{kind}-{ref}-{os.getpid()}"
    old = base / f".old-{kind}-{ref}-{os.getpid()}"
    for d in (tmp, old):
        shutil.rmtree(d, ignore_errors=True)
    try:
        for name, text in files.items():
            f = tmp / name
            f.parent.mkdir(parents=True, exist_ok=True)
            f.write_text(text, encoding="utf-8")
        read_bundle(cfg, kind, str(tmp.resolve().relative_to(Path(cfg.repo_root).resolve())), ref)
        if target.exists():
            target.rename(old)
        tmp.rename(target)
    finally:
        shutil.rmtree(tmp, ignore_errors=True)
        shutil.rmtree(old, ignore_errors=True)
    return {"path": str(target.resolve().relative_to(Path(cfg.repo_root).resolve())), "files": sorted(files)}


# ------------------------------------------------------------------ solution
def parse_solution(obj: dict, path: str, sha: str, ref: str = "") -> Bundle:
    """Solution document as Orca stores it. Both shapes are read:

    * the document itself: ``{"options": [...], "recommendation": {...}, "assumptions": [...], "open_questions": [...], ...}``;
    * the stored record: ``{"options": {<the document>}, "chosenOption": 0 | "opt-1", "status": "approved"}``.

    Optional siblings: ``chosen_option`` (index or id), ``acceptance_criteria`` ``[{"id", "text"}]`` and ``request`` ``{"ref", "title"}``.
    Only the chosen option feeds the run (the options that lost are not the design of the change). A Solution that is not approved is used
    too; a note says so."""
    if not isinstance(obj, dict):
        raise _invalid(f"{path}: the solution must be a JSON object")
    obj = obj.get("solution") if isinstance(obj.get("solution"), dict) else obj
    doc = obj["options"] if isinstance(obj.get("options"), dict) else obj
    options = doc.get("options")
    if not isinstance(options, list) or not options or not all(isinstance(o, dict) for o in options):
        raise _invalid(f"{path}: the solution has no options")
    notes: list[str] = []
    status = str(obj.get("status") or "").lower()
    if status and status != "approved":
        notes.append(f"solution status is {status!r}, not approved: its assumptions are proposals")
    chosen, how = _chosen(obj, doc, options)
    if how != "chosen":
        notes.append(f"no chosen option in the document: the {how} option {chosen.get('id')} is used")
    req = obj.get("request") if isinstance(obj.get("request"), dict) else {}
    ref = ref or str(req.get("ref") or obj.get("requestRef") or "SOL")
    items: list[Item] = []
    areas = [str(a.get("name", "")) for a in chosen.get("affected_areas") or [] if isinstance(a, dict)]
    asked = [clip(q.get("text")) for q in doc.get("open_questions") or [] if isinstance(q, dict) and not q.get("blocking")]
    hints = codes_in(*areas, chosen.get("title"), chosen.get("summary"), chosen.get("approach"),
                     *[c.get("note") for c in doc.get("requirement_coverage") or [] if isinstance(c, dict)])
    reason = ""
    rec = doc.get("recommendation") if isinstance(doc.get("recommendation"), dict) else {}
    if rec.get("option_id") == chosen.get("id"):
        reason = clip(rec.get("reason"))
    items.append(Item("decision", str(chosen.get("id") or "opt"), clip(f"{chosen.get('title', '')}. {chosen.get('summary', '')}".strip(". ")),
                      {"rationale": reason, "approach": clip(chosen.get("approach"), MAX_APPROACH), "affectedAreas": areas,
                       "rollback": clip(chosen.get("rollback")), "openQuestions": asked, "optionTitle": clip(chosen.get("title"), 200)},
                      anchors=codes_in(*areas)))
    for r in chosen.get("risks") or []:
        if isinstance(r, dict) and r.get("description"):
            items.append(Item("risk", f"R{len([i for i in items if i.kind == 'risk']) + 1}", clip(r["description"]),
                              {"impact": SEVERITY.get(str(r.get("severity", "")).lower(), "Medium")}, anchors=codes_in(r["description"])))
    for a in doc.get("assumptions") or []:
        if isinstance(a, dict) and a.get("text"):
            kind = "assumption" if a.get("needs_confirmation") else "constraint"
            items.append(Item(kind, str(a.get("id") or f"A{len(items)}"), clip(a["text"]), {"needsConfirmation": bool(a.get("needs_confirmation"))},
                              anchors=codes_in(a["text"])))
    for q in doc.get("open_questions") or []:
        if isinstance(q, dict) and q.get("blocking") and q.get("text"):
            items.append(Item("open_question", str(q.get("id") or f"Q{len(items)}"), clip(q["text"]), {}, anchors=codes_in(q["text"])))
    cover = {str(c.get("ac_id")): c for c in doc.get("requirement_coverage") or [] if isinstance(c, dict)}
    for ac in obj.get("acceptance_criteria") or doc.get("acceptance_criteria") or []:
        if isinstance(ac, dict) and ac.get("id") and ac.get("text"):
            c = cover.get(str(ac["id"]), {})
            items.append(Item("acceptance", str(ac["id"]), clip(ac["text"]),
                              {"coverage": c.get("status"), "coverageNote": clip(c.get("note")), "option": chosen.get("id")},
                              anchors=codes_in(ac["text"], c.get("note"))))
    hints += [c for c in codes_in(*[i.text for i in items if i.kind == "acceptance"]) if c not in hints]
    refs = doc.get("design_references") if isinstance(doc.get("design_references"), dict) else {}
    return Bundle("solution", path, sha, ref, items, hints, notes, snapshot=str(refs.get("snapshot") or ""))


def _chosen(obj: dict, doc: dict, options: list[dict]) -> tuple[dict, str]:
    pick = obj.get("chosen_option", obj.get("chosenOption"))
    if isinstance(pick, int) and not isinstance(pick, bool) and 0 <= pick < len(options):
        return options[pick], "chosen"
    if isinstance(pick, str):
        for o in options:
            if o.get("id") == pick:
                return o, "chosen"
    rec = doc.get("recommendation") if isinstance(doc.get("recommendation"), dict) else {}
    for o in options:
        if o.get("id") == rec.get("option_id"):
            return o, "recommended"
    for o in options:
        if o.get("recommended"):
            return o, "recommended"
    return options[0], "first"


# ------------------------------------------------------------------ plan
TASK_RE = re.compile(r"^- \[[ xX]\] T(\d+)\.(\d+) (.*)$")
ATTR_RE = re.compile(r"\s*\[([a-z]+)=([^\]]*)\]")
PHASE_RE = re.compile(r"^##\s+(PH-\d+|TASKS)\b\s*(.*)$")
FIELD_RE = re.compile(r"^\s{2}(Serves|Code|Verify|Done when):\s*(.*)$")
SCENARIO_RE = re.compile(r"^#{3,5}\s+Scenario:\s*(.+?)\s*$")
REQUIREMENT_RE = re.compile(r"^#{2,4}\s+Requirement:\s*(.+?)\s*$")
STEP_RE = re.compile(r"^\s*[-*]\s+\*\*(GIVEN|WHEN|THEN|AND)\*\*\s*(.*)$", re.IGNORECASE)
CODE_TARGET_RE = re.compile(r"(create|modify|delete)\s+`([^`]+)`", re.IGNORECASE)
VERIFY_RE = re.compile(r"^(.*?)\s+via\s+`([^`]+)`\s*$")
SECTION_RE = re.compile(r"^##\s+(.+?)\s*$")
BULLET_RE = re.compile(r"^\s{0,3}[-*]\s+(.*\S)\s*$")
ASSUMPTION_RE = re.compile(r"assum", re.IGNORECASE)
PROPOSAL_SECTIONS = {"why": "why", "what changes": "changes", "capabilities": "capabilities", "impact": "impact", "rollback": "rollback"}
DESIGN_SECTIONS = {"context": "context", "goals": "goals", "goals / non-goals": "goals", "non-goals": "nongoals", "decisions": "decisions",
                   "risks / trade-offs": "risks", "risks": "risks", "trade-offs": "risks", "open questions": "questions",
                   "migration plan": "migration", "rollback": "rollback"}


def _sections(text: str, known: dict[str, str]) -> tuple[dict[str, list[str]], list[str]]:
    """Lines of each ``## name`` section the document knows (names are case-insensitive), and the names it does not."""
    out: dict[str, list[str]] = {}
    unknown: list[str] = []
    cur: str | None = None
    for line in text.splitlines():
        m = SECTION_RE.match(line)
        if m:
            name = m.group(1).strip().lower()
            cur = known.get(name)
            if cur is None:
                unknown.append(m.group(1).strip())
            else:
                out.setdefault(cur, [])
            continue
        if cur is not None:
            out[cur].append(line)
    return out, unknown


def _bullets(lines: list[str]) -> list[str]:
    """One entry per list item; an indented continuation line joins the item above it."""
    out: list[str] = []
    for line in lines:
        b = BULLET_RE.match(line)
        if b:
            out.append(b.group(1))
        elif out and line.startswith("  ") and line.strip():
            out[-1] += " " + line.strip()
    return out


def _prose(lines: list[str]) -> str:
    return clip(" ".join(x.strip() for x in lines if x.strip() and not x.lstrip().startswith("#")), 4000)


def parse_proposal(text: str) -> tuple[list[Item], list[str], list[str]]:
    """``proposal.md`` -> one Decision (what changes and why). Returns (items, element codes named, notes)."""
    sec, unknown = _sections(text, PROPOSAL_SECTIONS)
    notes = [f"proposal.md: section {n!r} is not read" for n in unknown]
    changes = _bullets(sec.get("changes", []))
    why, impact = _prose(sec.get("why", [])), _prose(sec.get("impact", []))
    if not changes and not why:
        return [], [], notes + ["proposal.md has neither 'Why' nor 'What Changes': it gives no decision"]
    caps = [clip(b, 200) for b in _bullets(sec.get("capabilities", []))]
    statement = clip("; ".join(changes) or why)
    props = {"why": clip(why), "capabilities": caps, "impact": clip(impact), "rollback": clip(_prose(sec.get("rollback", [])))}
    named = codes_in(why, " ".join(changes), impact)
    return [Item("decision", "PROPOSAL", statement, props, anchors=named)], named, notes


def parse_design(text: str) -> tuple[list[Item], list[str], list[str]]:
    """``design.md`` -> Decisions, Risks, Assumptions (a risk item that says "assumed"), Constraints (non-goals).
    Open questions stay a note on the decisions (they never block). Returns (items, element codes named, notes)."""
    sec, unknown = _sections(text, DESIGN_SECTIONS)
    notes = [f"design.md: section {n!r} is not read" for n in unknown]
    context = clip(_prose(sec.get("context", [])))
    asked = [clip(b) for b in _bullets(sec.get("questions", []))]
    items: list[Item] = []
    named: list[str] = []
    for n, b in enumerate(_bullets(sec.get("decisions", [])), 1):
        items.append(Item("decision", f"D{n}", clip(b), {"context": context, "openQuestions": asked}, anchors=codes_in(b)))
    for n, b in enumerate(_bullets(sec.get("risks", [])), 1):
        kind = "assumption" if ASSUMPTION_RE.search(b) else "risk"
        items.append(Item(kind, f"R{n}", clip(b), {"impact": "Medium"} if kind == "risk" else {}, anchors=codes_in(b)))
    for n, b in enumerate(_bullets(sec.get("nongoals", [])), 1):
        items.append(Item("constraint", f"N{n}", clip(b), {"constraintType": "non-goal"}, anchors=codes_in(b)))
    for it in items:
        named += [c for c in it.anchors if c not in named]
    named += [c for c in codes_in(context) if c not in named]
    if not items:
        notes.append("design.md has no decision, risk or non-goal in a section this reader knows")
    return items, named, notes


def parse_plan(tasks_path: Path, path: str, sha: str, ref: str = "") -> Bundle:
    """The Plan of an OpenSpec change: tasks (their Done when sentence is a checkable statement; Code and Verify say where and how it
    is checked), the spec scenarios, ``proposal.md`` (what changes and why) and ``design.md`` (decisions, risks, non-goals).
    ``tasks.md`` is required; a missing ``proposal.md`` or ``design.md`` is a note."""
    tasks_path = Path(tasks_path)
    text = tasks_path.read_text(encoding="utf-8")
    m = re.search(r"<!--\s*orca:begin plan\s+request=(\S+)", text)
    ref = ref or (m.group(1) if m else "PLAN")
    body = text.split("<!-- orca:begin", 1)[-1] if m else text
    body = body.split("<!-- orca:end", 1)[0]
    items: list[Item] = []
    notes: list[str] = []
    hints: list[str] = []
    phase = ""
    cur: dict | None = None

    def flush():
        nonlocal cur
        if cur is None:
            return
        f = cur["fields"]
        done = f.get("Done when", "")
        if not done:
            notes.append(f"task T{cur['id']} has no Done when sentence: it gives no acceptance criterion")
        else:
            ver = VERIFY_RE.match(f.get("Verify", ""))
            serves = codes_in(f.get("Serves", ""))
            hints.extend(c for c in serves if c not in hints)
            items.append(Item("acceptance", f"T{cur['id']}", clip(done),
                              {"task": clip(cur["title"], 200), "phase": phase, "type": cur["attrs"].get("type", ""),
                               "acceptanceIds": [x for x in cur["attrs"].get("ac", "").split(",") if x],
                               "dependsOn": [x for x in cur["attrs"].get("depends", "").split(",") if x],
                               "codeTargets": [f"{a.lower()} {t}" for a, t in CODE_TARGET_RE.findall(f.get("Code", ""))],
                               "verifyScenario": clip(ver.group(1)) if ver else clip(f.get("Verify")),
                               "verifyCommand": ver.group(2) if ver else ""}, anchors=serves))
        cur = None

    for line in body.splitlines():
        ph = PHASE_RE.match(line)
        if ph:
            flush()
            phase = f"{ph.group(1)} {ph.group(2)}".strip()
            continue
        t = TASK_RE.match(line)
        if t:
            flush()
            attrs = dict(ATTR_RE.findall(t.group(3)))
            cur = {"id": f"{t.group(1)}.{t.group(2)}", "title": ATTR_RE.sub("", t.group(3)).strip(), "attrs": attrs, "fields": {}}
            continue
        f = FIELD_RE.match(line)
        if f and cur is not None:
            cur["fields"][f.group(1)] = f.group(2).strip()
    flush()
    n = 0
    for spec in _spec_files(tasks_path):
        req_name, req_lines = "", []
        cur_sc: dict | None = None

        def close_scenario(s=spec, name=lambda: req_name, lines=lambda: req_lines):
            nonlocal n, cur_sc
            if cur_sc and cur_sc["steps"]:
                n += 1
                anchors = codes_in(cur_sc["name"], *lines())
                hints.extend(c for c in anchors if c not in hints)
                rel = str(s.relative_to(tasks_path.parent))
                items.append(Item("acceptance", f"SC{n}", clip(" ".join(f"{k.upper()} {v}" for k, v in cur_sc["steps"])),
                                  {"scenario": clip(cur_sc["name"], 200), "requirement": clip(name(), 200), "file": rel}, anchors=anchors))
            cur_sc = None

        for line in spec.read_text(encoding="utf-8").splitlines():
            r = REQUIREMENT_RE.match(line)
            if r:
                close_scenario()
                req_name, req_lines = r.group(1), []
                continue
            sc = SCENARIO_RE.match(line)
            if sc:
                close_scenario()
                cur_sc = {"name": sc.group(1), "steps": []}
                continue
            st = STEP_RE.match(line)
            if st and cur_sc is not None:
                cur_sc["steps"].append((st.group(1), st.group(2).strip()))
            elif cur_sc is None and line.strip() and not line.startswith("#"):
                req_lines.append(line)
        close_scenario()
    for name, parser in (("proposal.md", parse_proposal), ("design.md", parse_design)):
        f = tasks_path.parent / name
        if not f.is_file():
            notes.append(f"{name} not found in the change: it adds no context")
            continue
        got, named, more = parser(f.read_text(encoding="utf-8"))
        items.extend(got)
        hints.extend(c for c in named if c not in hints)
        notes.extend(more)
    if not any(i.kind == "acceptance" for i in items) and not items:
        raise _invalid(f"{path}: no task with a Done when sentence, no spec scenario, no proposal and no design was found")
    return Bundle("plan", path, sha, ref, items, hints, notes)
