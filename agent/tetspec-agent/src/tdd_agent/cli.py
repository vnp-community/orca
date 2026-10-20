"""``tdd-agent`` command line.

Every command prints one JSON document on stdout (machine readable); human text
goes to stderr.  Exit codes: 0 success, 1 the run/operation ended blocked or was
rejected, 2 usage or configuration error.
"""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from .config import load_config
from .errors import TddAgentError
from .obs import render_tree
from .orchestrator import ApprovalRequest
from .runtime import Runtime
from .store.base import SourceDoc

EXIT_OK, EXIT_BLOCKED, EXIT_USAGE = 0, 1, 2


OUTPUT: dict[str, bool | None] = {"json": None}


def _json_mode() -> bool:
    """Machine output (JSON on stdout) when asked with --json or when stdout is not a terminal; readable text in a terminal (--human forces it)."""
    return OUTPUT["json"] if OUTPUT["json"] is not None else not sys.stdout.isatty()


def _emit(result: Any, lines: list[str]) -> None:
    """A project command's answer: text for a person, or the JSON with the text on stderr for a script."""
    if _json_mode():
        _print(result)
        _human(lines)
    else:
        for line in lines:
            print(line)


def _fail(exc: TddAgentError) -> None:
    if _json_mode():
        _print({"error": exc.as_dict()})
    else:
        print(f"Error: {exc}  [{exc.code}]", file=sys.stderr)


def _print(value: Any) -> None:
    json.dump(value, sys.stdout, indent=2, ensure_ascii=False, default=str)
    sys.stdout.write("\n")


def parse_source(spec: str) -> SourceDoc:
    """``ID@REVISION:ROLE=path`` (e.g. ``SRS@2:SRS=docs/srs-v2.md``)."""
    try:
        head, path = spec.split("=", 1)
        ident_rev, role = head.rsplit(":", 1)
        ident, revision = ident_rev.split("@", 1)
    except ValueError as exc:
        raise TddAgentError(f"source must look like ID@REVISION:ROLE=path, got {spec!r}", code="SOURCE_SPEC_INVALID") from exc
    return SourceDoc.make(ident, revision, role, Path(path).read_text(encoding="utf-8"))


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="tdd-agent", description="TDD agent runtime")
    p.add_argument("--config", help="path to a configuration file (default: the project's .tdd-agent/config.toml, else ./tdd-agent.toml)")
    p.add_argument("-C", "--project", help="project folder (default: the nearest folder above the current one that has .tdd-agent/config.toml)")
    p.add_argument("--set", action="append", default=[], metavar="SECTION.KEY=VALUE", help="override a configuration value (repeatable)")
    shown = p.add_mutually_exclusive_group()
    shown.add_argument("--json", action="store_true", help="print JSON on stdout (the default when stdout is not a terminal)")
    shown.add_argument("--human", action="store_true", help="print readable text even when stdout is not a terminal")
    sub = p.add_subparsers(dest="command", required=True)

    sub.add_parser("config", help="show the effective configuration (secrets masked)")
    sub.add_parser("doctor", help="check what the commands need (agent CLI, OpenSpec, documents, baseline) without calling a model")

    ini = sub.add_parser("init", help="set the project up and build its knowledge graph and TDD from the documents (URD/SRS/SAD)")
    ini.add_argument("--agent", choices=["claude", "codex"], help="the agent that reads and writes (default claude)")
    ini.add_argument("--approver", action="append", default=[], metavar="ID[:KIND[:ROLES]]", help="register an approver of the change-request gates, e.g. alice:human:BA,SA or claude:coding_agent (repeatable)")
    ini.add_argument("--no-generate", action="store_true", help="only create .tdd-agent/, do not build the knowledge graph and the TDD yet")
    ini.add_argument("--no-prose", action="store_true", help="do not read free text (no language model call); saved in the configuration")
    ini.add_argument("--prose-limit", type=int)
    ini.add_argument("--name"); ini.add_argument("--tdd-id"); ini.add_argument("--docs-dir", default="docs/tdd"); ini.add_argument("--force", action="store_true")
    for role in ("urd", "srs", "sad"):
        ini.add_argument(f"--{role}", help=f"the project's {role.upper()} document")
    gen = sub.add_parser("generate", help="refresh the knowledge graph and the TDD after the documents changed (init already does the first run)")
    for role in ("urd", "srs", "sad"):
        gen.add_argument(f"--{role}", help=f"{role.upper()} document (default: [project.sources] of the configuration)")
    gen.add_argument("--id", dest="tdd_id", help="TDD id (default: [project] tdd_id)"); gen.add_argument("--out", help="folder for the TDD (default: [project] docs_dir)")
    gen.add_argument("--no-prose", action="store_true", help="do not read free text (no language model call)"); gen.add_argument("--prose-limit", type=int)
    gen.add_argument("--force", action="store_true", help="run again even when the sources did not change")
    sub.add_parser("status", help="project, sources, knowledge graph and last TDD")

    ch = sub.add_parser("change", help="change request: edit the SRS, then impact -> change spec -> OpenSpec bundle")
    cs = ch.add_subparsers(dest="action", required=True)
    crun = cs.add_parser("run", help="the whole change in one command: `tdd-agent change <cr.md>` (checks, names, starts, and runs it as far as it can go; run it again to continue)")
    crun.add_argument("cr_file", metavar="cr.md"); crun.add_argument("--name", help="folder name of the change (default: made from the CR's title)")
    crun.add_argument("--owner", help="who owns the change (a team or a person)"); crun.add_argument("--purpose"); crun.add_argument("--scope")
    crun.add_argument("--acceptance", action="append", default=[], metavar="AC-ID"); crun.add_argument("--allow-draft", action="store_true")
    crun.add_argument("--code-target", action="append", default=[], metavar="REQ=path::symbol"); crun.add_argument("--verify", action="append", default=[], metavar="REQ=GIVEN…WHEN…THEN…")
    crun.add_argument("--satisfied-by", action="append", default=[], metavar="REQ=ELEMENT-ID")
    crun.add_argument("--yes", action="store_true", help="do not ask anything (also the case when there is no terminal)")
    cst = cs.add_parser("start", help="start a change request: the CR is read against the SRS and SAD the TDD came from, the impact on the TDDs is assessed, then the OpenSpec bundle is written")
    cst.add_argument("name", help="change name: lowercase words joined by '-' (also the OpenSpec change folder)")
    cst.add_argument("--cr", metavar="FILE", help="the change request as a Markdown file (title, `AC-<id>: …` acceptance criteria, optionally a `CR-<id>`)")
    cst.add_argument("--cr-text", metavar="TEXT", help="the change request typed on the command line")
    cst.add_argument("--title", help="title of the CR (default: the first line of the CR)"); cst.add_argument("--acceptance", action="append", default=[], metavar="AC-ID", help="acceptance criterion id the CR does not list itself (repeatable)")
    cst.add_argument("--cr-id"); cst.add_argument("--srs-after", metavar="FILE", help="an SRS you already edited by hand: the delta is read from it instead of being derived from the CR")
    cst.add_argument("--purpose"); cst.add_argument("--scope"); cst.add_argument("--owner", help="owner of the capability (not invented when omitted)")
    cst.add_argument("--code-target", action="append", default=[], metavar="REQ=path::symbol", help="code the requirement changes (otherwise proposed from the repository)")
    cst.add_argument("--verify", action="append", default=[], metavar="REQ=GIVEN…WHEN…THEN…", help="verification of a requirement (otherwise derived from its acceptance)")
    cst.add_argument("--new-in", action="append", default=[], metavar="REQ=TDD-ID", help="TDD that should receive a NEW requirement")
    cst.add_argument("--allow-draft", action="store_true", help="when the spec is not eligible, still write a DRAFT bundle for inspection (never published)")
    for action, text in (("status", "where the change is and what it waits for"), ("advance", "run the next gates"), ("decision", "read the decision of the waiting gate")):
        sp = cs.add_parser(action, help=text); sp.add_argument("run_id", nargs="?", help="default: the latest change")
        if action == "decision":
            sp.add_argument("--actor", required=True)
        if action == "status":
            sp.add_argument("--why", action="store_true", help="explain the blockers: the cause behind each and the command that settles it")
        if action == "advance":
            sp.add_argument("--retry", action="store_true", help="run again a change that failed on a program error (an interrupted change simply continues)")
    cup = cs.add_parser("update", help="give a running change what its gates found missing (code target, verification, TDD for a new requirement) and run on")
    cup.add_argument("run_id", nargs="?"); cup.add_argument("--code-target", action="append", default=[], metavar="REQ=path::symbol")
    cup.add_argument("--verify", action="append", default=[], metavar="REQ=GIVEN…WHEN…THEN…"); cup.add_argument("--new-in", action="append", default=[], metavar="REQ=TDD-ID")
    cup.add_argument("--satisfied-by", action="append", default=[], metavar="REQ=ELEMENT-ID", help="the existing component, operation or entity that satisfies a modified requirement")
    cup.add_argument("--owner"); cup.add_argument("--purpose"); cup.add_argument("--scope"); cup.add_argument("--acceptance", action="append", default=[], metavar="AC-ID", help="an acceptance criterion the CR does not list")
    cw = cs.add_parser("waive", help="the SA sets aside one finding of the SAD check (a SAD decision against one requirement), with the reason")
    cw.add_argument("run_id", nargs="?"); cw.add_argument("--sad-element", required=True); cw.add_argument("--requirement", required=True); cw.add_argument("--actor", required=True); cw.add_argument("--reason", required=True)
    cap = cs.add_parser("approve", help="approve the waiting gate (the role is taken from your approver entry)")
    cap.add_argument("run_id", nargs="?"); cap.add_argument("--actor", required=True); cap.add_argument("--rationale", required=True); cap.add_argument("--hash", required=True, dest="decision_hash")
    cap.add_argument("--role", help="the role to approve as (default: the role the gate asks for that your approver entry has)")
    cs.add_parser("list", help="the change requests of this project")

    bl = sub.add_parser("baseline", help="write the OpenSpec baseline of the requirements a change modifies or removes (the change flow also does it by itself)")
    bl.add_argument("run_id", nargs="?", help="default: the latest change"); bl.add_argument("--force", action="store_true", help="rewrite requirements already in the baseline")
    ed = sub.add_parser("edit", help="edit the TDD with a prompt (controlled: authorization, upstream values are never overridden, re-rendered, undoable)")
    ed.add_argument("prompt", nargs="?", help='for example: "set the timeout of API-I-01 to 25 seconds"')
    ed.add_argument("--scope", action="append", default=[], metavar="ID", help="elements the edit may touch (default: any)")
    ed.add_argument("--allow", action="append", default=[], choices=["modify", "add", "remove"], help="operations the edit may do (default: modify)")
    ed.add_argument("--user", default="user"); ed.add_argument("--undo", metavar="RUN_ID", help="undo the edit made by this run")

    run = sub.add_parser("run", help="start, advance and inspect runs")
    rs = run.add_subparsers(dest="action", required=True)
    st = rs.add_parser("start"); st.add_argument("flow"); st.add_argument("--request", required=True, help="JSON file with mode/params")
    st.add_argument("--source", action="append", default=[], help="ID@REVISION:ROLE=path"); st.add_argument("--run-id"); st.add_argument("--no-advance", action="store_true")
    for name in ("advance", "show", "trace"):
        sp = rs.add_parser(name); sp.add_argument("run_id")
    rs.choices["advance"].add_argument("--heartbeat", action="store_true", help="record pid and a heartbeat for remote pollers (set by run_dispatch)")
    ls = rs.add_parser("list"); ls.add_argument("--flow"); ls.add_argument("--status"); ls.add_argument("--limit", type=int, default=20)

    dec = sub.add_parser("decision", help="read the pending decision of a gate (records your review receipt)")
    dec.add_argument("run_id"); dec.add_argument("gate"); dec.add_argument("--actor", required=True)

    ap = sub.add_parser("approve", help="approve a pending gate (allowlisted actor, role, rationale and the decision hash you read)")
    ap.add_argument("run_id"); ap.add_argument("gate"); ap.add_argument("--actor", required=True); ap.add_argument("--role", required=True)
    ap.add_argument("--rationale", required=True); ap.add_argument("--hash", required=True, dest="decision_hash")

    rv = sub.add_parser("review", help="approve/return knowledge elements")
    rv.add_argument("--actor"); rv.add_argument("--role"); rv.add_argument("--rationale")
    rv.add_argument("--decision", default="approved", choices=["approved", "needs_review", "rejected"])
    g = rv.add_mutually_exclusive_group(required=True); g.add_argument("--ids", nargs="+"); g.add_argument("--all-pending", action="store_true"); g.add_argument("--list", action="store_true")

    rc = sub.add_parser("receipt", help="record a test receipt for a change-spec row")
    rc.add_argument("run_id"); rc.add_argument("cs_id"); rc.add_argument("--criteria", required=True, help="comma separated"); rc.add_argument("--result", required=True, choices=["pass", "fail"])
    rc.add_argument("--actor", required=True); rc.add_argument("--evidence")

    mt = sub.add_parser("metrics", help="operational metrics and LLM share per agent")
    mt.add_argument("--run", help="what one run cost: model calls, tokens, time, per skill")
    cl = sub.add_parser("clean", help="remove working copies no run needs any more (lists first; --yes removes)")
    cl.add_argument("--yes", action="store_true")

    sk = sub.add_parser("skills", help="skill catalogue, regression examples, demotion report")
    ss = sk.add_subparsers(dest="action", required=True)
    ss.add_parser("list")
    tt = ss.add_parser("test"); tt.add_argument("--ids", nargs="*"); tt.add_argument("--tiers", nargs="*", default=["logic", "hybrid", "llm"])
    ss.add_parser("demotion-report")

    mc = sub.add_parser("mcp", help="MCP server")
    ms = mc.add_subparsers(dest="action", required=True)
    sv = ms.add_parser("serve"); sv.add_argument("--transport", choices=["stdio", "http"])
    me = ms.add_parser("enable", help="register this project's knowledge graph as an MCP server (.mcp.json) and set what clients may do")
    me.add_argument("--read-only", action="store_true", help="expose only the read tools (default: read and write tools: run_start, run_resume, changeset_propose)")
    me.add_argument("--approvals", action="store_true", help="also expose approval_submit (needs --client-actor)")
    me.add_argument("--client-actor", default="", help="approver identity of the MCP client; must be in [[approvals.actors]]")
    ms.add_parser("disable", help="remove the MCP registration and switch the write/approval tools off")

    kg = sub.add_parser("kg", help="knowledge store utilities")
    ks = kg.add_subparsers(dest="action", required=True)
    ks.add_parser("stats"); ks.add_parser("snapshots")
    kse = ks.add_parser("search", help="full-text search of the knowledge graph (accents and capitals ignored; an id in full or in part comes first)")
    kse.add_argument("query"); kse.add_argument("--class", dest="classes", action="append", default=[]); kse.add_argument("--snapshot"); kse.add_argument("--limit", type=int, default=20)
    ks.add_parser("reindex", help="rebuild the full-text index from the elements (the graph itself is not touched)")
    ks.add_parser("index-check", help="does the full-text index hold exactly the elements?")
    kc = ks.add_parser("conflicts", help="conflicts the knowledge graph records (unresolved by default)"); kc.add_argument("--all", action="store_true", help="include resolved ones")
    kr = ks.add_parser("resolve", help="settle conflicts: who decided what and why is kept (SA or BA)"); kr.add_argument("keys", nargs="+")
    kr.add_argument("--actor", required=True); kr.add_argument("--role", required=True, choices=["SA", "BA"]); kr.add_argument("--decision", required=True); kr.add_argument("--rationale", required=True)
    im = ks.add_parser("import"); im.add_argument("graph"); im.add_argument("--no-validate", action="store_true"); im.add_argument("--actor", default="import")

    ac = sub.add_parser("acceptance", help="acceptance runs (real provider)")
    asub = ac.add_subparsers(dest="action", required=True)
    a1 = asub.add_parser("generation"); a1.add_argument("--out", required=True); a1.add_argument("--repetitions", type=int)
    a1.add_argument("--inputs", nargs="*", default=["pbom", "prose", "second"])
    a2 = asub.add_parser("isolated"); a2.add_argument("--out", required=True); a2.add_argument("--agents", nargs="*", default=["generation", "impact", "changespec", "promptedit"]); a2.add_argument("--repetitions", type=int)
    a3 = asub.add_parser("existing-tools"); a3.add_argument("--out", required=True)
    a4 = asub.add_parser("approve-oracle"); a4.add_argument("--out", required=True); a4.add_argument("--oracle", required=True); a4.add_argument("--actor", required=True)
    a4.add_argument("--role", required=True); a4.add_argument("--rationale", required=True)
    asub.add_parser("search", help="known-item queries over the PBOM graph: full-text index against the in-memory scan (recall@5, recall@20, MRR, time)").add_argument("--out", required=True)
    ap = asub.add_parser("providers", help="the regression examples of every skill that uses a model, on each agent (version, model, calls, tokens, time, failures)")
    ap.add_argument("--out", required=True); ap.add_argument("--agents", nargs="*", default=["claude", "codex"], choices=["claude", "codex"]); ap.add_argument("--tiers", nargs="*", default=["hybrid", "llm"])
    a5 = asub.add_parser("case-table"); a5.add_argument("--out", required=True)
    a6 = asub.add_parser("report"); a6.add_argument("--out", required=True)
    a7 = asub.add_parser("e2e-pbom"); a7.add_argument("--out", required=True); a7.add_argument("--allow-draft", action="store_true", help="stage a DRAFT OpenSpec bundle at G5 even though the spec is not eligible (inspection only; never published)")
    a8 = asub.add_parser("e2e-second"); a8.add_argument("--out", required=True)
    asub.add_parser("real-cases-report").add_argument("--out", required=True)
    a9 = asub.add_parser("real-cases"); a9.add_argument("--out", required=True); a9.add_argument("--only", nargs="*")
    return p


def _overrides(pairs: list[str]) -> dict:
    out: dict = {}
    for pair in pairs:
        key, _, raw = pair.partition("=")
        node = out
        parts = key.split(".")
        for part in parts[:-1]:
            node = node.setdefault(part, {})
        try:
            node[parts[-1]] = json.loads(raw)
        except json.JSONDecodeError:
            node[parts[-1]] = raw
    return out


def _run_summary(rt: Runtime, run_id: str) -> dict:
    view = rt.orchestrator.run_view(run_id)
    run = view["run"]
    return {"run_id": run["run_id"], "flow": run["flow"], "status": run["status"], "state": run["state"], "blockers": run["result"].get("blockers", []),
            "pending_gate": run["result"].get("pending_gate"), "blocked_gate": run["result"].get("blocked_gate"), "approvals": [{k: a[k] for k in ("gate", "actor_id", "approval_grade")} for a in view["approvals"]]}


def _human(lines: list[str]) -> None:
    for line in lines:
        print(line, file=sys.stderr)


def _init(args) -> int:
    """``init``: create the project when it does not exist, then build the knowledge graph and the TDD from its documents (the first ``generate``)."""
    from . import project
    root = Path(args.project).resolve() if args.project else Path.cwd()
    flags = {"urd": args.urd, "srs": args.srs, "sad": args.sad}
    result = project.init_project(root, name=args.name, tdd_id=args.tdd_id, docs_dir=args.docs_dir, force=args.force, sources=flags, approvers=args.approver, prose=not args.no_prose, agent=args.agent)
    lines = [f"Initialized {result['config']}." if result["created"] else f"Project already initialized ({result['config']} kept; --force rewrites it)."]
    if args.no_generate:
        result["generate"] = {"status": "skipped", "reason": "--no-generate"}
        _emit(result, lines + ["Next: `tdd-agent init` again (or `tdd-agent generate`) to build the knowledge graph and the TDD."])
        return EXIT_OK
    config = load_config(root / project.STATE_DIR / project.CONFIG_NAME, overrides=_overrides(args.set), base_dir=root)
    try:
        project.resolve_sources(config, flags)
    except TddAgentError as exc:
        if exc.code != "SOURCES_MISSING":
            raise
        result["generate"] = {"status": "skipped", "reason": "no source documents yet"}
        _emit(result, lines + ["No documents to read yet: pass --urd/--srs/--sad (or fill [project.sources] in the configuration) and run `tdd-agent init` again."])
        return EXIT_OK
    _, estimate = project.estimate_generate(config, flags, prose=False if args.no_prose else None, prose_limit=args.prose_limit)
    _human(lines + ["Reading the documents and building the knowledge graph…", estimate])
    rt = Runtime.build(config, configure_logging=True)
    try:
        generated, glines = project.generate(rt, flags, tdd_id=None, out=None, prose=False if args.no_prose else None, prose_limit=args.prose_limit, force=False)
    finally:
        rt.close()
    result["generate"] = generated
    _emit(result, glines)
    return EXIT_OK if generated["status"] in ("completed", "up_to_date") else EXIT_BLOCKED


CHANGE_ACTIONS = ("run", "start", "status", "advance", "decision", "update", "waive", "approve", "list", "-h", "--help")
_VALUE_OPTIONS = ("--config", "-C", "--project", "--set")


def _shorthand(argv: list[str]) -> list[str]:
    """``change <cr.md>`` is ``change run <cr.md>``; ``--json`` and ``--human`` are accepted after the command too."""
    flags = [a for a in argv if a in ("--json", "--human")]
    out, i = [a for a in argv if a not in ("--json", "--human")], 0
    while i < len(out):
        if out[i] in _VALUE_OPTIONS:
            i += 2
        elif out[i].startswith("-"):
            i += 1
        else:
            break
    if i + 1 < len(out) and out[i] == "change" and out[i + 1] not in CHANGE_ACTIONS and not out[i + 1].startswith("-"):
        out.insert(i + 1, "run")
    return flags[:1] + out


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(_shorthand(sys.argv[1:] if argv is None else argv))
    OUTPUT["json"] = True if args.json else (False if args.human else None)
    from . import project
    try:
        if args.command == "init":
            return _init(args)
        root = project.find_project_root(args.project or Path.cwd()) if not args.config else None
        needs_project = args.command in ("generate", "status", "change", "edit", "doctor", "baseline", "clean") or (args.command == "mcp" and args.action in ("enable", "disable"))
        if needs_project and root is None and not args.config:
            raise TddAgentError("no project here: run `tdd-agent init` in the project folder first", code="PROJECT_NOT_INITIALIZED")
        if args.command == "mcp" and args.action in ("enable", "disable"):
            if root is None:
                raise TddAgentError("MCP registration is per project: pass -C <project folder> or run it inside the project", code="PROJECT_NOT_INITIALIZED")
            if args.action == "enable":
                result = project.mcp_enable(root, write=not args.read_only, approvals=args.approvals, client_actor=args.client_actor)
                _emit(result, [f"MCP server registered in {result['mcp_file']} (tools: {result['tools']}).",
                        "Claude Code picks up .mcp.json in this project; for another client run: tdd-agent -C " + str(root) + " mcp serve"])
            else:
                result = project.mcp_disable(root)
                _emit(result, ["MCP registration removed; write and approval tools are off."])
            return EXIT_OK
        config = (load_config(root / project.STATE_DIR / project.CONFIG_NAME, overrides=_overrides(args.set), base_dir=root) if root
                  else load_config(args.config, overrides=_overrides(args.set)))
    except TddAgentError as exc:
        _fail(exc)
        return EXIT_USAGE
    if args.command == "doctor":
        from . import doctor
        checks = doctor.check(config, for_change=True)
        _emit({"checks": checks, "ok": not doctor.failures(checks)}, doctor.lines(checks))
        return EXIT_OK if not doctor.failures(checks) else EXIT_BLOCKED
    if args.command == "config":
        asked = config.get("approvals.mode")
        why = "set in the configuration" if asked else ("approvers are declared" if config.get("approvals.actors") else "nothing declared: the default")
        _print({"config": config.redacted(), "fingerprint": config.fingerprint(), "sources": config.source,
                "effective": {"agents": {role: (config.get(f"llm.roles.{role}") or config.get("llm.default")) for role in ("author", "critic", "extractor")},
                              "approvals_mode": config.approval_mode(), "approvals_mode_because": why, "approvals_required": config.get("approvals.required")}})
        return EXIT_OK
    try:
        return _dispatch(args, config)
    except TddAgentError as exc:
        _fail(exc)
        return EXIT_BLOCKED if exc.code in ("REVIEW_NOT_PERMITTED", "NO_PENDING_DECISION", "NOT_FOUND", "RUN_NOT_FOUND") else EXIT_USAGE


def _dispatch(args, config) -> int:
    if args.command == "mcp":
        from .mcp.server import serve
        rt = Runtime.build(config, configure_logging=True)
        serve(rt, args.transport)
        return EXIT_OK
    rt = Runtime.build(config, configure_logging=True)
    try:
        return _run_command(rt, args)
    finally:
        rt.close()


def _run_command(rt: Runtime, args) -> int:
    o = rt.orchestrator
    if args.command == "generate":
        from . import project
        from .agents.generation import ingested_hashes
        if not _json_mode() and not args.force:
            try:
                _human([project.estimate_generate(rt.config, {r: getattr(args, r) for r in ("urd", "srs", "sad")}, prose=False if args.no_prose else None, prose_limit=args.prose_limit,
                                                         ingested=ingested_hashes(rt.knowledge.materialize()))[1]])
            except TddAgentError:
                pass
        result, lines = project.generate(rt, {r: getattr(args, r) for r in ("urd", "srs", "sad")}, tdd_id=args.tdd_id, out=args.out, prose=False if args.no_prose else None,
                                         prose_limit=args.prose_limit, force=args.force)
        _emit(result, lines)
        return EXIT_OK if result["status"] in ("completed", "up_to_date") else EXIT_BLOCKED
    if args.command == "status":
        from . import project
        result, lines = project.status(rt)
        _emit(result, lines)
        return EXIT_OK
    if args.command == "baseline":
        from . import project
        result, lines = project.baseline(rt, args.run_id, args.force)
        _emit(result, lines)
        return EXIT_OK if not result["unwritable"] else EXIT_BLOCKED
    if args.command == "change":
        from . import project
        if args.action == "run":
            interactive = sys.stdin.isatty() and sys.stderr.isatty() and not args.yes
            ask = (lambda q: input(q)) if interactive else None
            if not _json_mode():
                print("Reading the change request; the model is called here, it can take several minutes…", file=sys.stderr)
            result, lines = project.change_run(rt, args.cr_file, name=args.name, owner=args.owner, purpose=args.purpose, scope=args.scope, acceptance=args.acceptance, allow_draft=args.allow_draft, ask=ask,
                                            code_targets=args.code_target, verifications=args.verify, satisfied_by=args.satisfied_by)
        elif args.action == "start":
            result, lines = project.change_start(rt, args.name, cr_file=args.cr, cr_text=args.cr_text, title=args.title, acceptance=args.acceptance, cr_id=args.cr_id, srs_after=args.srs_after, purpose=args.purpose, scope=args.scope,
                                                 owner=args.owner, code_targets=args.code_target, verifications=args.verify, new_in=args.new_in, allow_draft=args.allow_draft)
        elif args.action == "status":
            result, lines = project.change_status(rt, args.run_id, args.why)
        elif args.action == "advance":
            result, lines = project.change_advance(rt, args.run_id, args.retry)
        elif args.action == "update":
            result, lines = project.change_update(rt, args.run_id, args.code_target, args.verify, args.new_in, owner=args.owner, purpose=args.purpose, scope=args.scope, acceptance=args.acceptance, satisfied_by=args.satisfied_by)
        elif args.action == "waive":
            result, lines = project.change_waive(rt, args.run_id, args.sad_element, args.requirement, args.actor, args.reason)
        elif args.action == "decision":
            result, lines = project.change_decision(rt, args.run_id, args.actor)
        elif args.action == "approve":
            result, lines = project.change_approve(rt, args.run_id, args.actor, args.rationale, args.decision_hash, args.role)
        else:
            result, lines = project.change_list(rt)
        _emit(result, lines)
        return EXIT_BLOCKED if isinstance(result, dict) and (result.get("status") in ("blocked", "failed") or result.get("accepted") is False) else EXIT_OK
    if args.command == "edit":
        from . import project
        result, lines = project.edit(rt, args.prompt, scope=args.scope, operations=args.allow or ["modify"], user=args.user, undo=args.undo)
        _emit(result, lines)
        return EXIT_OK if result.get("status") in ("committed", "committed_pending_render", "presentation_committed", "undone") else EXIT_BLOCKED
    if args.command == "run":
        if args.action == "start":
            request = json.loads(Path(args.request).read_text(encoding="utf-8"))
            run = o.start(args.flow, request, run_id=args.run_id, sources=[parse_source(s) for s in args.source])
            if not args.no_advance and run.status == "running":
                o.advance(run.run_id)
            _print(_run_summary(rt, run.run_id))
            return EXIT_OK
        if args.action == "advance":
            if args.heartbeat:
                from . import project
                from .orchestrator.dispatch import Heartbeat
                with Heartbeat(rt.config.work_dir, args.run_id):
                    o.advance(args.run_id)
                    project.after_background_advance(rt, args.run_id)
            else:
                o.advance(args.run_id)
            summary = _run_summary(rt, args.run_id)
            _print(summary)
            return EXIT_BLOCKED if summary["status"] in ("blocked", "failed") else EXIT_OK
        if args.action == "show":
            _print(o.run_view(args.run_id))
            return EXIT_OK
        if args.action == "trace":
            run = rt.operational.get_run(args.run_id)
            spans = rt.operational.list_spans(trace_id=run.trace_id)
            print(render_tree(spans))
            _print({"run_id": args.run_id, "trace_id": run.trace_id, "spans": len(spans)})
            return EXIT_OK
        if args.action == "list":
            _print([{"run_id": r.run_id, "flow": r.flow, "status": r.status, "state": r.state, "created_at": r.created_at}
                    for r in rt.operational.list_runs(flow=args.flow, status=args.status, limit=args.limit)])
            return EXIT_OK
    if args.command == "decision":
        _print(o.get_decision(args.run_id, args.gate, args.actor, source="cli"))
        return EXIT_OK
    if args.command == "approve":
        res = o.submit_approval(ApprovalRequest(args.run_id, args.gate, args.actor, args.role, args.decision_hash, args.rationale, channel="cli"))
        _print({"accepted": res.accepted, "code": res.code, "message": res.message, "run_status": res.run_status})
        return EXIT_OK if res.accepted else EXIT_BLOCKED
    if args.command == "review":
        if args.list:
            _print(o.elements_needing_review())
            return EXIT_OK
        if not (args.actor and args.role and args.rationale):
            raise TddAgentError("review needs --actor, --role and --rationale", code="REVIEW_ARGUMENTS_MISSING")
        ids = args.ids if args.ids else [e["id"] for e in o.elements_needing_review()]
        _print(o.review_elements(ids, args.decision, args.actor, args.role, args.rationale))
        return EXIT_OK
    if args.command == "receipt":
        el = o.record_test_receipt(args.run_id, args.cs_id, [c for c in args.criteria.split(",") if c], args.result, args.actor, evidence_ref=args.evidence)
        _print({"receipt": el["id"], "cs": el["props"]["cs_id"], "result": el["props"]["result"]})
        return EXIT_OK
    if args.command == "clean":
        from . import project
        result, lines = project.clean(rt, args.yes)
        _emit(result, lines)
        return EXIT_OK
    if args.command == "metrics" and args.run:
        from . import project
        cost = project.run_cost(rt, args.run)
        _emit(cost, project.cost_lines(cost))
        return EXIT_OK
    if args.command == "metrics":
        _print({"metrics": rt.observer.metrics.summary(), "llm_share": rt.observer.metrics.llm_share()})
        return EXIT_OK
    if args.command == "skills":
        return _skills(rt, args)
    if args.command == "kg":
        if args.action == "stats":
            _print(rt.knowledge.stats())
        elif args.action == "snapshots":
            _print([s.as_dict() for s in rt.knowledge.snapshots()])
        elif args.action == "search":
            _print([h.as_dict() for h in rt.knowledge.search(args.query, args.snapshot, args.classes or None, args.limit)])
        elif args.action in ("reindex", "index-check"):
            if not hasattr(rt.knowledge, "reindex"):
                from .errors import SearchNotSupported
                raise SearchNotSupported("this knowledge store keeps no index on disk", code="SEARCH_NOT_SUPPORTED")
            _print(rt.knowledge.reindex() if args.action == "reindex" else rt.knowledge.index_check())
        elif args.action == "conflicts":
            _print(rt.orchestrator.list_conflicts(None if args.all else "unresolved"))
        elif args.action == "resolve":
            _print(rt.orchestrator.resolve_conflicts(args.keys, args.actor, args.role, args.decision, args.rationale))
        else:
            graph = json.loads(Path(args.graph).read_text(encoding="utf-8"))
            head = rt.knowledge.head()
            res = rt.knowledge.commit_graph(head.id if head else None, graph, actor=args.actor, validate=not args.no_validate)
            _print({"snapshot": res.snapshot.id, "hash": res.snapshot.hash, "elements": len(graph["elements"]), "edges": len(graph["edges"])})
        return EXIT_OK
    if args.command == "acceptance":
        from .acceptance import cli_entry
        return cli_entry(rt, args)
    return EXIT_USAGE


def _skills(rt: Runtime, args) -> int:
    from .agents.common import registry
    from .skills import SkillRunner
    from .skills.demotion import demotion_report
    from .skills.examples import run_examples
    if args.action == "list":
        _print([{"id": s.meta.id, "tier": s.meta.tier, "agent": s.meta.agent, "version": s.meta.version, "package_hash": s.package_hash[:12], "reason": s.meta.reason or None, "demotion": s.meta.demotion or None}
                for s in registry().all()])
        return EXIT_OK
    if args.action == "test":
        runner = SkillRunner(registry(), rt.observer, rt.gateway)
        results = run_examples(registry(), runner, args.ids or None, tuple(args.tiers))
        _print({"passed": sum(r.passed for r in results), "failed": sum(not r.passed for r in results), "provider": rt.gateway.describe(),
                "results": [{"skill": r.skill, "example": r.name, "passed": r.passed, "resolved_by": r.resolved_by, "errors": r.errors} for r in results]})
        return EXIT_OK if all(r.passed for r in results) else EXIT_BLOCKED
    _print(demotion_report(rt.operational))
    return EXIT_OK


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
