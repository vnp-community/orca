"""tsa generate | check | mcp serve | run-job. Result JSON on stdout, progress on stderr; exit 0 ok, 1 some files need review/failed, 2 error."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

from . import conformance, log, tcir
from .errors import TsaError
from .generators import make
from .pipeline import Options, run


def _source(a):
    if bool(a.tcir_file) == bool(a.mcp_command):
        raise TsaError("give exactly one of --tcir-file or --mcp-command", code="USAGE")
    if a.tcir_file:
        return tcir.load_file(a.tcir_file)
    return tcir.McpSource.from_command_line(a.mcp_command, a.mcp_tool, a.mcp_cwd)


def cmd_generate(a) -> int:
    src = _source(a)
    opts = Options(repo=Path(a.repo), base=a.base, feature=a.feature, level=a.level, branch=a.branch,
                   worktree_path=Path(a.worktree) if a.worktree else None, reuse=a.reuse, max_fix=a.max_fix,
                   only_approved=a.only_approved, only=set(a.case) if a.case else None, commit=not a.no_commit)
    out = run(opts, src, make(a.generator, a.model))
    print(json.dumps(out.as_dict(), ensure_ascii=False, indent=1))
    return 0 if all(f.status == "generated" for f in out.files) else 1


def cmd_check(a) -> int:
    bundle = tcir.load_file(a.tcir_file)
    cases, _ = tcir.select(bundle)
    root = Path(a.worktree)
    files = a.file or sorted(str(p.relative_to(root)) for p in root.rglob("test_tsa_*.py"))
    scans = [conformance.scan((root / f).read_text(encoding="utf-8"), f) for f in files]
    marked = {cid for s in scans for cs in s.functions.values() for cid in cs}
    findings, manifest = conformance.check([c for c in cases if c["id"] in marked] if a.marked_only else cases, scans)
    print(json.dumps({"files": files, "findings": conformance.as_dicts(findings), "manifest": manifest}, ensure_ascii=False, indent=1))
    return 1 if any(f.severity == conformance.ERROR for f in findings) else 0


def cmd_mcp(a) -> int:
    from .mcp_server import serve
    serve(Path(a.C or "."))
    return 0


def cmd_run_job(a) -> int:
    from .runner import run_job
    return run_job(Path(a.project_dir), a.job)


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="tsa", description="Test Script Agent: spec-anchored pytest scripts from TCIR")
    p.add_argument("-q", "--quiet", action="store_true")
    p.add_argument("-C", metavar="DIR", help="project directory (mcp serve)")
    sub = p.add_subparsers(dest="cmd", required=True)
    g = sub.add_parser("generate", help="generate test scripts into a dedicated worktree and commit them")
    g.add_argument("--feature", required=True)
    g.add_argument("--repo", required=True, help="the test repository (never the application repository)")
    g.add_argument("--base", default="HEAD", help="branch or commit the worktree starts from")
    g.add_argument("--tcir-file", help="TCIR JSON (bundle from get_tcir/testspec_tcir_get)")
    g.add_argument("--mcp-command", help="stdio MCP server serving TCIR, e.g. 'tdd-agent -C /project mcp serve' or 'tsagent serve --stdio'")
    g.add_argument("--mcp-tool", choices=tcir.MCP_TOOLS, help="default: get_tcir for tsagent, testspec_tcir_get otherwise")
    g.add_argument("--mcp-cwd")
    g.add_argument("--level", choices=["L1", "L2", "L3"])
    g.add_argument("--case", action="append", help="only this test case id (repeatable)")
    g.add_argument("--only-approved", action="store_true", help="skip cases whose review status is not approved")
    g.add_argument("--generator", choices=["claude", "template"], default="claude")
    g.add_argument("--model")
    g.add_argument("--max-fix", type=int, default=3)
    g.add_argument("--branch", help="default tsa/<feature>-v<specVersion>")
    g.add_argument("--worktree", help="default <repo>/../<repo>.tsa-worktrees/<branch>")
    g.add_argument("--reuse", action="store_true", help="continue in an existing worktree/branch of the same name")
    g.add_argument("--no-commit", action="store_true")
    g.set_defaults(fn=cmd_generate)
    c = sub.add_parser("check", help="run the spec conformance check on generated files")
    c.add_argument("--tcir-file", required=True)
    c.add_argument("--worktree", required=True)
    c.add_argument("--file", action="append", help="file relative to the worktree (default: every test_tsa_*.py)")
    c.add_argument("--marked-only", action="store_true", help="only check cases that have a marked test function")
    c.set_defaults(fn=cmd_check)
    m = sub.add_parser("mcp", help="MCP server for the Dev Server Agent bridge")
    msub = m.add_subparsers(dest="mcp_cmd", required=True)
    ms = msub.add_parser("serve", help="serve tsa_start, tsa_get, tsa_report, tsa_file, tsa_cancel")
    ms.add_argument("--transport", choices=["stdio"], default="stdio")
    ms.set_defaults(fn=cmd_mcp)
    r = sub.add_parser("run-job", help=argparse.SUPPRESS)
    r.add_argument("--project-dir", required=True)
    r.add_argument("--job", required=True)
    r.set_defaults(fn=cmd_run_job)
    return p


def main(argv: list[str] | None = None) -> int:
    a = parser().parse_args(argv)
    log.QUIET = a.quiet
    try:
        return a.fn(a)
    except TsaError as exc:
        print(json.dumps({"error": {"code": exc.code, "message": str(exc)}}, ensure_ascii=False), file=sys.stderr)
        return 2



if __name__ == "__main__":
    sys.exit(main())
