"""Project mode: ``tdd-agent`` used inside a project folder, the way a code-indexing tool is.

``tdd-agent init`` creates ``<project>/.tdd-agent/`` (configuration, a local SQLite knowledge graph, run records, logs).
``tdd-agent generate`` reads the project's URD/SRS/SAD, builds or updates the knowledge graph and writes the TDD under
``docs/tdd``.  Every command run inside the project (or a sub-folder) finds it by walking up to the nearest
``.tdd-agent/config.toml``, as git finds ``.git``.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
from pathlib import Path

from .errors import NotFound, TddAgentError
from .store.base import SourceDoc

STATE_DIR = ".tdd-agent"
CONFIG_NAME = "config.toml"
STATE_FILE = "last-generate.json"
ROLES = ("URD", "SRS", "SAD")
AGENTS = {"claude": "claude_code", "codex": "codex_cli", "claude_code": "claude_code", "codex_cli": "codex_cli"}
GITIGNORE = ("# local state of tdd-agent: the knowledge graph and run records are rebuilt from the sources by `tdd-agent generate`\n"
             "*.db\n*.db-shm\n*.db-wal\nlogs/\nruns/\nstaging/\nlast-generate.json\n")


def find_project_root(start: Path | str) -> Path | None:
    """Nearest ancestor of ``start`` (itself included) that holds ``.tdd-agent/config.toml``."""
    here = Path(start).resolve()
    for parent in (here, *here.parents):
        if (parent / STATE_DIR / CONFIG_NAME).is_file():
            return parent
    return None


def default_tdd_id(name: str) -> str:
    slug = re.sub(r"[^A-Za-z0-9]+", "-", name).strip("-").upper()
    return f"TDD-{slug or 'PROJECT'}"


def _rel(root: Path, path: str | Path) -> str:
    p = Path(path).expanduser()
    p = p if p.is_absolute() else (Path.cwd() / p)
    try:
        return str(p.resolve().relative_to(root))
    except ValueError:
        return str(p.resolve())


def _toml(value) -> str:
    return json.dumps(value, ensure_ascii=False)


ALL_ROLES = ("BA", "SA/Dev", "SA", "BA/owner")


def parse_approver(spec: str) -> dict:
    """``id[:kind[:role,role]]`` (kind human|coding_agent, default human; roles default to all four)."""
    parts = spec.split(":")
    ident = parts[0].strip()
    kind = (parts[1].strip() if len(parts) > 1 and parts[1].strip() else "human")
    roles = [r.strip() for r in parts[2].split(",") if r.strip()] if len(parts) > 2 else list(ALL_ROLES)
    if not ident or kind not in ("human", "coding_agent") or not roles or any(r not in ALL_ROLES for r in roles):
        raise TddAgentError(f"approver must look like id[:human|coding_agent[:{','.join(ALL_ROLES)}]], got {spec!r}", code="ARGUMENT_INVALID")
    return {"id": ident, "kind": kind, "roles": roles}


def init_project(root: Path, *, name: str | None = None, tdd_id: str | None = None, docs_dir: str = "docs/tdd", sources: dict[str, str] | None = None, force: bool = False,
                 approvers: list[str] | None = None, prose: bool = True, agent: str | None = None) -> dict:
    """Create ``.tdd-agent/`` in ``root``.  An existing project is left as it is (``created`` is False) unless ``force`` rewrites its configuration."""
    if agent and agent not in AGENTS:
        raise TddAgentError(f"agent must be claude or codex, got {agent!r}", code="ARGUMENT_INVALID")
    root = root.resolve()
    state = root / STATE_DIR
    config = state / CONFIG_NAME
    if config.exists() and not force:
        import tomllib
        existing = tomllib.loads(config.read_text(encoding="utf-8")).get("project", {})
        return {"project": str(root), "created": False, "config": str(config.relative_to(root)), "name": existing.get("name") or root.name,
                "tdd_id": existing.get("tdd_id") or default_tdd_id(existing.get("name") or root.name), "docs_dir": existing.get("docs_dir", docs_dir), "sources": existing.get("sources", {})}
    name = name or root.name
    tdd_id = tdd_id or default_tdd_id(name)
    sources = {k.lower(): _rel(root, v) for k, v in (sources or {}).items() if v}
    state.mkdir(parents=True, exist_ok=True)
    lines = [
        "# tdd-agent project configuration. Relative paths are relative to the project folder (the parent of .tdd-agent/).",
        "# Everything not listed here keeps its default; see `tdd-agent config`.",
        "",
        "[project]",
        f"name = {_toml(name)}",
        f"tdd_id = {_toml(tdd_id)}",
        f"docs_dir = {_toml(docs_dir)}          # where `tdd-agent generate` writes the TDD",
        f"prose = {_toml(prose)}                     # also read the free text of the documents (uses the language model)",
        "prose_limit = 40                 # at most this many prose passages per document",
        "",
        "[project.sources]                # the project's documents; `generate` flags override these",
        f"urd = {_toml(sources.get('urd', ''))}",
        f"srs = {_toml(sources.get('srs', ''))}",
        f"sad = {_toml(sources.get('sad', ''))}",
        "",
        "# The agent that reads and writes (the model behind every skill that needs one): claude_code (default) or codex_cli. Roles: author, critic, extractor.",
        "[llm]",
        f"default = {_toml(AGENTS[agent] if agent else 'claude_code')}",
        "# [llm.roles]",
        '# critic = "codex_cli"            # e.g. a second opinion from another agent',
        "# [llm.claude_code]",
        '# model = "claude-sonnet-5-5"',
        '# effort = "medium"',
        "# [llm.codex_cli]",
        '# model = ""                      # empty = the Codex default for your account',
        "",
        "# Approvals. By default tdd-agent approves the gates of a change request itself (G1 SRS delta, G2 impact, G5 OpenSpec bundle, G7 acceptance) once the",
        "# rules passed, and records \"auto\" as the approver. Rules, the read-only SAD and the test receipts of G6 are never bypassed.",
        "# To require people, register approvers (below, or `tdd-agent init --approver id:kind:roles`): the mode then becomes manual unless you set it.",
        "# [approvals]",
        '# mode = "auto"                  # auto | manual',
        '# required = ["G5"]              # in auto mode, the gates that still wait for a person',
        "# Pipeline agents are never approvers. An approver:",
        "# [[approvals.actors]]",
        '# id = "alice"',
        '# kind = "human"                 # human | coding_agent',
        '# roles = ["BA", "SA/Dev", "SA", "BA/owner"]',
        "",
        "# Test specs generated by tsagent right after the change spec (gate GT, in parallel with G5 and coding).",
        "# `llm` is REQUIRED (no default): until it is set, the test-spec run of each change stops with LLM_NOT_CONFIGURED.",
        "[testspec]",
        "enabled = true",
        'llm = ""                         # claude-cli | anthropic | openai (key + model needed) | none (rules only)',
        "parallel = 4                     # model calls in parallel",
        "group_retries = 2                # retries of a failed group of model calls; > 50 % of a step still failing fails the run",
        'specs_dir = "docs/testspec"      # where the three test-spec documents are written',
        '# context_mode = "none"         # extra context for the test specs: none | solution | plan (one mode per run, always from here)',
        '# context_path = ""             # the Solution JSON or the OpenSpec change folder, inside the repository; a run may override it',
        '# kg_mcp = ""                    # the design KG is read over MCP: empty = this project\'s server over stdio, or http(s)://host/mcp',
        '# kg_mcp_token_env = "TDD_AGENT_MCP_TOKEN"   # bearer token of an http kg_mcp server',
        "",
        "# [render]",
        '# style = "standard"            # standard = reader document | kg = lossless knowledge-graph projection',
        '# language = "vi"               # vi | en',
        '# source_detail = "section"     # section = "SRS §5.1" | line = "SRS@1 d.463"',
        "",
    ]
    for a in map(parse_approver, approvers or []):
        lines += ["[[approvals.actors]]", f"id = {_toml(a['id'])}", f"kind = {_toml(a['kind'])}", f"roles = {_toml(a['roles'])}", ""]
    config.write_text("\n".join(lines), encoding="utf-8")
    ignore = state / ".gitignore"
    if not ignore.exists():
        ignore.write_text(GITIGNORE, encoding="utf-8")
    (root / docs_dir).mkdir(parents=True, exist_ok=True)
    return {"project": str(root), "created": True, "config": str(config.relative_to(root)), "name": name, "tdd_id": tdd_id, "docs_dir": docs_dir, "sources": sources, "approvers": [parse_approver(a)["id"] for a in approvers or []]}


def _kg_location(config) -> str:
    """Where the knowledge graph really is (``[store] knowledge`` may point anywhere), relative to the project when it is inside it."""
    url = config.store_url("knowledge")
    if url.startswith("sqlite:") and url != "sqlite::memory:":
        return _rel(config.base_dir, url[len("sqlite:"):])
    return url


def _settings(config) -> dict:
    return config.section("project")


def resolve_sources(config, flags: dict[str, str | None]) -> dict[str, Path]:
    """Role -> file, from the command line first, then ``[project.sources]``; every file must exist."""
    root = config.base_dir
    configured = _settings(config)["sources"]
    out: dict[str, Path] = {}
    for role in ROLES:
        raw = flags.get(role.lower()) or configured.get(role.lower())
        if not raw:
            continue
        p = Path(raw).expanduser()
        p = p if p.is_absolute() else ((Path.cwd() / p) if flags.get(role.lower()) else (root / p))
        if not p.is_file():
            raise TddAgentError(f"{role} document not found: {p}", code="SOURCE_FILE_MISSING")
        out[role] = p.resolve()
    if not out:
        raise TddAgentError("no source documents: pass --urd/--srs/--sad or set [project.sources] in .tdd-agent/config.toml", code="SOURCES_MISSING")
    return out


def _revision_for(store, role: str, text: str) -> SourceDoc:
    """The document id is its role; the revision is the next integer when the content differs from the latest registered one."""
    same_id = [d for d in store.list_sources(role) if d.id == role]
    candidate = SourceDoc.make(role, "0", role, text)
    for d in same_id:
        if d.content_hash == candidate.content_hash:
            return d
    numbers = [int(d.revision) for d in same_id if d.revision.isdigit()]
    return SourceDoc.make(role, str(max(numbers, default=0) + 1), role, text)


def _state_path(root: Path) -> Path:
    return root / STATE_DIR / STATE_FILE


def _read_state(root: Path) -> dict:
    try:
        return json.loads(_state_path(root).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}


def _kg_summary(rt) -> dict:
    graph = rt.knowledge.materialize()
    head = rt.knowledge.head()
    return {"snapshot": head.id if head else None, "elements": len(graph["elements"]), "edges": len(graph["edges"]), "gaps": len(graph.get("gaps", [])), "conflicts": len(graph.get("conflicts", []))}


def prepare_generation(rt, flags: dict, *, tdd_id: str | None, out: str | None, prose: bool | None, prose_limit: int | None, force: bool,
                       origin: dict | None = None) -> tuple[dict, list[str]] | str:
    """Start a ``tdd-generation`` run without advancing it; returns its run id, or (result, lines) when the TDD is up to date.

    The run's request carries what publishing needs (``params.publish``), so a background advance can write the document
    when the run completes (``publish_generation``)."""
    config = rt.config
    root = config.base_dir
    cfg = _settings(config)
    paths = resolve_sources(config, flags)
    tdd_id = tdd_id or cfg["tdd_id"] or default_tdd_id(cfg["name"] or root.name)
    docs_dir = root / (out or cfg["docs_dir"])
    docs = [_revision_for(rt.knowledge, role, p.read_text(encoding="utf-8")) for role, p in sorted(paths.items(), key=lambda kv: ROLES.index(kv[0]))]
    pins = {d.role: {"path": _rel(root, paths[d.role]), "revision": d.revision, "content_hash": d.content_hash} for d in docs}
    target = docs_dir / f"{tdd_id}.md"
    last = _read_state(root)
    if not force and last.get("tdd_id") == tdd_id and last.get("sources") == pins and target.is_file() and target.read_text(encoding="utf-8") == _published_text(rt, tdd_id):  # a knowledge store that lost the document is not "up to date"
        result = {"status": "up_to_date", "tdd_id": tdd_id, "document": _rel(root, target), "revision": last.get("revision"), "readiness": last.get("readiness"), "sources": pins, "kg": _kg_summary(rt)}
        return result, [f"Sources unchanged since the last run: {result['document']} is up to date (use --force to run again)."]
    n = len(rt.operational.list_runs(flow="tdd-generation", limit=100000)) + 1
    run_id = f"gen-{n}"
    request = {"mode": "initial", "params": {"tdd_id": tdd_id, "prose": cfg["prose"] if prose is None else prose, "prose_limit": prose_limit or cfg["prose_limit"],
                                             **({"id_class_map": dict(cfg["id_classes"])} if cfg.get("id_classes") else {}), **({"force_extract": True} if force else {})},
               "publish": {"tdd_id": tdd_id, "document": _rel(root, target), "sources": pins}}
    if origin:
        request["origin"] = origin
    rt.orchestrator.start("tdd-generation", request, run_id=run_id, sources=docs)
    return run_id


def publish_generation(rt, run_id: str) -> tuple[dict, list[str]]:
    """After a ``tdd-generation`` run: write the TDD document and the project state (completed), or report why not."""
    root = rt.config.base_dir
    run = rt.operational.get_run(run_id)
    publish = run.request.get("publish") or {}
    tdd_id, pins = publish.get("tdd_id"), publish.get("sources", {})
    target = root / publish["document"] if publish.get("document") else None
    task = rt.operational.get_task(run_id, "G3") or {}
    output = task.get("output") or {}
    if run.status != "completed" or target is None:
        blockers = list((run.result or {}).get("blockers") or output.get("blockers") or [])
        result = {"status": run.status, "run_id": run_id, "blockers": blockers, "sources": pins}
        return result, [f"Generation did not complete ({run.status}): {', '.join(map(str, blockers)) or 'see `tdd-agent run show ' + run_id + '`'}"]
    text = _published_text(rt, tdd_id)
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")
    kg = _kg_summary(rt)
    revision = output.get("tdd", {}).get("revision")
    state = {"tdd_id": tdd_id, "run_id": run_id, "revision": revision, "readiness": output.get("status"), "document": _rel(root, target), "sources": pins}
    _state_path(root).write_text(json.dumps(state, indent=2, ensure_ascii=False), encoding="utf-8")
    render = output.get("roundtrip") or {}
    questions = len(output.get("questions", []))
    result = {"status": "completed", "run_id": run_id, "tdd_id": tdd_id, "revision": revision, "readiness": output.get("status"), "document": _rel(root, target), "kg": kg,
              "questions": questions, "checks": {k: render.get(k) for k in ("ok", "anchored", "elements", "rows_without_source", "missing_blocks", "deterministic")}, "sources": pins}
    lines = [f"TDD written: {result['document']}  (revision {revision}, readiness: {output.get('status')})",
             f"Knowledge graph: {kg['elements']} elements, {kg['edges']} relations, {kg['gaps']} gaps  ({_kg_location(rt.config)})",
             f"Parts still missing information are marked THIẾU in the document ({render.get('missing_blocks', '?')} places); {questions} question(s) for the owners.",
             "Next: read the document, then `tdd-agent status`."]
    return result, lines


def generate(rt, flags: dict, *, tdd_id: str | None, out: str | None, prose: bool | None, prose_limit: int | None, force: bool) -> tuple[dict, list[str]]:
    """Read the project's documents, build or update the knowledge graph and write the TDD.  Returns (result, human lines)."""
    started = prepare_generation(rt, flags, tdd_id=tdd_id, out=out, prose=prose, prose_limit=prose_limit, force=force)
    if not isinstance(started, str):
        return started
    rt.orchestrator.advance(started)
    return publish_generation(rt, started)


def after_background_advance(rt, run_id: str) -> None:
    """Work the CLI does after a run advances in the foreground, for runs advanced in the background (run_dispatch)."""
    run = rt.operational.get_run(run_id)
    if run.flow == "tdd-generation" and run.status == "completed" and run.request.get("publish"):
        publish_generation(rt, run_id)


def _published_text(rt, tdd_id: str) -> str | None:
    try:
        return rt.knowledge.get_source(tdd_id).text
    except NotFound:
        return None


def status(rt) -> tuple[dict, list[str]]:
    config = rt.config
    root = config.base_dir
    cfg = _settings(config)
    last = _read_state(root)
    sources = []
    for role in ROLES:
        raw = cfg["sources"].get(role.lower())
        if not raw:
            continue
        p = (root / raw) if not Path(raw).is_absolute() else Path(raw)
        current = hashlib.sha256(p.read_bytes()).hexdigest() if p.is_file() else None
        registered = [d for d in rt.knowledge.list_sources(role) if d.id == role]
        known = last.get("sources", {}).get(role)
        text_hash = SourceDoc.make(role, "0", role, p.read_text(encoding="utf-8")).content_hash if p.is_file() else None
        sources.append({"role": role, "path": raw, "exists": p.is_file(), "revision": known["revision"] if known else (registered[-1].revision if registered else None),
                        "changed_since_last_generate": bool(known) and text_hash != known["content_hash"], "never_generated": not known, "file_sha256": current})
    tdds = sorted({d.id: d for d in rt.knowledge.list_sources("TDD")}.values(), key=lambda d: d.id)
    latest = {}
    for d in rt.knowledge.list_sources("TDD"):
        latest[d.id] = d
    result = {"project": {"root": str(root), "name": cfg["name"] or root.name, "tdd_id": cfg["tdd_id"] or default_tdd_id(cfg["name"] or root.name), "config": f"{STATE_DIR}/{CONFIG_NAME}"},
              "sources": sources, "knowledge": _kg_summary(rt), "documents": [{"id": d.id, "revision": d.revision, "content_hash": d.content_hash[:12]} for d in sorted(latest.values(), key=lambda d: d.id)],
              "last_generate": last or None, "needs_review": len(rt.orchestrator.elements_needing_review())}
    kg = result["knowledge"]
    lines = [f"Project {result['project']['name']} ({root})", f"Knowledge graph: {kg['elements']} elements, {kg['edges']} relations, {kg['gaps']} gaps, {kg['conflicts']} conflicts"]
    for s in sources:
        flag = "not generated yet" if s["never_generated"] else "CHANGED since the last generate" if s["changed_since_last_generate"] else "up to date"
        lines.append(f"  {s['role']}: {s['path']}  [{flag}]")
    if last:
        lines.append(f"Last TDD: {last['document']} (revision {last.get('revision')}, readiness {last.get('readiness')})")
    lines.append(f"{result['needs_review']} element(s) waiting for review (`tdd-agent review --list`).")
    return result, lines


# ====================================================================== documents on disk
def sync_documents(rt) -> list[str]:
    """Write the latest revision of every TDD known to the knowledge store into the project's docs folder (only files that differ)."""
    root = rt.config.base_dir
    docs_dir = root / _settings(rt.config)["docs_dir"]
    latest: dict[str, SourceDoc] = {}
    for d in rt.knowledge.list_sources("TDD"):
        latest[d.id] = d
    written = []
    for doc_id, d in sorted(latest.items()):
        target = docs_dir / f"{doc_id}.md"
        if not target.is_file() or target.read_text(encoding="utf-8") != d.text:
            docs_dir.mkdir(parents=True, exist_ok=True)
            target.write_text(d.text, encoding="utf-8")
            written.append(_rel(root, target))
    state = _read_state(root)
    if written and state.get("tdd_id") in latest:
        state["revision"] = latest[state["tdd_id"]].revision
        _state_path(root).write_text(json.dumps(state, indent=2, ensure_ascii=False), encoding="utf-8")
    return written


# ====================================================================== change request: impact -> change spec -> OpenSpec
def _parse_pairs(items: list[str] | None, what: str) -> dict[str, str]:
    out = {}
    for item in items or []:
        key, sep, value = item.partition("=")
        if not sep or not key.strip() or not value.strip():
            raise TddAgentError(f"{what} must look like ID=value, got {item!r}", code="ARGUMENT_INVALID")
        out[key.strip()] = value.strip()
    return out


def _ensure_openspec(root: Path) -> list[str]:
    """The change is published into <project>/openspec; create the minimal skeleton when the project has none."""
    made = []
    for rel in ("openspec/specs", "openspec/changes"):
        if not (root / rel).is_dir():
            (root / rel).mkdir(parents=True)
            made.append(rel)
    if not (root / "openspec" / "config.yaml").is_file():
        (root / "openspec" / "config.yaml").write_text("schema: spec-driven\n", encoding="utf-8")
        made.append("openspec/config.yaml")
    return made


def _change_runs(rt) -> list:
    return rt.operational.list_runs(flow="change-request", limit=100000)


def _latest_change_run(rt, run_id: str | None) -> str:
    if run_id:
        return run_id
    runs = _change_runs(rt)
    if not runs:
        raise TddAgentError("no change request yet: start one with `tdd-agent change start`", code="NO_CHANGE_RUN")
    return runs[0].run_id


def _register_cr(rt, cr_id: str, text: str) -> SourceDoc:
    """The change request as a source document: the same text is the same revision, a changed text is the next one."""
    same = [d for d in rt.knowledge.list_sources("CR") if d.id == cr_id]
    probe = SourceDoc.make(cr_id, "0", "CR", text)
    for d in same:
        if d.content_hash == probe.content_hash:
            return d
    numbers = [int(d.revision) for d in same if d.revision.isdigit()]
    return SourceDoc.make(cr_id, str(max(numbers, default=0) + 1), "CR", text)


def change_start(rt, name: str, *, cr_file: str | None, cr_text: str | None, title: str | None, acceptance: list[str], cr_id: str | None, srs_after: str | None, purpose: str | None,
                 scope: str | None, owner: str | None, code_targets: list[str] | None, verifications: list[str] | None, new_in: list[str] | None, allow_draft: bool,
                 satisfied_by: list[str] | None = None, advance: bool = True, origin: dict | None = None) -> tuple[dict, list[str]]:
    """Start the change-request flow (``advance=False``: only create the run, for a background advance).  Inputs: the CR (a file or text), the SRS and SAD the TDD was generated from, and the TDDs.

    G0 reads the CR against the SRS and proposes the SRS it would produce; G1 asks the BA to confirm the resulting SRS delta; G2 assesses the impact on
    the TDDs; G3 realizes the TDDs; G4 builds the change-spec rows; G5 writes the OpenSpec bundle.  ``srs_after`` (an SRS already edited by hand) replaces
    the derivation of the delta from the CR.
    """
    from .agents.changespec import CHANGE_NAME
    from .skills.lib import srs_edit
    if not CHANGE_NAME.match(name):
        raise TddAgentError(f"change name {name!r} must be lowercase words joined by '-', for example add-t14-reminder", code="CHANGE_NAME_INVALID")
    root = rt.config.base_dir
    last = _read_state(root)
    if not last.get("sources", {}).get("SRS") or not last.get("sources", {}).get("SAD"):
        raise TddAgentError("no baseline: run `tdd-agent init` (with SRS and SAD) before a change request", code="NO_BASELINE_TDD")
    run_id = f"change-{name}"
    if any(r.run_id == run_id for r in _change_runs(rt)):
        raise TddAgentError(f"run {run_id} already exists: see `tdd-agent change status {run_id}`", code="RUN_EXISTS")
    if cr_file and cr_text:
        raise TddAgentError("give the CR as a file (--cr) or as text (--cr-text), not both", code="ARGUMENT_INVALID")
    if cr_file:
        cr_path = Path(cr_file).expanduser()
        if not cr_path.is_file():
            raise TddAgentError(f"CR file not found: {cr_path}", code="SOURCE_FILE_MISSING")
        cr_text = cr_path.read_text(encoding="utf-8")
    if not (cr_text and cr_text.strip()):
        raise TddAgentError("no change request: give --cr <file.md> or --cr-text \"…\"", code="CR_REQUIRED")
    parsed = srs_edit.parse_cr(cr_text)
    cr_title = title or parsed["title"]
    criteria = list(dict.fromkeys(list(acceptance) + parsed["acceptance"]))
    before = rt.knowledge.get_source("SRS", last["sources"]["SRS"]["revision"])
    sad = rt.knowledge.get_source("SAD", last["sources"]["SAD"]["revision"])
    notes = []
    sad_path = resolve_sources(rt.config, {}).get("SAD")
    if sad_path and SourceDoc.make("SAD", "0", "SAD", sad_path.read_text(encoding="utf-8")).content_hash != sad.content_hash:
        notes.append("The SAD file differs from the revision the TDD was generated from; the change uses the generated one (the SAD is a read-only baseline). Run `tdd-agent generate` to adopt the new SAD first.")
    cr_identity = cr_id or parsed["cr_id"] or f"CR-{len(_change_runs(rt)) + 1}"
    cr_doc = _register_cr(rt, cr_identity, cr_text)
    sources = [before, sad, cr_doc]
    srs_params = {"id": "SRS", "before": before.revision}
    if srs_after:
        after = _revision_for(rt.knowledge, "SRS", Path(srs_after).expanduser().read_text(encoding="utf-8"))
        if after.content_hash == before.content_hash:
            raise TddAgentError(f"{srs_after} is identical to SRS revision {before.revision} that the TDD was generated from", code="NO_SRS_CHANGE")
        sources.append(after)
        srs_params["after"] = after.revision
    made = _ensure_openspec(root)
    cr = {"id": cr_identity, "revision": cr_doc.revision, "title": cr_title, "acceptance": criteria, "text_hash": cr_doc.content_hash}
    capability = {"purpose": purpose or cr_title, "scope": scope or cr_title, **({"owner": owner} if owner else {})}
    params = {"srs": srs_params, "cr": cr, "change_name": name, "repo_root": str(root), "capability": capability,
              "code_targets": _parse_pairs(code_targets, "--code-target"), "verifications": _parse_pairs(verifications, "--verify"), "tdd_for_new": _parse_pairs(new_in, "--new-in"),
              **({"satisfied_by": _parse_pairs(satisfied_by, "--satisfied-by")} if satisfied_by else {}), "allow_draft": allow_draft, "baseline_from_srs": True, **({"id_class_map": dict(_settings(rt.config)["id_classes"])} if _settings(rt.config).get("id_classes") else {})}
    request = {"mode": "change-request", "params": params}
    if origin:
        request["origin"] = origin
    rt.orchestrator.start("change-request", request, run_id=run_id, sources=sources)
    if advance:
        rt.orchestrator.advance(run_id)
    result, lines = _change_summary(rt, run_id)
    if made:
        lines.insert(0, f"Created the OpenSpec skeleton the change is published into: {', '.join(made)}.")
    return result, notes + lines


def change_name_from(title: str, taken: set[str]) -> str:
    """A folder-safe name made of the CR's title ('Giữ bằng chứng 5 năm' -> 'giu-bang-chung-5-nam'), with a number when the name is already used."""
    import unicodedata
    ascii_title = unicodedata.normalize("NFKD", title.replace("đ", "d").replace("Đ", "D")).encode("ascii", "ignore").decode()
    words = re.findall(r"[a-z0-9]+", ascii_title.lower())
    base = "-".join(words[:6])[:40].strip("-") or "change"
    name, n = base, 1
    while name in taken:
        n += 1
        name = f"{base}-{n}"
    return name


def change_run(rt, cr_file: str, *, name: str | None = None, owner: str | None = None, purpose: str | None = None, scope: str | None = None, acceptance: list[str] | None = None,
               allow_draft: bool = False, ask=None, precheck: bool = True, code_targets: list[str] | None = None, verifications: list[str] | None = None,
               satisfied_by: list[str] | None = None) -> tuple[dict, list[str]]:
    """One command for a change request: check what is needed, name the change, start it (or continue it when the same CR was started before) and run it as far as it can go.

    ``ask(question) -> answer`` is given only when a person can answer (a terminal): it is used once, for what the CR cannot say (who owns the change).
    """
    from . import doctor
    from .skills.lib import srs_edit
    path = Path(cr_file).expanduser()
    if not path.is_file():
        raise TddAgentError(f"CR file not found: {path}", code="SOURCE_FILE_MISSING")
    if precheck:
        failed = doctor.failures(doctor.check(rt.config, for_change=True))
        if failed:
            raise TddAgentError("cannot start: " + "; ".join(f"{c['name']}: {c['detail']} ({c['fix']})" for c in failed), code="PRECHECK_FAILED", details={"checks": failed})
    text = path.read_text(encoding="utf-8")
    text_hash = SourceDoc.make("CR", "0", "CR", text).content_hash
    runs = {r.run_id: r for r in _change_runs(rt)}
    if name is None:
        for r in runs.values():  # the same CR started earlier is continued, not started again
            if (r.request.get("params", {}).get("cr") or {}).get("text_hash") == text_hash and r.status != "failed":
                return change_advance(rt, r.run_id)
        name = change_name_from(srs_edit.parse_cr(text)["title"], {rid.removeprefix("change-") for rid in runs})
    elif f"change-{name}" in runs:
        existing = runs[f"change-{name}"]
        if (existing.request.get("params", {}).get("cr") or {}).get("text_hash") == text_hash:
            return change_advance(rt, existing.run_id)
        raise TddAgentError(f"a change named {name} already exists for another text of the CR: choose another --name, or see `tdd-agent change status change-{name}`", code="RUN_EXISTS")
    notes: list[str] = []
    who = owner or _settings(rt.config).get("owner") or ""
    if not who:
        who = (ask("Who owns this change (a team or a person)? ") or "").strip() if ask else ""
    if not who:
        who = _settings(rt.config)["name"] or rt.config.base_dir.name
        notes.append(f"No owner was given: the change is owned by {who!r} (use --owner, or `owner` in [project], to say who).")
    result, lines = change_start(rt, name, cr_file=str(path), cr_text=None, title=None, acceptance=list(acceptance or []), cr_id=None, srs_after=None, purpose=purpose, scope=scope,
                                 owner=who, code_targets=code_targets or [], verifications=verifications or [], new_in=[], allow_draft=allow_draft, satisfied_by=satisfied_by)
    return {**result, "name": name}, [f"Change {name}"] + notes + lines


_GATE_TITLES = {"G1": "the SRS delta (what changed in the SRS)", "G2": "the impact on the TDDs", "G5": "the OpenSpec bundle", "G7": "the acceptance of the change request"}


def _approvers(rt, gate: str) -> list[str]:
    from .orchestrator.model import GATE_APPROVAL_ROLES
    roles = set(GATE_APPROVAL_ROLES.get(gate, ()))
    return [a.id for a in rt.config.actors() if roles & set(a.roles)]


def _approver_note(rt, gate: str) -> str:
    """Who may approve a gate, by the role the gate asks for (a gate asks for one role; the list of people differs from gate to gate for that reason)."""
    from .orchestrator.model import GATE_APPROVAL_ROLES
    roles = GATE_APPROVAL_ROLES.get(gate, ())
    by_role = {r: [a.id for a in rt.config.actors() if r in a.roles] for r in roles}
    return "; ".join(f"as {r}: {', '.join(ids)}" for r, ids in by_role.items() if ids) or "nobody configured"


def _change_summary(rt, run_id: str, why: bool = False) -> tuple[dict, list[str]]:
    view = rt.orchestrator.run_view(run_id)
    run = view["run"]
    res = run["result"]
    status, pending, blocked = run["status"], res.get("pending_gate"), res.get("blocked_gate")
    summary = {"run_id": run_id, "status": status, "state": run["state"], "pending_gate": pending, "blocked_gate": blocked, "blockers": res.get("blockers", []),
               "approvals": [{k: a[k] for k in ("gate", "actor_id", "approval_grade")} for a in view["approvals"]], "documents_written": sync_documents(rt)}
    lines = [f"Change {run_id}: {status} ({run['state']})"]
    auto = [a["gate"] for a in view["approvals"] if a["approval_grade"] == "auto"]
    if auto:
        lines.append(f"Approved automatically by tdd-agent (approvals.mode = auto): {', '.join(auto)}. Set approvers or `required` in .tdd-agent/config.toml to have people decide.")
    proposal = _proposal(rt, run_id)
    if proposal:
        summary["proposal"] = proposal["summary"]
        lines += proposal["lines"]
    if status == "awaiting_approval" and pending:
        who = _approvers(rt, pending)
        summary["approvers"] = who
        lines.append(f"Waiting for the approval of {_GATE_TITLES.get(pending, pending)} at {pending}.")
        lines.append(f"  1. read it:    tdd-agent change decision {run_id} --actor <you>" + (f"     (who may, {_approver_note(rt, pending)})" if who else ""))
        lines.append(f"  2. approve it: tdd-agent change approve {run_id} --actor <you> --rationale \"…\" --hash <decision_hash>")
        if not who:
            lines.append("  No approver is configured for this gate: add [[approvals.actors]] to .tdd-agent/config.toml (see docs/CLI_Project_Usage.md).")
    elif status == "blocked":
        task = rt.operational.get_task(run_id, blocked or "") or {}
        out = task.get("output") or {}
        lines.append(f"Blocked at {blocked}: {', '.join(map(str, summary['blockers'])) or 'see `tdd-agent run show ' + run_id + '`'}")
        g4 = ((rt.operational.get_task(run_id, "G4") or {}).get("output") or {}).get("change_spec") or {}
        rows = {i["id"]: ((i.get("srs_refs") or [{}])[0]).get("section_id", "") for i in list(g4.get("items", [])) + list(g4.get("pending", []))}
        lines += _hints(summary["blockers"], rows)
        causes = explain_failures(out)
        if causes:
            summary["why"] = causes
            if why:
                lines.append("Why it is blocked:")
                for c in causes:
                    lines.append(f"  - {c['what']} ({c['rule']}, {c['count']}): " + "; ".join(c["examples"][:3]))
                    lines.append(f"      fix: {c['fix']}")
            else:
                lines.append(f"  {len(causes)} cause(s) behind it: tdd-agent change status --why")
        if out.get("draft_path"):
            summary["draft_path"] = out["draft_path"]
            lines.append(f"A DRAFT OpenSpec bundle (inspection only, never published) is at {_rel(rt.config.base_dir, Path(out['draft_path']))}")
    elif status == "awaiting_evidence":
        lines.append(f"The change is published; it waits for test results (G6): tdd-agent receipt {run_id} <CS-id> --criteria <AC,...> --result pass --actor <you>")
    elif status == "completed":
        lines.append("Accepted. The change is complete.")
    if summary["documents_written"]:
        lines.append("TDD updated: " + ", ".join(summary["documents_written"]))
    review = write_review(rt, run_id)
    if review:
        summary["review"] = review
        lines.append(f"What this change does, for review: {review}")
    ts, ts_line = testspec_status(rt, run_id)
    if ts is not None:
        summary["testspec"] = ts
        lines.append(ts_line)
    return summary, lines


def sweep_data(prop: dict) -> list[dict]:
    return list(prop.get("term_sweep") or [])


def _pid_alive(pid) -> bool:
    if not pid:
        return False
    try:
        os.kill(int(pid), 0)
    except ProcessLookupError:
        return False
    except (PermissionError, OSError, ValueError):
        return True
    return True


def testspec_status(rt, run_id: str) -> tuple[dict | None, str | None]:
    """State of the test-spec branch of a change (gate GT and its latest `testspec` child run)."""
    gt = rt.operational.get_task(run_id, "GT") or {}
    out = gt.get("output") or {}
    if gt.get("status") != "completed":
        return None, None
    if out.get("enabled") is False:
        return {"enabled": False}, "Test specs: off ([testspec] enabled = false)."
    from .agents.testspec.gate import children_of
    kids = children_of(rt.operational, run_id)
    if not kids:
        return None, None
    child = kids[0]
    task = rt.operational.get_task(child.run_id, "TS1") or {}
    res = task.get("output") or {}
    info = {"run_id": child.run_id, "status": child.status, "blockers": child.result.get("blockers", [])}
    if child.status == "running":
        if out.get("child_run_id") == child.run_id and out.get("launched") == "detached" and not _pid_alive(out.get("pid")) \
                and task.get("status") != "completed":
            info["status"] = "interrupted"
            return info, f"Test specs: stopped unexpectedly ({child.run_id}); resume with: tdd-agent run advance {child.run_id}"
        return info, f"Test specs: being generated ({child.run_id}, started {str(out.get('launched_at', ''))[:19]})."
    if child.status == "completed":
        feats = res.get("features") or []
        info.update(features=[{k: f.get(k) for k in ("feature", "version", "tr", "tc", "gaps", "files", "context") if k in f} for f in feats])
        failed = sum(len(f.get("warnings") or []) for f in feats)
        dirs = sorted({str(Path(f["files"][0]).parent) + "/" for f in feats if f.get("files")})
        line = (f"Test specs: done ({child.run_id}) · {sum(f.get('tr', 0) for f in feats)} TR, {sum(f.get('tc', 0) for f in feats)} TC, "
                f"{sum(f.get('gaps', 0) for f in feats)} gaps · {', '.join(dirs) or '-'}")
        ctx = next((f["context"] for f in feats if f.get("context")), None)
        if ctx:
            line += f" · context: {ctx['mode']} {ctx['ref']} ({ctx['nodes']} nodes)"
        if failed:
            line += f" · {failed} LLM groups failed after retry"
        return info, line
    codes = info["blockers"]
    if any(str(c).startswith("LLM_") for c in codes):
        return info, f"Test specs: configuration error {', '.join(codes)}: set [testspec] llm (claude-cli | anthropic | openai | none) in .tdd-agent/config.toml, then tdd-agent run advance {child.run_id}"
    err = next((f.get("error") for f in res.get("features") or [] if f.get("error")), "") or (task.get("evidence") or {}).get("error", "")
    return info, f"Test specs: failed {', '.join(map(str, codes))}{': ' + str(err)[:200] if err else ''}; see tdd-agent run show {child.run_id}"


def _proposal(rt, run_id: str) -> dict | None:
    """How the CR was read (change requests without an edited SRS): the edits with the sentence of the CR behind each, and the SRS they produce, saved for review."""
    out = (rt.operational.get_task(run_id, "G0") or {}).get("output") or {}
    if not out.get("cr_driven"):
        return None
    prop = out["cr_proposal"]
    run = rt.operational.get_run(run_id)
    folder = rt.config.base_dir / STATE_DIR / "changes" / run.request["params"]["change_name"]
    folder.mkdir(parents=True, exist_ok=True)
    pin = prop["proposed_srs"]
    (folder / "SRS.proposed.md").write_text(rt.knowledge.get_source(pin["id"], pin["revision"]).text, encoding="utf-8")
    (folder / "srs.diff").write_text("\n".join(prop["diff"]) + "\n", encoding="utf-8")
    files = [_rel(rt.config.base_dir, folder / "SRS.proposed.md"), _rel(rt.config.base_dir, folder / "srs.diff")]
    edits = [{k: e.get(k) for k in ("op", "id", "before", "after", "near_id", "cr_quote")} for e in prop["edits"]]
    lines = [f"Reading of the CR: {len(edits)} change(s) to the SRS, each tied to a sentence of the CR:"]
    for e in prop["edits"]:
        what = {"modify": f"modify {e['id']}", "add": f"add {e['id']} (next to {e.get('near_id')})", "remove": f"remove {e['id']}"}[e["op"]]
        lines.append(f"  - {what}: “{e.get('after') or e.get('before')}”  ← CR: “{e['cr_quote']}”")
    for u in prop.get("unmatched") or []:
        lines.append(f"  - NOT mapped to any requirement: “{u['cr_quote']}” ({u['reason']})")
    sweep = prop.get("term_sweep") or []
    for t in sweep:
        lines.append(f"  - The CR renames “{t['term']}” to “{t['renamed_to']}”; still using the old term and not changed by any edit: {', '.join(t['rows_not_changed'])}")
    lines.append(f"  Proposed SRS and its diff: {files[0]}, {files[1]}")
    return {"summary": {"edits": edits, "not_mapped": prop.get("unmatched", []), "term_sweep": sweep_data(prop), "files": files, "proposed_srs_revision": pin["revision"]}, "lines": lines}


def change_update(rt, run_id: str | None, code_targets: list[str] | None, verifications: list[str] | None, new_in: list[str] | None, *, owner: str | None = None,
                  purpose: str | None = None, scope: str | None = None, acceptance: list[str] | None = None, satisfied_by: list[str] | None = None) -> tuple[dict, list[str]]:
    """Give a running change what its gates found missing (the code a requirement changes, how to verify it, the TDD of a new requirement) and run on."""
    run_id = _latest_change_run(rt, run_id)
    run = rt.operational.get_run(run_id)
    params = run.request.get("params", {})
    merged = {"code_targets": {**params.get("code_targets", {}), **_parse_pairs(code_targets, "--code-target")},
              "verifications": {**params.get("verifications", {}), **_parse_pairs(verifications, "--verify")},
              "tdd_for_new": {**params.get("tdd_for_new", {}), **_parse_pairs(new_in, "--new-in")}}
    capability = {**params.get("capability", {}), **{k: v for k, v in (("owner", owner), ("purpose", purpose), ("scope", scope)) if v}}
    extra = list(dict.fromkeys(list(params.get("acceptance_extra", [])) + [a for a in (acceptance or []) if a]))
    satisfied = {**params.get("satisfied_by", {}), **_parse_pairs(satisfied_by, "--satisfied-by")}
    unchanged = (merged == {k: params.get(k, {}) for k in merged} and capability == params.get("capability", {}) and extra == list(params.get("acceptance_extra", []))
                 and satisfied == params.get("satisfied_by", {}))
    if unchanged:
        raise TddAgentError("nothing to update: pass --code-target REQ=path::symbol, --verify REQ=GIVEN…WHEN…THEN…, --new-in REQ=TDD-ID, --satisfied-by REQ=ELEMENT-ID, --owner, --purpose, --scope or --acceptance AC-ID", code="ARGUMENT_INVALID")
    merged.update(capability=capability, acceptance_extra=extra, satisfied_by=satisfied)
    rt.orchestrator.update_sources(run_id, [], keep_revisions=True, params=merged)
    rt.orchestrator.advance(run_id)
    return _change_summary(rt, run_id)


_BLOCK_HINTS = (("CS_CODE_TARGET_MISSING:", "the code a requirement changes is not known (no matching code in the repository): tdd-agent change update --code-target {id}=path/to/file::symbol"),
                ("CR_ACCEPTANCE_MISSING", "the CR lists no acceptance criterion: add one to the CR as `- AC-<id>: GIVEN … WHEN … THEN …`, or tdd-agent change update --acceptance AC-<id> (with --verify REQ=\"GIVEN … WHEN … THEN …\")"),
                ("CS_VERIFICATION_MISSING:", "no GIVEN/WHEN/THEN verification for it: write the CR's acceptance criteria in that form, or tdd-agent change update --verify {id}=\"GIVEN … WHEN … THEN …\""),
                ("CR_NO_REQUIREMENT_CHANGE", "the model found no requirement of the SRS that this CR changes: check the CR text (or hand-edit the SRS and use --srs-after)"),
                ("SAD_CHANGE_REQUIRED", "the SAD (read-only) says otherwise for a requirement: raise a SAD change request, or, if the SA judges the finding wrong, tdd-agent change waive --sad-element <ADR> --requirement <REQ> --actor <SA> --reason \"…\""),
                ("BASELINE_UNWRITABLE:", "the SRS revision the change starts from has no statement for this requirement, so there is nothing to put in its OpenSpec baseline; `tdd-agent baseline` shows which ones"),
                ("EDIT_NUMBER_NOT_IN_CR", "the proposed wording used a number the CR does not state: the proposal was refused, nothing was registered; rewrite the CR to state it, or hand-edit the SRS (--srs-after)"))


# what a failing rule of the OpenSpec gate means, and the command that settles it
_RULE_CAUSES = {
    "QG-03": ("facts the spec reads have not been reviewed", "tdd-agent review --list, then tdd-agent review <ids> --actor <you> --role SA --rationale \"…\" (automatic in auto mode)"),
    "QG-07": ("a requirement has no work item or no test", "tdd-agent change update --code-target REQ=path::symbol --verify REQ=\"GIVEN … WHEN … THEN …\""),
    "TS-02": ("evidence or the OpenSpec baseline does not match the sources", "tdd-agent baseline (writes the baseline of the requirements the change modifies)"),
    "TS-10": ("the OpenSpec baseline of a modified requirement is missing", "tdd-agent baseline"),
    "TS-06": ("a requirement has no acceptance, or no design element that implements it has a work item", "acceptance: write it in the CR as AC-<id>: GIVEN … WHEN … THEN …; implementation: a component that satisfies the requirement must exist (see TS-05)"),
    "QG-06": ("a requirement has no acceptance", "write the CR's acceptance as AC-<id>: GIVEN … WHEN … THEN …"),
    "TS-05": ("no design element (component, operation, entity) is recorded as satisfying a modified requirement", "say which existing element satisfies it: tdd-agent change update --satisfied-by REQ=SVC-… (a component, operation or entity of the SAD or TDD); or add the row to the SRS"),
    "TS-08": ("a gap or conflict the change must settle", "tdd-agent kg conflicts, then tdd-agent kg resolve <key> --actor <SA|BA> --role <SA|BA> --decision \"…\" --rationale \"…\""),
}


def explain_failures(output: dict) -> list[dict]:
    """The causes behind a blocked OpenSpec gate, one per root rule: a rule that only summarizes other rules (``TS-04`` listing ``QG-03: …``) is folded into them."""
    failing = output.get("failing_rules") or {}
    rule_ids = set(failing) | {"QG-03", "QG-07"}
    causes = []
    for rid, evidence in sorted(failing.items()):
        items = [str(e) for e in (evidence or [])]
        own = [e for e in items if not any(e.startswith(f"{other}:") or e.startswith(f"{other.replace('QG', 'TS')}:") for other in rule_ids if other != rid)]
        if not own:
            continue  # nothing of its own: the rules it quotes are reported
        what, fix = _RULE_CAUSES.get(rid, (f"rule {rid} failed", "see `tdd-agent run show` for the evidence"))
        if rid == "QG-03":   # name the elements, so the person reviews what the spec reads and not the whole graph
            ids = sorted({e.rsplit(":review required", 1)[0] for e in own if e.endswith(":review required")})
            fix = (f"tdd-agent review --ids {' '.join(ids[:25])}{' …' if len(ids) > 25 else ''} --decision approved --actor <you> --role SA --rationale \"…\"  ({len(ids)} element(s); in auto mode the engine does it)")
        causes.append({"rule": rid, "what": what, "fix": fix, "count": len(own), "examples": own[:4]})
    review = output.get("review") or {}
    for f in (review.get("findings") or [])[:5]:
        causes.append({"rule": "review", "what": "the independent reviewer of the written bundle found", "fix": "tdd-agent change advance (the bundle is rewritten with this finding)", "count": 1,
                       "examples": [f"{f.get('artifact')} {f.get('anchor')}: {f.get('reason') or f.get('message') or f}"[:300]]})
    return causes


def _hints(blockers: list[str], rows: dict[str, str] | None = None) -> list[str]:
    """One line per kind of blocker; ``rows`` (change-spec row -> the requirement it is about) lets the line name the requirement."""
    out = []
    for prefix, text in _BLOCK_HINTS:
        hits = [str(b) for b in blockers if str(b).startswith(prefix)]
        if not hits:
            continue
        if prefix.endswith(":") and "{id}" in text:
            for req in sorted({(rows or {}).get(h[len(prefix):], h[len(prefix):]) for h in hits}):
                out.append("  → " + text.replace("{id}", req))
        else:
            out.append("  → " + text)
    return out


def baseline(rt, run_id: str | None, force: bool = False) -> tuple[dict, list[str]]:
    """Write the OpenSpec baseline a change needs for the requirements it modifies or removes (`openspec/specs/<change>/spec.md`).

    The change flow does this by itself before the OpenSpec bundle is written; the command is for looking at it first or rewriting it with ``--force``.
    """
    from .agents.baseline import ensure_baseline
    from .agents.changespec import _row_statement
    run_id = _latest_change_run(rt, run_id)
    run = rt.operational.get_run(run_id)
    out = (rt.operational.get_task(run_id, "G1") or {}).get("output") or {}
    changes = (out.get("srs_delta") or {}).get("changes") or []
    touched = {c["id"]: _row_statement(c.get("before_quote", "")) for c in changes if c["kind"] in ("modified", "removed")}
    name = run.request["params"]["change_name"]
    if not touched:
        return {"run_id": run_id, "written": [], "kept": [], "unwritable": []}, [f"Change {run_id} modifies or removes no existing requirement (or its SRS delta is not ready yet): no baseline is needed."]
    result = ensure_baseline(rt.config.base_dir, name, touched, force=force)
    lines = [f"Baseline of {name}: {_rel(rt.config.base_dir, Path(result['path']))}"]
    lines += [f"  written: {', '.join(result['written'])}"] if result["written"] else []
    lines += [f"  already there: {', '.join(result['kept'])} (--force rewrites)"] if result["kept"] else []
    lines += [f"  cannot be written: {u['id']} - {u['reason']}" for u in result["unwritable"]]
    return {"run_id": run_id, **result}, lines


def change_waive(rt, run_id: str | None, sad_element: str, change_id: str, actor: str, rationale: str) -> tuple[dict, list[str]]:
    """The SA sets aside one finding of the SAD check (a SAD decision against one requirement), with the reason; the change runs on.

    The waiver names the pair, belongs to the impact decision the approvers read, and is not available to the automatic mode.
    """
    run_id = _latest_change_run(rt, run_id)
    me = next((a for a in rt.config.actors() if a.id == actor), None)
    if me is None or "SA" not in me.roles:
        raise TddAgentError(f"{actor} is not registered as an SA: only the SA may waive a SAD finding (see [[approvals.actors]])", code="WAIVE_NOT_PERMITTED")
    if not (rationale or "").strip():
        raise TddAgentError("a waiver needs the reason", code="RATIONALE_REQUIRED")
    out = (rt.operational.get_task(run_id, "G2") or {}).get("output") or {}
    pairs = {(f["sad_element"], f["change_id"]) for f in out.get("sad_findings", [])}
    if (sad_element, change_id) not in pairs:
        known = ", ".join(f"{a}/{b}" for a, b in sorted(pairs)) or "none"
        raise TddAgentError(f"{run_id} has no SAD finding for {sad_element} against {change_id} (findings: {known})", code="NO_SUCH_FINDING")
    run = rt.operational.get_run(run_id)
    waivers = [w for w in run.request.get("params", {}).get("sad_waivers", []) if (w["sad_element"], w["change_id"]) != (sad_element, change_id)]
    waivers.append({"sad_element": sad_element, "change_id": change_id, "actor": actor, "role": "SA", "rationale": rationale})
    rt.operational.add_audit("sad_finding_waived", {"actor": actor, "sad_element": sad_element, "change_id": change_id, "rationale": rationale}, run_id)
    rt.orchestrator.update_sources(run_id, [], keep_revisions=True, params={"sad_waivers": waivers})
    rt.orchestrator.advance(run_id)
    return _change_summary(rt, run_id)


def estimate_generate(config, flags: dict, *, prose: bool | None = None, prose_limit: int | None = None, ingested: dict | None = None) -> tuple[dict, str]:
    """How many model calls a generation will make, counted by rules from the documents (no model): one per 25 prose passages of each document, when prose is read."""
    from .skills.lib import facts, text as T
    cfg = _settings(config)
    use_prose = cfg["prose"] if prose is None else prose
    limit = prose_limit or cfg["prose_limit"]
    per: dict[str, int] = {}
    for role, path in resolve_sources(config, flags).items():
        body = path.read_text(encoding="utf-8")
        if SourceDoc.make(role, "0", role, body).content_hash in (ingested or {}).get(role, ()):
            per[role] = 0   # its text is already in the knowledge graph: it is not read again
            continue
        blocks = T.segment(body)
        covered = {el["provenance"][0]["block"] for el in facts.structured_facts(body, role, "0", blocks)["elements"] if el.get("provenance")}
        passages = [b for b in T.prose_candidates(blocks) if b.id not in covered]
        per[role] = -(-min(len(passages), limit) // 25) if use_prose else 0
    total = sum(per.values())
    return {"model_calls": total, "by_document": per, "entity_match_calls_at_most": 20}, (
        f"This reads free text with the model: about {total} call(s) ({', '.join(f'{r} {n}' for r, n in per.items())}), plus up to 20 short ones to compare similar names; --no-prose skips the first kind."
        if use_prose and total else "No model call is needed to read these documents (up to 20 short ones may compare similar names).")


def run_cost(rt, run_id: str) -> dict:
    """What a run cost: calls, tokens, money (when the provider reports it) and time, in total and per skill."""
    calls = rt.operational.list_llm_calls(run_id, limit=100000)
    per: dict[str, dict] = {}
    total = {"calls": 0, "failed": 0, "input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "cost_usd": 0.0, "latency_s": 0.0}
    for c in calls:
        usage = c.get("usage") or {}
        row = per.setdefault((c.get("skill") or {}).get("id") or c.get("gate") or "other", {"calls": 0, "tokens": 0, "latency_s": 0.0})
        tokens = int(usage.get("input_tokens") or 0) + int(usage.get("output_tokens") or 0) + int(usage.get("total_tokens") or 0)
        row["calls"] += 1; row["tokens"] += tokens; row["latency_s"] = round(row["latency_s"] + (c.get("latency_ms") or 0) / 1000, 1)
        total["calls"] += 1
        total["failed"] += 0 if c.get("status") == "ok" else 1
        total["input_tokens"] += int(usage.get("input_tokens") or 0); total["output_tokens"] += int(usage.get("output_tokens") or 0)
        total["total_tokens"] += int(usage.get("input_tokens") or 0) + int(usage.get("output_tokens") or 0) + int(usage.get("total_tokens") or 0)
        total["cost_usd"] = round(total["cost_usd"] + float(c.get("cost_usd") or 0), 4); total["latency_s"] = round(total["latency_s"] + (c.get("latency_ms") or 0) / 1000, 1)
    return {"run_id": run_id, **total, "by_skill": dict(sorted(per.items(), key=lambda kv: -kv[1]["calls"]))}


def cost_lines(cost: dict) -> list[str]:
    out = [f"{cost['run_id']}: {cost['calls']} model call(s) ({cost['failed']} failed), {cost['total_tokens']} tokens, "
           f"{cost['latency_s']} s" + (f", ${cost['cost_usd']}" if cost["cost_usd"] else "")]
    out += [f"  {skill}: {row['calls']} call(s), {row['tokens']} tokens, {row['latency_s']} s" for skill, row in cost["by_skill"].items()]
    return out


def clean(rt, yes: bool = False) -> tuple[dict, list[str]]:
    """Remove the working copies that no run needs any more: of failed runs, of runs that no longer exist, and the ``.tdd-runs`` an earlier version left in the repository.

    Never touches the knowledge graph, the run records, ``docs/`` or ``openspec/``.  Without ``yes`` it only lists.
    """
    import shutil
    root, work = rt.config.base_dir, rt.config.work_dir
    runs = {r.run_id: r for r in rt.operational.list_runs(limit=100000)}
    candidates: list[tuple[Path, str]] = []
    for parent, keep_statuses in ((work / "staging", ("running", "blocked", "awaiting_approval")), (work / "runs", ("running", "blocked", "awaiting_approval", "awaiting_evidence", "completed"))):
        if parent.is_dir():
            for d in sorted(p for p in parent.iterdir() if p.is_dir()):
                run = runs.get(d.name)
                if run is None:
                    candidates.append((d, "no run of this name exists"))
                elif run.status not in keep_statuses:
                    candidates.append((d, f"the run is {run.status}"))
    changes = work / "changes"
    if changes.is_dir():
        known = {r.request.get("params", {}).get("change_name") for r in runs.values() if r.flow == "change-request"}
        candidates += [(d, "no change of this name exists") for d in sorted(p for p in changes.iterdir() if p.is_dir()) if d.name not in known]
    legacy = root / ".tdd-runs"
    if legacy.is_dir():
        candidates.append((legacy, "working copies an earlier version left in the repository"))
    size = lambda d: sum(f.stat().st_size for f in d.rglob("*") if f.is_file())  # noqa: E731
    items = [{"path": _rel(root, d), "reason": why, "bytes": size(d)} for d, why in candidates]
    if yes:
        for d, _ in candidates:
            shutil.rmtree(d, ignore_errors=True)
    verb = "Removed" if yes else "Would remove (run again with --yes)"
    lines = [f"{verb}: {i['path']} ({i['bytes'] // 1024} KiB) - {i['reason']}" for i in items] or ["Nothing to clean."]
    return {"removed" if yes else "candidates": items}, lines


def write_review(rt, run_id: str) -> str | None:
    """``.tdd-agent/changes/<name>/review.md``: what the change does, for the person who approves it (built by rules from what the gates recorded, no model)."""
    run = rt.operational.get_run(run_id)
    name = run.request["params"]["change_name"]
    tasks = {g: (rt.operational.get_task(run_id, g) or {}) for g in ("G0", "G1", "G2", "G3", "G4", "G5")}
    g1, g2, g3, g5 = (tasks[g].get("output") or {} for g in ("G1", "G2", "G3", "G5"))
    if not g1.get("srs_delta"):
        return None
    cr = run.request["params"]["cr"]
    out = [f"# Review of the change {name}", "", f"Change request: {cr.get('id')} - {cr.get('title')}", ""]
    out += ["## What changes in the SRS", ""]
    for c in g1["srs_delta"]["changes"]:
        out.append(f"- {c['id']} ({c['kind']}): {c.get('after_quote') or c.get('before_quote') or ''}".rstrip())
    sweep = ((tasks["G0"].get("output") or {}).get("cr_proposal") or {}).get("term_sweep") or []
    for t in sweep:
        out.append(f"- renamed term “{t['term']}” → “{t['renamed_to']}” still used by: {', '.join(t['rows_not_changed'])}")
    report = g2.get("report") or {}
    if report.get("entries"):
        out += ["", "## Impact on the TDDs", ""]
        out += [f"- {e['tdd_id']}: {e['action']} - {e.get('rationale', '')}".rstrip(" -") for e in report["entries"]]
    if g2.get("sad_waived") or (tasks["G2"].get("decision") or {}).get("sad_waived"):
        out += ["", "## SAD findings set aside", ""]
        out += [f"- {w['sad_element']} against {w['change_id']}: {w['rationale']} ({w['waived_by']})" for w in (tasks["G2"].get("decision") or {}).get("sad_waived", [])]
    maps = (g3.get("tdd_set") or {}).get("mappings") or []
    if maps:
        out += ["", "## TDD revisions", ""]
        out += [f"- {m['tdd_id']}: {m['action']} (revision {m['before_revision']} → {m['after_revision']})" for m in maps]
    if g5.get("files"):
        out += ["", "## OpenSpec bundle", "", f"Staged files: {', '.join(g5['files'])}", f"Readiness: {g5.get('readiness')}"]
    proposed = ((tasks["G4"].get("output") or {}).get("proposed")) or {}
    if proposed.get("acceptance") or proposed.get("satisfied_by") or proposed.get("code_targets"):
        out += ["", "## Proposed by tdd-agent, not given by a person", "",
                "The change request did not say these; the model proposed them (each checked by rule) so the change could go on. Overrule with `tdd-agent change update`.", ""]
        out += [f"- acceptance {c['id']} for {c['change_id']}: GIVEN {c['given']} WHEN {c['when']} THEN {c['then']}  (CR: “{c['cr_quote']}”)" for c in proposed.get("acceptance", [])]
        out += [f"- {cid} is realized by {v['id']} ({v.get('name')}): {v['reason']}" for cid, v in (proposed.get("satisfied_by") or {}).items()]
        out += [f"- place of the change for {cid}: {where}" for cid, where in (proposed.get("code_targets") or {}).items()]
    approvals = rt.operational.list_approvals(run_id)
    if approvals:
        out += ["", "## Approvals", ""]
        out += [f"- {a['gate']}: {a['actor_id']} ({a['approval_grade']}) - {a.get('rationale', '')}" for a in approvals]
    folder = rt.config.work_dir / "changes" / name
    folder.mkdir(parents=True, exist_ok=True)
    (folder / "review.md").write_text("\n".join(out) + "\n", encoding="utf-8")
    return _rel(rt.config.base_dir, folder / "review.md")


def change_status(rt, run_id: str | None, why: bool = False) -> tuple[dict, list[str]]:
    return _change_summary(rt, _latest_change_run(rt, run_id), why)


def change_list(rt) -> tuple[list, list[str]]:
    rows = [{"run_id": r.run_id, "status": r.status, "state": r.state, "created_at": r.created_at} for r in _change_runs(rt)]
    return rows, [f"{r['run_id']}: {r['status']} ({r['state']})" for r in rows] or ["No change requests yet."]


def change_advance(rt, run_id: str | None, retry: bool = False) -> tuple[dict, list[str]]:
    run_id = _latest_change_run(rt, run_id)
    if retry:
        rt.orchestrator.retry(run_id)
    else:
        rt.orchestrator.advance(run_id)
    return _change_summary(rt, run_id)


def change_decision(rt, run_id: str | None, actor: str) -> tuple[dict, list[str]]:
    run_id = _latest_change_run(rt, run_id)
    gate = rt.operational.get_run(run_id).result.get("pending_gate")
    if not gate:
        raise TddAgentError(f"run {run_id} has no gate waiting for approval", code="NO_PENDING_DECISION")
    decision = rt.orchestrator.get_decision(run_id, gate, actor, source="cli")
    return decision, [f"Decision of {gate} recorded as read by {actor}.", f"Approve with: tdd-agent change approve {run_id} --actor {actor} --rationale \"…\" --hash {decision['decision_hash']}"]


def change_approve(rt, run_id: str | None, actor: str, rationale: str, decision_hash: str, role: str | None = None) -> tuple[dict, list[str]]:
    from .orchestrator import ApprovalRequest
    from .orchestrator.model import GATE_APPROVAL_ROLES
    run_id = _latest_change_run(rt, run_id)
    gate = rt.operational.get_run(run_id).result.get("pending_gate")
    if not gate:
        raise TddAgentError(f"run {run_id} has no gate waiting for approval", code="NO_PENDING_DECISION")
    mine = next((a for a in rt.config.actors() if a.id == actor), None)
    role_ok = [r for r in GATE_APPROVAL_ROLES[gate] if mine and r in mine.roles]
    if role and role not in role_ok:
        raise TddAgentError(f"{actor!r} cannot approve {gate} as {role}: {gate} asks for {' or '.join(GATE_APPROVAL_ROLES[gate])} ({_approver_note(rt, gate)})", code="APPROVER_NOT_ALLOWED")
    role = role or (role_ok[0] if role_ok else None)
    if role is None:
        raise TddAgentError(f"{actor!r} is not an approver for {gate}: allowed {_approvers(rt, gate) or 'nobody configured'}", code="APPROVER_NOT_ALLOWED")
    res = rt.orchestrator.submit_approval(ApprovalRequest(run_id, gate, actor, role, decision_hash, rationale, channel="cli"))
    if not res.accepted:
        return {"accepted": False, "code": res.code, "message": res.message}, [f"Not approved: {res.code}: {res.message}"]
    summary, lines = _change_summary(rt, run_id)
    return {"accepted": True, "gate": gate, "role": role, **summary}, [f"{gate} approved by {actor} as {role}."] + lines


# ====================================================================== edit the TDD with a prompt
def edit(rt, prompt: str | None, *, scope: list[str], operations: list[str], user: str, undo: str | None) -> tuple[dict, list[str]]:
    from .agents.promptedit import undo_last_edit
    from .agents.common import render_document
    from types import SimpleNamespace
    root = rt.config.base_dir
    if undo:
        task = rt.operational.get_task(undo, "P1") or {}
        output = task.get("output") or {}
        if output.get("status") != "committed":
            raise TddAgentError(f"run {undo} did not commit an edit, there is nothing to undo", code="NOTHING_TO_UNDO")
        res = undo_last_edit(rt.knowledge, output, actor=user)
        if res.get("status") != "undone":
            return res, [f"Not undone: {res.get('status')} (the graph changed after the edit; use a new prompt instead)"]
        head = rt.knowledge.head()
        graph = rt.knowledge.materialize(head.id)
        shim = SimpleNamespace(config=rt.config, knowledge=SimpleNamespace(_store=rt.knowledge))
        for doc in {d.id for d in rt.knowledge.list_sources("TDD")}:
            text, _ = render_document(shim, graph, doc, snapshot=head)
            rt.knowledge.register_source(SourceDoc.make(doc, str(_next_number(rt.knowledge, doc)), "TDD", text, {"snapshot": head.id, "cause": f"undo of {undo}"}))
        return {"status": "undone", "run_id": undo, "documents_written": sync_documents(rt)}, [f"Edit of {undo} undone; TDD re-rendered."]
    if not prompt or not prompt.strip():
        raise TddAgentError("give the edit as text: tdd-agent edit \"set the timeout of API-I-01 to 25 seconds\"", code="PROMPT_REQUIRED")
    n = len(rt.operational.list_runs(flow="prompt-edit", limit=100000)) + 1
    run_id = f"edit-{n}"
    request = {"params": {"prompt": prompt, "authorization": {"scope": scope or ["*"], "operations": operations}, "user_id": user}}
    rt.orchestrator.start("prompt-edit", request, run_id=run_id)
    run = rt.orchestrator.advance(run_id)
    out = (rt.operational.get_task(run_id, "P1") or {}).get("output") or {}
    status = out.get("status")
    result = {"run_id": run_id, "run_status": run.status, "status": status or run.status, "blockers": list((run.result or {}).get("blockers") or []), "target": out.get("target"),
              "affected": out.get("affected"), "readiness": out.get("readiness"), "documents_written": []}
    if status in ("committed", "committed_pending_render", "presentation_committed"):
        result["documents_written"] = sync_documents(rt)
        sec = out.get("section_edit")
        if sec:
            result["section"] = sec["section"]
            lines = [f"Section {sec['section']} {sec['title']} {'rewritten' if sec['mode'] == 'replace' else 'extended'} ({run_id}): {sec['summary']}",
                     f"TDD updated: {', '.join(result['documents_written']) or 'no document changed'}. The text is marked in the document as written by a person. Undo with: tdd-agent edit --undo {run_id}"]
        else:
            lines = [f"Edit applied ({run_id}) to {out.get('target') or 'the document'}; TDD updated: {', '.join(result['documents_written']) or 'no document changed'}.",
                     "The edited element needs review again. Undo with: tdd-agent edit --undo " + run_id]
        if status == "committed_pending_render":
            lines.append(out.get("note", "The edit is committed but rendering failed; the published documents are unchanged."))
    elif status == "clarification_required":
        result["question"] = out.get("question")
        result["candidates"] = out.get("candidates", [])
        lines = [f"Nothing changed. {out.get('question')}"] + ([f"Candidates: {', '.join(out['candidates'])}"] if out.get("candidates") else [])
    else:
        why = ", ".join(result["blockers"]) or "see `tdd-agent run show " + run_id + "`"
        lines = [f"Nothing changed: {why}"]
        if out.get("next"):
            lines.append(out["next"])
        if "UNAUTHORIZED" in result["blockers"]:
            lines.append("Allow the operation with --allow modify|add|remove and/or the element with --scope <id>.")
    return result, lines


def _next_number(store, doc_id: str) -> int:
    numbers = [int(d.revision) for d in store.list_sources("TDD") if d.id == doc_id and d.revision.isdigit()]
    return max(numbers, default=0) + 1


# ====================================================================== MCP server
MCP_SERVER_NAME = "tdd-agent"


def _mcp_block(write: bool, approvals: bool, client_actor: str) -> str:
    return f"[mcp]\nwrite = {_toml(write)}\napprovals = {_toml(approvals)}\nclient_actor = {_toml(client_actor)}\n"


def _replace_table(text: str, table: str, block: str) -> str:
    """Replace (or append) one ``[table]`` section of a TOML file, leaving every other line as it was."""
    lines = text.split("\n")
    out, skipping = [], False
    for line in lines:
        stripped = line.strip()
        if stripped.startswith("["):
            skipping = stripped == f"[{table}]"
        if not skipping:
            out.append(line)
    body = "\n".join(out).rstrip("\n")
    return (body + "\n\n" if body else "") + block


def mcp_enable(root: Path, *, write: bool = True, approvals: bool = False, client_actor: str = "") -> dict:
    """Make the project's knowledge graph available to MCP clients: ``.mcp.json`` (project scope, read by Claude Code) and the [mcp] policy."""
    import tomllib
    config = root / STATE_DIR / CONFIG_NAME
    if approvals and not client_actor:
        raise TddAgentError("--approvals needs --client-actor: the approver identity of this MCP client (it must be in [[approvals.actors]])", code="ARGUMENT_INVALID")
    text = config.read_text(encoding="utf-8")
    if approvals:
        known = [a.get("id") for a in tomllib.loads(text).get("approvals", {}).get("actors", [])]
        if client_actor not in known:
            raise TddAgentError(f"client actor {client_actor!r} is not in [[approvals.actors]] ({known or 'none configured'})", code="ARGUMENT_INVALID")
    config.write_text(_replace_table(text, "mcp", _mcp_block(write, approvals, client_actor)), encoding="utf-8")
    mcp_file = root / ".mcp.json"
    try:
        data = json.loads(mcp_file.read_text(encoding="utf-8")) if mcp_file.is_file() else {}
    except ValueError as exc:
        raise TddAgentError(f"{mcp_file} is not valid JSON: {exc}", code="MCP_FILE_INVALID") from exc
    servers = data.setdefault("mcpServers", {})
    servers[MCP_SERVER_NAME] = {"command": "tdd-agent", "args": ["-C", str(root), "mcp", "serve"]}
    mcp_file.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return {"project": str(root), "mcp_file": ".mcp.json", "server": servers[MCP_SERVER_NAME], "policy": {"write": write, "approvals": approvals, "client_actor": client_actor or None},
            "tools": "read: knowledge graph, evidence closure, sources, runs, traces" + ("; write: run_start, run_resume, changeset_propose" if write else " (read-only)") + ("; approval_submit" if approvals else "")}


def mcp_disable(root: Path) -> dict:
    config = root / STATE_DIR / CONFIG_NAME
    config.write_text(_replace_table(config.read_text(encoding="utf-8"), "mcp", _mcp_block(False, False, "")), encoding="utf-8")
    mcp_file = root / ".mcp.json"
    removed = False
    if mcp_file.is_file():
        data = json.loads(mcp_file.read_text(encoding="utf-8"))
        removed = data.get("mcpServers", {}).pop(MCP_SERVER_NAME, None) is not None
        if data.get("mcpServers") == {} and list(data) == ["mcpServers"]:
            mcp_file.unlink()
        else:
            mcp_file.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    return {"project": str(root), "removed_from_mcp_json": removed, "policy": {"write": False, "approvals": False}}
