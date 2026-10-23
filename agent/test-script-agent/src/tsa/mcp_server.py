"""`tsa mcp serve`: the MCP server the Dev Server Agent bridges to (server name test-script-agent in its mcp.callTool allowlist).

Tools: tsa_start (write), tsa_get, tsa_report, tsa_file, tsa_cancel (write). The project directory is the server's working directory (the
bridge starts one server per project). Results are JSON; errors are {error: {code, message}} with isError set, the shape the bridge
and backend-go/common/tddagent read (docs/integrate/TDD-REQ-TESTSCRIPT-001 section 5.4).
"""
from __future__ import annotations

import inspect
import json
from pathlib import Path
from typing import Any

from . import __version__, jobs
from .errors import TsaError

INSTRUCTIONS = ("Test Script Agent: generates spec-anchored pytest scripts from the TCIR of a feature into a dedicated git worktree. "
                "tsa_start returns at once with a job id; poll tsa_get until state is done, error or cancelled.")
EXTERNAL_KEYS = ("system", "executionId", "stepId")


def _result(payload: dict, is_error: bool = False):
    from mcp_types import CallToolResult, TextContent
    text = json.dumps(payload, ensure_ascii=False, default=str)
    return CallToolResult(content=[TextContent(type="text", text=text)], structured_content=payload, is_error=is_error)


def _error(code: str, message: str):
    return _result({"error": {"code": code, "message": message[:500]}}, is_error=True)


def _guard(fn):
    def wrapped(*args, **kwargs):
        try:
            return _result(fn(*args, **kwargs))
        except TsaError as exc:
            return _error(exc.code, str(exc))
        except Exception as exc:   # never a traceback over the wire
            return _error("TSA_INTERNAL", f"{type(exc).__name__}: {exc}")
    return wrapped


class Tools:
    def __init__(self, project_dir: Path, spawn: bool = True):
        self.project_dir = Path(project_dir).resolve()
        self.spawn = spawn

    def tsa_start(self, feature: str, external: dict[str, Any], spec_version: int | None = None, branch: str | None = None,
                  base: str | None = None, tests_paths: list[str] | None = None, test_repo: str | None = None,
                  generator: str | None = None, model: str | None = None, max_fix: int | None = None,
                  only_approved: bool | None = None) -> dict:
        """Start generating the test scripts of `feature` in the background (idempotent on external + feature + spec_version)."""
        if not feature:
            raise TsaError("feature is required", code="INVALID_ARGUMENT")
        ext = {k: str(external.get(k, "")) for k in EXTERNAL_KEYS} if isinstance(external, dict) else {}
        # A caller's re-run carries a new attempt so it regenerates; absent, the key stays what older callers sent.
        if isinstance(external, dict) and external.get("attempt") not in (None, ""):
            ext["attempt"] = str(external["attempt"])
        if not ext.get("system") or not ext.get("executionId"):
            raise TsaError("external.system and external.executionId are required", code="INVALID_ARGUMENT")
        if generator not in (None, "claude", "template"):
            raise TsaError("generator must be claude or template", code="INVALID_ARGUMENT")
        params = {"feature": feature, "spec_version": spec_version, "branch": branch, "base": base, "tests_paths": tests_paths,
                  "test_repo": test_repo, "generator": generator, "model": model, "max_fix": max_fix, "only_approved": only_approved,
                  "external": ext}
        job, reused = jobs.start(self.project_dir, {k: v for k, v in params.items() if v is not None}, spawn=self.spawn)
        return {**jobs.view(job), "reused": reused}

    def tsa_get(self, job_id: str) -> dict:
        """State of a job: queued, running, done, error or cancelled; when done the branch, commit, counts and report path."""
        return jobs.view(jobs.read(self.project_dir, job_id))

    def tsa_report(self, job_id: str, offset: int = 0) -> dict:
        """The tsa-report.md of a finished job, cut by characters: repeat with offset=next."""
        return jobs.report(self.project_dir, job_id, max(offset, 0))

    def tsa_file(self, job_id: str, path: str, offset: int = 0) -> dict:
        """One output of a finished job (a generated script, assertion-manifest.json or tsa-report.md), cut by characters:
        repeat with offset=next. Only the files the job wrote can be read."""
        return jobs.read_output(self.project_dir, job_id, path, max(offset, 0))

    def tsa_cancel(self, job_id: str, reason: str = "") -> dict:
        """Stop a queued or running job; the worktree and the branch are kept."""
        return jobs.view(jobs.cancel(self.project_dir, job_id, reason))


def build_server(project_dir: Path, spawn: bool = True):
    from mcp.server.mcpserver import MCPServer
    from mcp_types import ToolAnnotations
    tools = Tools(project_dir, spawn)
    server = MCPServer("test-script-agent", instructions=INSTRUCTIONS, version=__version__)
    for name, read_only in (("tsa_start", False), ("tsa_get", True), ("tsa_report", True), ("tsa_file", True), ("tsa_cancel", False)):
        method = getattr(tools, name)
        fn = _guard(method)
        fn.__name__, fn.__doc__ = name, method.__doc__
        fn.__signature__ = inspect.signature(method)
        fn.__annotations__ = method.__annotations__
        server.add_tool(fn, name=name, description=method.__doc__,
                        annotations=ToolAnnotations(read_only_hint=read_only, destructive_hint=name == "tsa_cancel", open_world_hint=False),
                        structured_output=False)
    return server


def serve(project_dir: Path) -> None:
    build_server(project_dir).run("stdio")
