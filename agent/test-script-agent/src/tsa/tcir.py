"""TCIR sources (a JSON file, or the MCP of tsagent/tdd-agent) and the checks a test case must pass before scripts are generated."""
from __future__ import annotations

import json
import shlex
from collections.abc import Awaitable, Callable
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any

from .errors import SourceError, TcirError

SCHEMA_VERSION = "tcir/v1"
# testspec_tcir_get lives on the tdd-agent server, get_tcir on the tsagent gateway; both page with `next`.
MCP_TOOLS = ("testspec_tcir_get", "get_tcir")


@dataclass
class Bundle:
    feature: str
    spec_version: int
    status: str | None
    cases: list[dict]
    baseline: str | None = None

    @classmethod
    def from_json(cls, data: Any) -> Bundle:
        if isinstance(data, dict) and "testCase" in data:   # one case from testspec_tcir_get(code=...)
            data = data["testCase"]
        if isinstance(data, dict) and data.get("id") and "expected" in data:
            data = {"schemaVersion": data.get("schemaVersion"), "feature": data.get("feature"), "specVersion": data.get("specVersion"),
                    "testCases": [data]}
        if not isinstance(data, dict) or not isinstance(data.get("testCases"), list):
            raise TcirError("not a TCIR bundle: expected an object with testCases")
        if data.get("schemaVersion") != SCHEMA_VERSION:
            raise TcirError(f"unsupported TCIR schemaVersion {data.get('schemaVersion')!r}; expected {SCHEMA_VERSION}")
        if not data.get("feature") or not isinstance(data.get("specVersion"), int):
            raise TcirError("TCIR bundle needs feature and an integer specVersion")
        return cls(data["feature"], data["specVersion"], data.get("status"), list(data["testCases"]), data.get("baseline"))


def load_file(path: str | Path) -> Bundle:
    try:
        return Bundle.from_json(json.loads(Path(path).read_text(encoding="utf-8")))
    except (OSError, ValueError) as exc:
        raise SourceError(f"cannot read TCIR file {path}: {exc}") from None


CallFn = Callable[[str, dict], Awaitable[dict]]


# get_tcir pages by count and its server cuts long text, so ask for few cases per page; testspec_tcir_get pages by size itself.
PAGE_LIMIT = {"get_tcir": 5}


async def collect_pages(call: CallFn, tool: str, feature: str, level: str | None = None) -> dict:
    """Call the TCIR tool page by page (offset=next) and return one bundle with every test case."""
    args: dict[str, Any] = {"feature": feature}
    if level:
        args["level"] = level
    if tool in PAGE_LIMIT:
        args["limit"] = PAGE_LIMIT[tool]
    head, cases, offset = None, [], 0
    for _ in range(10_000):
        page = await call(tool, {**args, "offset": offset} if offset else args)
        if "error" in page:
            err = page["error"]
            raise SourceError(f"{tool}: {err.get('code')}: {err.get('message')}" if isinstance(err, dict) else f"{tool}: {err}")
        cases.extend(page.get("testCases") or [])
        head = head or page
        nxt = page.get("next")
        if nxt is None:
            break
        offset = nxt
    return {**{k: v for k, v in (head or {}).items() if k not in ("testCases", "next", "truncated", "page", "total", "offset")},
            "testCases": cases}


@dataclass
class McpSource:
    """Reads TCIR from an MCP server started over stdio, e.g. `tdd-agent -C <project> mcp serve` or `tsagent serve --stdio`."""
    command: list[str]
    tool: str = "testspec_tcir_get"
    cwd: str | None = None
    env: dict[str, str] | None = field(default=None)

    @classmethod
    def from_command_line(cls, command: str, tool: str | None = None, cwd: str | None = None) -> McpSource:
        parts = shlex.split(command)
        if not parts:
            raise SourceError("empty MCP command")
        tool = tool or ("get_tcir" if Path(parts[0]).name == "tsagent" else "testspec_tcir_get")
        if tool not in MCP_TOOLS:
            raise SourceError(f"unknown TCIR tool {tool!r}; expected one of {MCP_TOOLS}")
        return cls(parts, tool, cwd)

    def load(self, feature: str, level: str | None = None) -> Bundle:
        import anyio
        return Bundle.from_json(anyio.run(self._load, feature, level))

    async def _load(self, feature: str, level: str | None) -> dict:
        from mcp import Client, StdioServerParameters
        params = StdioServerParameters(command=self.command[0], args=self.command[1:], cwd=self.cwd, env=self.env)
        try:
            async with Client(params) as client:
                async def call(name: str, args: dict) -> dict:
                    res = await client.call_tool(name, args)
                    text = res.content[0].text if res.content else ""
                    if getattr(res, "is_error", None) or getattr(res, "isError", False):
                        raise SourceError(f"{name}: {text[:500]}")
                    try:
                        return json.loads(text)
                    except ValueError:
                        raise SourceError(f"{name} returned non-JSON: {text[:200]}") from None
                return await collect_pages(call, self.tool, feature, level)
        except SourceError:
            raise
        except Exception as exc:   # spawn failures, protocol errors
            leaf = _leaf(exc)
            if isinstance(leaf, SourceError):
                raise leaf from None
            raise SourceError(f"MCP server {' '.join(self.command)!r} failed: {type(leaf).__name__}: {leaf}") from None


def _leaf(exc: BaseException) -> BaseException:
    """The first real error inside anyio's exception groups."""
    while isinstance(exc, BaseExceptionGroup) and exc.exceptions:
        exc = exc.exceptions[0]
    return exc


@dataclass
class Skipped:
    case: str
    reason: str


def select(bundle: Bundle, only_approved: bool = False, only: set[str] | None = None) -> tuple[list[dict], list[Skipped]]:
    """Cases a script can be generated for, and the others with the reason. Never guesses a missing expected result."""
    ok, skipped, seen = [], [], set()
    for c in bundle.cases:
        cid = c.get("id")
        if only is not None and cid not in only:
            continue
        reason = _problem(c, seen)
        if reason is None and not c.get("automatable", True):
            reason = "test case is not automatable (verificationMethod inspection)"
        if reason is None and only_approved and (c.get("review") or {}).get("status") != "approved":
            reason = f"review status is {(c.get('review') or {}).get('status')!r}, only approved cases were requested"
        if reason:
            skipped.append(Skipped(str(cid), reason))
        else:
            ok.append(c)
        seen.add(cid)
    return ok, skipped


def _problem(c: dict, seen: set) -> str | None:
    cid = c.get("id")
    if not cid or not isinstance(cid, str):
        return "test case without id"
    if cid in seen:
        return "duplicate test case id"
    if c.get("schemaVersion") not in (None, SCHEMA_VERSION):
        return f"schemaVersion {c.get('schemaVersion')!r}"
    exp = c.get("expected") or []
    if not exp:
        return "no expected result: the script would have nothing to assert"
    ids = [e.get("id") for e in exp]
    if any(not i or not str(i).startswith(cid + ".E") for i in ids):
        return "expected ids must be <case>.E<n>"
    if len(set(ids)) != len(ids):
        return "duplicate expected ids"
    if any(e.get("op") in (None, "") for e in exp):
        return "expected result without operator"
    return None
