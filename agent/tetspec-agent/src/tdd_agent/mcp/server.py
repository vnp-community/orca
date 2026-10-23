"""MCP server over the runtime's stores, retrieval and orchestrator.

Tool registration is decided once from configuration: write tools exist only with
``[mcp] write = true`` and ``approval_submit`` only with ``[mcp] approvals = true``
*and* a client identity that is on the ``[approvals.actors]`` allowlist.  Every call
is traced (span kind ``tool``), logged, size-limited and its errors are returned as
structured results, never as tracebacks.
"""
from __future__ import annotations

import functools
import inspect
import json
import logging
import time
from dataclasses import dataclass
from typing import Any, Callable, Mapping

from mcp.server.mcpserver import MCPServer
from mcp.server.mcpserver.exceptions import ResourceError, ResourceNotFoundError, ToolError
from mcp_types import CallToolResult, TextContent, ToolAnnotations
from pydantic import ValidationError

from .. import __version__
from ..config import Config
from ..errors import ConfigError, TddAgentError
from ..runtime import Runtime
from .auth import BearerAuthMiddleware, TokenAuth
from .tools import APPROVAL_TOOL, WRITE_TOOLS, McpPolicy, McpTools, Paged, dumps, paginate

READ_TOOLS = ("kg_get_element", "kg_elements", "kg_neighbors", "kg_resolve_scope", "kg_query", "kg_search", "kg_snapshots", "source_get", "impact_get",
              "readiness_get", "run_list", "run_get", "trace_get", "metrics_get", "testspec_get",
              "testspec_tcir_get")
TRANSPORTS = ("stdio", "http")
ARG_TEXT = 200
ARG_ITEMS = 10
ARG_KEYS = 20

INSTRUCTIONS = ("Read-only access to the TDD knowledge graph, evidence closures, runs and traces. Every result names the snapshot it was "
                "computed on and sets truncated/next when it was cut to the size limit: repeat the call with offset=next for the rest. "
                "Errors are returned as {error: {code, message, details}}.")


def abridge(value: Any, depth: int = 0) -> Any:
    """Shorten call arguments for logs and spans; document text is reduced to its length."""
    if isinstance(value, str):
        return value if len(value) <= ARG_TEXT else f"{value[:ARG_TEXT]}...(+{len(value) - ARG_TEXT} chars)"
    if isinstance(value, (list, tuple)):
        out = [abridge(v, depth + 1) for v in value[:ARG_ITEMS]]
        return out + [f"...(+{len(value) - ARG_ITEMS} items)"] if len(value) > ARG_ITEMS else out
    if isinstance(value, Mapping):
        if depth >= 2:
            return {"...": f"{len(value)} keys"}
        out = {}
        for key, item in list(value.items())[:ARG_KEYS]:
            out[str(key)] = f"<{len(item)} chars>" if key == "text" and isinstance(item, str) else abridge(item, depth + 1)
        if len(value) > ARG_KEYS:
            out["..."] = f"+{len(value) - ARG_KEYS} keys"
        return out
    return value


@dataclass
class Outcome:
    payload: dict
    is_error: bool


class ToolRunner:
    """Runs one tool or resource call: correlation ids, span, log event, error mapping, size limit."""

    def __init__(self, runtime: Runtime, tools: McpTools):
        self.rt, self.tools, self.policy = runtime, tools, tools.policy

    # ------------------------------------------------------------ shaping
    def _shape(self, raw: Any) -> dict:
        limit = self.policy.max_chars
        if isinstance(raw, Paged):
            page = paginate(raw.result, raw.parts, limit=limit, offset=raw.offset, text=raw.text, pinned=raw.pinned, derive=raw.derive,
                            base=raw.base)
            if raw.after:
                raw.after(page)
            return page
        page = dict(raw, truncated=False, next=None)
        if len(dumps(page)) > limit:
            page["oversize"] = True
        return page

    @staticmethod
    def _error(exc: BaseException) -> dict:
        if isinstance(exc, TddAgentError):
            return exc.as_dict()
        if isinstance(exc, ValueError):
            return {"code": "INVALID_ARGUMENT", "message": str(exc), "details": {}}
        return {"code": "INTERNAL_ERROR", "message": f"internal error ({type(exc).__name__}); see the server log", "details": {}}

    def _correlation(self, run_id: Any) -> dict:
        """Bind the run's ids so the call is logged and traced under the run it concerns."""
        if not isinstance(run_id, str) or not run_id:
            return {}
        try:
            run = self.rt.operational.get_run(run_id)
        except TddAgentError:
            return {"run_id": run_id}
        return {"run_id": run_id, "trace_id": run.trace_id or None} if run.trace_id else {"run_id": run_id}

    # ------------------------------------------------------------ execution
    def execute(self, name: str, fn: Callable[..., Any], kwargs: dict) -> Outcome:
        obs = self.rt.observer
        started = time.monotonic()
        run_id = kwargs.get("run_id")
        arguments = obs.redactor.data(abridge({k: v for k, v in kwargs.items() if v is not None}))
        with obs.bind(**self._correlation(run_id)):
            with obs.span(f"mcp:{name}", "tool", tool=name, actor=self.policy.actor_label, transport=self.policy.transport,
                          arguments=arguments) as span:
                error: dict | None = None
                payload: dict = {}
                try:
                    payload = self._shape(fn(**kwargs))
                except Exception as exc:
                    error = self._error(exc)
                    if error["code"] == "INTERNAL_ERROR":
                        obs.event("mcp_tool_exception", logging.ERROR, tool=name, error_type=type(exc).__name__, error=str(exc)[:300])
                snapshot = self._snapshot(kwargs, payload)
                if error:
                    span.fail(error["code"])
                    payload = {"snapshot": snapshot, "error": error}
                else:
                    payload.setdefault("snapshot", snapshot)
                payload_run = payload.get("run_id") or (run_id if isinstance(run_id, str) else None)
                size = len(dumps(payload))
                duration_ms = int((time.monotonic() - started) * 1000)
                span.set(snapshot=snapshot, result_chars=size, truncated=bool(payload.get("truncated")), duration_ms=duration_ms,
                         **({"run_id": payload_run} if payload_run else {}))
                obs.event("mcp_tool_call", logging.WARNING if error else logging.INFO, tool=name, actor=self.policy.actor_label,
                          transport=self.policy.transport, snapshot=snapshot, status="error" if error else "ok", duration_ms=duration_ms,
                          error_code=error["code"] if error else None, truncated=bool(payload.get("truncated")), result_chars=size,
                          arguments=arguments, **({"run_id": payload_run} if payload_run else {}))
        return Outcome(payload, error is not None)

    def _snapshot(self, kwargs: dict, payload: dict) -> str | None:
        explicit = kwargs.get("snapshot")
        if isinstance(explicit, str):
            return explicit
        if payload.get("snapshot") is not None:
            return payload["snapshot"]
        try:
            return self.tools.snapshot_used(None)
        except Exception:
            return None

    def deny(self, name: str, kwargs: dict, code: str, message: str, reason: str, **details: Any) -> CallToolResult:
        """Record and answer a call that never reached a tool (unknown tool, disabled write, bad arguments)."""
        def refuse(**_: Any) -> dict:
            raise TddAgentError(message, code=code, details={"tool": name, **details})

        self.rt.observer.event("mcp_tool_denied", logging.WARNING, tool=name, actor=self.policy.actor_label, reason=reason, code=code)
        return self.result(self.execute(name, refuse, kwargs))

    @staticmethod
    def result(outcome: Outcome) -> CallToolResult:
        text = dumps(outcome.payload)
        return CallToolResult(content=[TextContent(type="text", text=text)], structured_content=json.loads(text), is_error=outcome.is_error)

    def call_tool(self, name: str, fn: Callable[..., Any], kwargs: dict) -> CallToolResult:
        return self.result(self.execute(name, fn, kwargs))

    def read_resource(self, name: str, fn: Callable[..., Any], kwargs: dict) -> str:
        outcome = self.execute(name, fn, kwargs)
        text = dumps(outcome.payload)
        if outcome.is_error:
            err = outcome.payload["error"]
            raise (ResourceNotFoundError if err["code"].endswith("NOT_FOUND") else ResourceError)(text)
        return text


class TddMCPServer(MCPServer):
    """MCPServer whose protocol-level failures (unknown tool, bad arguments) are structured and logged too."""

    runner: ToolRunner

    async def call_tool(self, name: str, arguments: dict[str, Any], ctx: Any = None) -> Any:
        try:
            return await super().call_tool(name, arguments, ctx)
        except ToolError as exc:
            if self._tool_manager.get_tool(name) is not None:
                cause = exc.__cause__
                problems = [{"field": ".".join(str(p) for p in e["loc"]), "problem": e["msg"]} for e in cause.errors()] \
                    if isinstance(cause, ValidationError) else [{"problem": str(exc)[:300]}]
                message = "invalid arguments: " + "; ".join(f"{p.get('field', '')} {p['problem']}".strip() for p in problems)
                return self.runner.deny(name, arguments, "INVALID_ARGUMENTS", message[:500], "invalid_arguments", problems=problems)
            if name in WRITE_TOOLS:
                return self.runner.deny(name, arguments, "WRITE_DISABLED", "write tools are disabled; set [mcp] write = true", "write_disabled")
            reason = "approvals_unavailable" if name == APPROVAL_TOOL else "unknown_tool"
            return self.runner.deny(name, arguments, "TOOL_NOT_FOUND", f"unknown tool: {name}", reason)


def _register(server: MCPServer, runner: ToolRunner, tools: McpTools, name: str, *, read_only: bool, destructive: bool = False) -> None:
    method = getattr(tools, name)

    @functools.wraps(method)
    def tool(**kwargs: Any) -> CallToolResult:
        return runner.call_tool(name, method, kwargs)

    server.add_tool(tool, name=name, description=inspect.getdoc(method),
                    annotations=ToolAnnotations(read_only_hint=read_only, destructive_hint=destructive, open_world_hint=False),
                    structured_output=False)


def build_server(runtime: Runtime, *, client_actor: str | None = None, transport: str | None = None) -> MCPServer:
    """Build the MCP server for ``runtime``.

    ``client_actor`` overrides ``[mcp] client_actor``; it is the identity every call is made as
    (and the only one ``approval_submit`` can use).  The caller of a tool never chooses it.
    """
    policy = McpPolicy.from_runtime(runtime, client_actor, transport)
    tools = McpTools(runtime, policy)
    runner = ToolRunner(runtime, tools)
    server = TddMCPServer("tdd-agent", instructions=INSTRUCTIONS, version=__version__)
    server.runner = runner
    for name in READ_TOOLS:
        _register(server, runner, tools, name, read_only=True)
    if policy.write:
        _register(server, runner, tools, "run_start", read_only=False)
        _register(server, runner, tools, "run_resume", read_only=False)
        _register(server, runner, tools, "run_dispatch", read_only=False)
        _register(server, runner, tools, "run_cancel", read_only=False, destructive=True)
        _register(server, runner, tools, "generate_start", read_only=False)
        _register(server, runner, tools, "change_start", read_only=False)
        _register(server, runner, tools, "testspec_start", read_only=False)
        _register(server, runner, tools, "changeset_propose", read_only=False)
        _register(server, runner, tools, "proposals_list", read_only=True)
    if policy.approvals_requested and not policy.approvals:
        runtime.observer.event("mcp_approvals_unavailable", logging.WARNING, actor=policy.actor_label,
                               reason="client identity is not on the [approvals.actors] allowlist")
    if policy.approvals:
        _register(server, runner, tools, APPROVAL_TOOL, read_only=False, destructive=True)

    @server.resource("tdd://snapshot/{id}", name="snapshot", description="Snapshot metadata and statistics", mime_type="application/json")
    def snapshot_resource(id: str) -> str:
        return runner.read_resource("resource_snapshot", tools.resource_snapshot, {"id": id})

    @server.resource("tdd://source/{id}", name="source", description="Latest revision of a pinned source document", mime_type="application/json")
    def source_resource(id: str) -> str:
        return runner.read_resource("resource_source", tools.resource_source, {"id": id})

    @server.resource("tdd://run/{id}", name="run", description="A run with its gate ledger, transitions and approvals", mime_type="application/json")
    def run_resource(id: str) -> str:
        return runner.read_resource("resource_run", tools.resource_run, {"id": id})

    return server


def build_http_app(runtime: Runtime, server: MCPServer | None = None, auth: TokenAuth | None = None, env: Mapping[str, str] | None = None):
    """ASGI app for the streamable HTTP transport; every request needs the configured bearer token."""
    cfg = runtime.config.section("mcp")
    auth = auth or TokenAuth.from_config(runtime.config, env)
    server = server or build_server(runtime, client_actor=auth.actor or None, transport="http")
    return BearerAuthMiddleware(server.streamable_http_app(host=cfg["host"]), auth, runtime.observer)


def serve(runtime: Runtime, transport: str | None = None) -> None:
    """Run the server on stdio (default) or streamable HTTP (host/port from ``[mcp]``, token required)."""
    config: Config = runtime.config
    mode = transport or config.get("mcp.transport")
    if mode not in TRANSPORTS:
        raise ConfigError(f"unknown MCP transport {mode!r}; expected one of {TRANSPORTS}")
    if mode == "stdio":
        build_server(runtime, transport="stdio").run("stdio")
        return
    import uvicorn
    app = build_http_app(runtime)
    uvicorn.run(app, host=config.get("mcp.host"), port=int(config.get("mcp.port")), log_level=str(config.get("obs.log_level")).lower())
