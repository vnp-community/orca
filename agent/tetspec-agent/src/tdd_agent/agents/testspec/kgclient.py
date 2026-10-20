"""Read the design KG over the tdd-agent MCP server (TDD 0.6, SOLUTION 0.3 section 13).

TS1 never touches the ``KnowledgeStore``: it opens an MCP client and only uses the read tools ``kg_elements`` and ``kg_neighbors``.

* ``[testspec] kg_mcp = ""`` (default): run ``tdd-agent -C <project> mcp serve --transport stdio`` as a child process.
* ``kg_mcp = "http(s)://.../mcp"``: a remote MCP server (streamable HTTP), bearer token read from the variable ``kg_mcp_token_env``.

Only the subgraph the scope needs is read: the root elements, neighbours up to ``context_depth`` hops (through projectable classes, at most
``context_limit`` nodes, in the same order as ``scope.neighbors``) and every edge touching those elements.
"""
from __future__ import annotations

import os
import sys
from concurrent.futures import ThreadPoolExecutor
from contextlib import asynccontextmanager
from typing import Any

from ...errors import TddAgentError
from .scope import NO_SECTION, mappable

CHUNK = 200            # codes per call
CLIENT_FACTORY = None  # tests: callable(cfg) -> async context manager yielding an mcp.Client


class KgMcpError(TddAgentError):
    code = "TESTSPEC_KG_MCP"


# ------------------------------------------------------------------ connection
@asynccontextmanager
async def open_client(cfg):
    from mcp import Client, StdioServerParameters
    if CLIENT_FACTORY is not None:
        async with CLIENT_FACTORY(cfg) as client:
            yield client
        return
    ts = cfg.section("testspec")
    url = ts["kg_mcp"]
    if url:
        import httpx2
        from mcp.client.streamable_http import streamable_http_client
        token = os.environ.get(ts["kg_mcp_token_env"], "")
        if not token:
            raise KgMcpError(f"MCP server {url} needs a bearer token: set the environment variable {ts['kg_mcp_token_env']}",
                             code="TESTSPEC_KG_MCP_TOKEN")
        async with httpx2.AsyncClient(headers={"Authorization": f"Bearer {token}"}, timeout=120) as http, \
                Client(streamable_http_client(url, http_client=http)) as client:
            yield client
        return
    params = StdioServerParameters(command=sys.executable, args=["-m", "tdd_agent.cli", "-C", str(cfg.base_dir), "mcp", "serve", "--transport", "stdio"],
                                   env=dict(os.environ), cwd=str(cfg.base_dir))
    async with Client(params) as client:
        yield client


class Reader:
    """Read-tool calls, gathering the ``next`` pages."""

    def __init__(self, client, snapshot: str | None):
        self.client, self.snapshot = client, snapshot
        self.info: dict | None = None
        self.calls = 0

    async def call(self, tool: str, **args: Any) -> dict:
        args = {k: v for k, v in args.items() if v is not None}
        res = await self.client.call_tool(tool, args)
        self.calls += 1
        body = res.structured_content or {}
        if res.is_error:
            err = body.get("error") or {}
            raise TddAgentError(f"{tool}: {err.get('message') or res.content}", code=err.get("code") or "TESTSPEC_KG_MCP",
                                details=err.get("details") or {})
        return body

    async def paged(self, tool: str, part: str, **args: Any) -> dict:
        first, items, offset = None, [], 0
        while offset is not None:
            body = await self.call(tool, offset=offset, **args)
            first = first or body
            items += body.get(part) or []
            offset = body.get("next")
        return {**first, part: items}

    async def elements(self, ids: list[str]) -> tuple[list[dict], list[str]]:
        found, missing = [], []
        ids = list(dict.fromkeys(ids))
        for i in range(0, len(ids), CHUNK):
            body = await self.paged("kg_elements", "elements", ids=ids[i:i + CHUNK], snapshot=self.snapshot)
            self._pin(body)
            found += body["elements"]
            missing += body.get("missing") or []
        return found, missing

    async def members(self, tdd: str, classes: list[str]) -> tuple[list[dict], str]:
        body = await self.paged("kg_elements", "elements", tdd=tdd, classes=classes, snapshot=self.snapshot)
        self._pin(body)
        return body["elements"], body.get("membership") or "tdd"

    async def edges(self, ids: list[str]) -> list[dict]:
        out = []
        ids = sorted(set(ids))
        for i in range(0, len(ids), CHUNK):
            body = await self.paged("kg_neighbors", "edges", ids=ids[i:i + CHUNK], snapshot=self.snapshot)
            out += body["edges"]
        return out

    def _pin(self, body: dict) -> None:
        """Every later call reads the snapshot of the first call (default: the head at the start)."""
        if self.info is None:
            self.info = body["info"]
            self.snapshot = self.info["id"]


# ------------------------------------------------------------------ subgraph
class Subgraph:
    def __init__(self):
        self.els: dict[str, dict] = {}
        self.edges: dict[tuple, dict] = {}

    def add_edges(self, edges: list[dict]) -> None:
        for e in edges:
            self.edges.setdefault((e.get("src"), e.get("rel"), e.get("dst")), e)

    def graph(self) -> dict:
        return {"elements": [self.els[k] for k in sorted(self.els)], "edges": [self.edges[k] for k in sorted(self.edges, key=str)]}


async def _fetch(reader: Reader, sub: Subgraph, ids) -> list[str]:
    need = [i for i in dict.fromkeys(ids) if i and i not in sub.els]
    if not need:
        return []
    found, missing = await reader.elements(need)
    for e in found:
        sub.els[e["id"]] = e
    return missing


async def _expand(reader: Reader, sub: Subgraph, start: list[str], mapping: dict) -> None:
    """Neighbours walked exactly like ``scope.neighbors``, so the subgraph holds every node it will choose."""
    depth, limit = int(mapping.get("context_depth", 2)), int(mapping.get("context_limit", 400))
    seen, frontier, out = set(start), sorted(set(start)), 0
    for level in range(depth + 1):
        edges = await reader.edges(frontier)
        sub.add_edges(edges)
        ends = {x for e in edges for x in (e.get("src"), e.get("dst"))}
        await _fetch(reader, sub, sorted(ends))
        if level == depth or out >= limit:
            break
        adj: dict[str, set[str]] = {}
        for e in edges:
            s, d = e.get("src"), e.get("dst")
            if s in sub.els and d in sub.els:
                adj.setdefault(s, set()).add(d); adj.setdefault(d, set()).add(s)
        nxt = []
        for x in frontier:
            for y in sorted(adj.get(x, ())):
                if y in seen:
                    continue
                seen.add(y)
                if not mappable(mapping, sub.els[y]["class"]):
                    continue
                if out >= limit:
                    break
                out += 1
                nxt.append(y)
        frontier = nxt
        if not frontier:
            break


async def cr_graph(reader: Reader, params: dict, mapping: dict) -> dict:
    """Subgraph for a change-spec scope: the change-spec items, the elements they point to, neighbours."""
    sub = Subgraph()
    await _fetch(reader, sub, params.get("cs_elements") or [])
    refs = []
    for cs in params.get("cs_elements") or []:
        props = (sub.els.get(cs) or {}).get("props") or {}
        refs += [r.get("section_id") for r in list(props.get("srs_refs") or []) + list(props.get("tdd_refs") or []) if r.get("section_id") not in NO_SECTION]
    await _fetch(reader, sub, refs)
    start = [r for r in dict.fromkeys(refs) if r in sub.els and mappable(mapping, sub.els[r]["class"])]
    await _expand(reader, sub, start, mapping)
    return sub.graph()


async def tdd_graph(reader: Reader, tdd: str, mapping: dict) -> tuple[dict, list[str]]:
    """Subgraph for a one-TDD scope: the TDD's projectable elements and their neighbours. Returns (graph, member ids)."""
    sub = Subgraph()
    classes = sorted(set(mapping.get("classes") or {}) | set(mapping.get("extended_passthrough") or []))
    members, _membership = await reader.members(tdd, classes)
    for e in members:
        sub.els[e["id"]] = e
    ids = sorted(e["id"] for e in members)
    await _expand(reader, sub, ids, mapping)
    return sub.graph(), ids


# ------------------------------------------------------------------ synchronous call
def run_sync(fn):
    """Run the coroutine ``fn()`` on its own thread (TS1 may be called from inside an event loop, e.g. MCP ``run_start``)."""
    import anyio

    def go():
        return anyio.run(fn)
    with ThreadPoolExecutor(max_workers=1, thread_name_prefix="kg-mcp") as ex:
        return ex.submit(go).result()


def read_graph(cfg, mode: str, params: dict, mapping: dict) -> dict:
    """Read the subgraph over MCP. Returns {"graph", "info", "members", "calls"}; connection errors -> ``TESTSPEC_KG_MCP``."""
    async def main():
        async with open_client(cfg) as client:
            reader = Reader(client, params.get("snapshot"))
            members = None
            if mode == "cr":
                graph = await cr_graph(reader, params, mapping)
            else:
                graph, members = await tdd_graph(reader, params["tdd"], mapping)
            if reader.info is None:   # no kg_elements call happened (empty scope): the snapshot info is still needed
                body = await reader.call("kg_elements", ids=["-"], snapshot=reader.snapshot)
                reader._pin(body)
            return {"graph": graph, "info": reader.info, "members": members, "calls": reader.calls}
    try:
        return run_sync(main)
    except TddAgentError:
        raise
    except BaseException as exc:  # noqa: BLE001 - connection/protocol errors (including anyio's ExceptionGroup) become a clear gate error
        target = cfg.section("testspec")["kg_mcp"] or f"stdio: tdd-agent -C {cfg.base_dir} mcp serve"
        inner = exc
        while isinstance(inner, BaseExceptionGroup) and inner.exceptions:   # anyio nests error groups per TaskGroup
            inner = inner.exceptions[0]
        if isinstance(inner, TddAgentError):
            raise inner from None
        raise KgMcpError(f"cannot read the knowledge graph over MCP ({target}): {type(inner).__name__}: {inner}"[:500]) from None
