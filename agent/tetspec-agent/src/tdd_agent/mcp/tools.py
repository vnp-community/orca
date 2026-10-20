"""MCP tool implementations.

``McpTools`` holds the behaviour of every tool as a plain method returning JSON
data (or a :class:`Paged` result).  The server wraps each method with tracing,
logging, error mapping and size limiting; nothing here knows about the protocol.

Write and approval methods re-check their policy flag so a code path that
bypasses registration still cannot write or approve.
"""
from __future__ import annotations

import copy
import inspect
import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Callable, Collection, Mapping, Sequence

from ..core import o5_path
from ..errors import ApprovalRejected, ConfigError, NotFound, TddAgentError
from ..obs import context
from ..obs.tracing import render_tree
from ..orchestrator import dispatch
from ..orchestrator.context import sad_violations
from ..orchestrator.model import ApprovalRequest
from ..retrieval.expansion import PROFILES
from ..retrieval.queries import QueryService
from ..runtime import Runtime
from ..store.base import DIRECTIONS, SnapshotInfo, SourceDoc, digest, now_iso

IMPACT_GATES = ("G2",)
READINESS_GATES = ("G4", "G5")
WRITE_TOOLS = ("run_start", "run_resume", "run_dispatch", "run_cancel", "generate_start", "change_start", "changeset_propose", "proposals_list",
               "testspec_start")
EXTERNAL_KEYS = ("system", "executionId", "stepId", "userId")
TESTSPEC_DOCUMENTS = {"requirements": "01-test-requirements.md", "design": "02-test-design.md", "cases": "03-test-cases.md"}
APPROVAL_TOOL = "approval_submit"
MIN_RESULT_CHARS = 256
META_RESERVE = 160
MAX_LIST_LIMIT = 1000


def invalid(message: str, **details: Any) -> TddAgentError:
    return TddAgentError(message, code="INVALID_ARGUMENT", details=details)


# ---------------------------------------------------------------------- policy
@dataclass(frozen=True)
class McpPolicy:
    """What this server instance may do, decided once from configuration."""
    write: bool
    approvals_requested: bool
    actor: str
    actor_allowlisted: bool
    transport: str
    max_chars: int

    @classmethod
    def from_runtime(cls, runtime: Runtime, client_actor: str | None = None, transport: str | None = None) -> "McpPolicy":
        cfg = runtime.config.section("mcp")
        actor = (client_actor if client_actor is not None else cfg["client_actor"]) or ""
        max_chars = cfg["max_result_chars"]
        if not isinstance(max_chars, int) or isinstance(max_chars, bool) or max_chars < MIN_RESULT_CHARS:
            raise ConfigError(f"mcp.max_result_chars must be an integer >= {MIN_RESULT_CHARS}")
        return cls(write=bool(cfg["write"]), approvals_requested=bool(cfg["approvals"]), actor=actor,
                   actor_allowlisted=any(a.id == actor for a in runtime.config.actors()) if actor else False,
                   transport=transport or cfg["transport"], max_chars=max_chars)

    @property
    def approvals(self) -> bool:
        """The approval tool exists only with both keys: the flag and an allowlisted client."""
        return self.approvals_requested and self.actor_allowlisted

    @property
    def actor_label(self) -> str:
        return self.actor or "anonymous"


# ---------------------------------------------------------------------- pagination
@dataclass
class Paged:
    """A result whose list-like parts may be cut to fit ``max_result_chars``.

    ``parts`` are dotted paths to lists or dicts inside ``result``; their items form one
    sequence addressed by ``offset``.  ``text`` names a long string field that is cut by
    characters instead.  ``pinned`` keys of a dict part ride on the first page.  ``base`` is
    the absolute offset of the first fetched item when the tool itself pages a store.
    """
    result: dict
    parts: Sequence[str] = ()
    offset: int = 0
    text: str | None = None
    pinned: Mapping[str, Collection[str]] = field(default_factory=dict)
    derive: Callable[[dict], dict] | None = None
    base: int = 0
    after: Callable[[dict], None] | None = None


def size_of(value: Any) -> int:
    return len(dumps(value))


def dumps(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, default=str, separators=(",", ":"))


def _get(obj: Any, path: str) -> Any:
    for part in path.split("."):
        if not isinstance(obj, dict) or part not in obj:
            return None
        obj = obj[part]
    return obj


def _with(obj: dict, path: str, value: Any) -> dict:
    head, _, rest = path.partition(".")
    out = dict(obj)
    out[head] = _with(obj[head], rest, value) if rest else value
    return out


def paginate(result: dict, parts: Sequence[str], *, limit: int, offset: int = 0, text: str | None = None,
             pinned: Mapping[str, Collection[str]] | None = None, derive: Callable[[dict], dict] | None = None,
             base: int = 0) -> dict:
    """Return ``result`` cut to ``limit`` characters, with ``truncated``, ``next`` and ``page``.

    Nothing is dropped silently: the items left out are reachable with ``offset=next``.  An atomic
    item (or a result without list parts) that alone exceeds ``limit`` is returned whole and flagged
    ``oversize`` rather than cut in the middle.
    """
    if text is not None:
        return _paginate_text(result, text, limit=limit, offset=offset)
    pinned = pinned or {}
    entries: list[tuple[str, str | None, Any]] = []
    for path in parts:
        value = _get(result, path)
        if isinstance(value, list):
            entries.extend((path, None, v) for v in value)
        elif isinstance(value, dict):
            entries.extend((path, k, v) for k, v in value.items() if k not in pinned.get(path, ()))
    total = len(entries)
    if offset < 0 or offset > total:
        raise TddAgentError(f"offset {offset} is outside 0..{total}", code="INVALID_CURSOR", details={"offset": offset, "total": total})

    def assemble(count: int) -> dict:
        out = result
        for path in parts:
            value = _get(result, path)
            if isinstance(value, list):
                out = _with(out, path, [])
            elif isinstance(value, dict):
                out = _with(out, path, {k: v for k, v in value.items() if offset == 0 and k in pinned.get(path, ())})
        for path, key, item in entries[offset:offset + count]:
            container = _get(out, path)
            if key is None:
                container.append(item)
            else:
                container[key] = item
        return derive(out) if derive else out

    def finish(page: dict, count: int) -> dict:
        more = offset + count < total
        page = dict(page, truncated=more, next=base + offset + count if more else None)
        if more or offset > 0 or base:
            page["page"] = {"offset": base + offset, "returned": count, "total": total if not base else None}
        return page

    if offset == 0:
        whole = finish(assemble(total), total)
        if size_of(whole) <= limit:
            return whole
    spent = size_of(assemble(0)) + META_RESERVE
    count = 0
    for path, key, item in entries[offset:]:
        cost = size_of(item) + (len(key) + 4 if key is not None else 1)
        if count and spent + cost > limit:
            break
        spent += cost
        count += 1
    page = finish(assemble(count), count)
    while size_of(page) > limit and count > 1:
        count = max(1, count - max(1, count // 8))
        page = finish(assemble(count), count)
    if size_of(page) > limit:
        page["oversize"] = True
    return page


def _paginate_text(result: dict, key: str, *, limit: int, offset: int) -> dict:
    text = str(_get(result, key) or "")
    total = len(text)
    if offset < 0 or offset > total:
        raise TddAgentError(f"offset {offset} is outside 0..{total}", code="INVALID_CURSOR", details={"offset": offset, "total": total})

    def build(end: int) -> dict:
        more = end < total
        page = _with(result, key, text[offset:end]) if "." in key else dict(result, **{key: text[offset:end]})
        page.update(truncated=more, next=end if more else None)
        if more or offset:
            page["page"] = {"offset": offset, "returned": end - offset, "total": total, "unit": "chars"}
        return page

    end = total
    page = build(end)
    size = size_of(page)
    while size > limit and end - offset > 1:
        overshoot = limit / size
        end = offset + max(1, min(end - offset - 1, int((end - offset) * overshoot * 0.97)))
        page = build(end)
        size = size_of(page)
    if size > limit:
        page["oversize"] = True
    return page


# ---------------------------------------------------------------------- tools
class McpTools:
    def __init__(self, runtime: Runtime, policy: McpPolicy):
        self.rt = runtime
        self.policy = policy
        self.knowledge = runtime.knowledge
        self.operational = runtime.operational

    # -------------------------------------------------------------- helpers
    def snapshot_used(self, explicit: str | None) -> str | None:
        """Id echoed in every result: the explicit snapshot, else the head (``None`` for an empty store)."""
        if explicit is not None:
            return explicit
        head = self.knowledge.head()
        return head.id if head else None

    def _snap(self, ref: str | None, *, required: bool = True) -> SnapshotInfo | None:
        """Resolve an explicit snapshot (never falling back to head) or the head."""
        if ref is not None:
            if not str(ref).strip():
                raise invalid("snapshot must not be empty; omit it to use the head")
            return self.knowledge.get_snapshot(ref)
        head = self.knowledge.head()
        if head is None and required:
            raise NotFound("the knowledge store has no snapshot yet", code="SNAPSHOT_NOT_FOUND")
        return head

    @staticmethod
    def _offset(offset: int) -> int:
        if offset < 0:
            raise invalid("offset must be >= 0", offset=offset)
        return offset

    @staticmethod
    def _limit(limit: int | None, default: int | None = None) -> int | None:
        if limit is None:
            return default
        if limit < 1 or limit > MAX_LIST_LIMIT:
            raise invalid(f"limit must be between 1 and {MAX_LIST_LIMIT}", limit=limit)
        return limit

    def _can_review(self) -> bool:
        return bool(self.policy.actor) and self.policy.actor_allowlisted

    def _require_write(self) -> None:
        if not self.policy.write:
            raise TddAgentError("write tools are disabled; set [mcp] write = true", code="WRITE_DISABLED")

    def _origin(self) -> dict:
        return {"actor": self.policy.actor_label, "actor_kind": "agent", "channel": "mcp"}

    def _audit_write(self, tool: str, run_id: str | None, **fields: Any) -> None:
        self.operational.add_audit("mcp_write", {"tool": tool, **self._origin(), **fields}, run_id)

    # -------------------------------------------------------------- knowledge graph
    def kg_get_element(self, id: str, snapshot: str | None = None) -> dict:
        """Return one element of the knowledge graph at a snapshot (default: head)."""
        snap = self._snap(snapshot)
        element = self.knowledge.get_element(id, snap.id)
        if element is None:
            raise NotFound(f"element not found: {id}", code="ELEMENT_NOT_FOUND", details={"id": id, "snapshot": snap.id})
        return {"snapshot": snap.id, "snapshot_hash": snap.hash, "id": id, "element": element}

    def kg_elements(self, ids: list[str] | None = None, tdd: str | None = None, classes: list[str] | None = None,
                    snapshot: str | None = None, offset: int = 0) -> Paged:
        """Return many elements of one snapshot: by ids (unknown ids listed in missing), or the members of a TDD (tdd), optionally only some classes."""
        if not ids and not tdd:
            raise invalid("pass ids or tdd")
        snap = self._snap(snapshot)
        membership = None
        if ids:
            found = self.knowledge.get_elements(list(dict.fromkeys(ids)), snap.id)
            have = {e["id"] for e in found}
            missing = [i for i in dict.fromkeys(ids) if i not in have]
        else:
            from ..agents.common import tdd_members
            graph = self.knowledge.materialize(snap.id)
            found, _ = tdd_members(graph, tdd)
            membership = "tdd"
            if not found and not any(e.get("tdds") or (e.get("props") or {}).get("tdds") for e in graph.get("elements", [])):
                found, membership = list(graph.get("elements", [])), "whole_kg"   # one-TDD project: elements carry no TDD membership
            missing = []
        if classes:
            found = [e for e in found if e.get("class") in set(classes)]
        found = sorted(found, key=lambda e: e["id"])
        info = {"id": snap.id, "seq": snap.seq, "hash": snap.hash, "created_at": snap.created_at, "graph_version": snap.graph_version}
        return Paged({"snapshot": snap.id, "info": info, "membership": membership, "missing": missing, "elements": found},
                     parts=("elements",), offset=self._offset(offset))

    def kg_neighbors(self, ids: list[str], relations: list[str] | None = None, direction: str = "both",
                     snapshot: str | None = None, offset: int = 0) -> Paged:
        """Return the edges touching the given element ids, optionally restricted to relations and a direction (out, in, both)."""
        if direction not in DIRECTIONS:
            raise invalid(f"direction must be one of {list(DIRECTIONS)}", direction=direction)
        snap = self._snap(snapshot)
        edges = self.knowledge.neighbors(ids, relations, direction, snap.id)
        return Paged({"snapshot": snap.id, "id_count": len(ids), "relations": relations, "direction": direction, "edges": edges},
                     parts=("edges",), offset=self._offset(offset))

    def kg_resolve_scope(self, seeds: list[str], profile: str = "tdd-generation", budget: int | None = None,
                         snapshot: str | None = None, mandatory_ids: list[str] | None = None, offset: int = 0) -> Paged:
        """Resolve the evidence closure of the seed elements; the result carries the full closure and its resolution_hash."""
        if not seeds:
            raise invalid("seeds must not be empty")
        if profile not in PROFILES:
            raise invalid(f"unknown profile {profile!r}", profiles=sorted(PROFILES))
        if budget is not None and budget < 0:
            raise invalid("budget must be >= 0", budget=budget)
        snap = self._snap(snapshot)
        closure = self.rt.closure().resolve(seeds, profile=profile, budget=budget, snapshot=snap.id, mandatory_ids=mandatory_ids or ())
        data = closure.to_dict()
        result = {"snapshot": snap.id, "resolution_hash": data["resolution_hash"], "blockers": closure.blockers, "closure": data}
        parts = tuple(f"closure.{k}" for k in ("selected_ids", "reasons", "edges", "gaps", "conflicts", "unresolved", "excluded",
                                              "obligations", "source_refs", "boundary", "missing_ids"))
        return Paged(result, parts=parts, offset=self._offset(offset))

    def kg_query(self, name: str, args: dict | None = None, snapshot: str | None = None, offset: int = 0) -> Paged:
        """Run one of the typed queries Q1-Q9 (name) with its keyword arguments (args) at a snapshot."""
        svc = self.rt.queries()
        key = name.upper()
        if key not in QueryService.NAMES:
            raise NotFound(f"unknown query {name}; expected one of {sorted(QueryService.NAMES)}", code="QUERY_UNKNOWN")
        kwargs = dict(args or {})
        if "snapshot" in kwargs:
            raise invalid("pass snapshot as the tool argument, not inside args")
        snap = self._snap(snapshot)
        try:
            inspect.signature(getattr(svc, QueryService.NAMES[key])).bind(snapshot=snap.id, **kwargs)
        except TypeError as exc:
            raise invalid(f"bad arguments for {key}: {exc}", query=key) from exc
        result = svc.run(key, snapshot=snap.id, **kwargs).to_dict()
        return Paged(dict(result, snapshot=snap.id), parts=("rows",), offset=self._offset(offset))

    def kg_search(self, query: str, classes: list[str] | None = None, snapshot: str | None = None, limit: int | None = None, offset: int = 0) -> Paged:
        """Full-text search of the elements live at a snapshot (accents and capitals ignored; an id in full or in part comes first).

        A hit is a proposal: ``verified`` says whether the element exists in the snapshot with resolvable evidence and passes the authority checks, and
        ``rejected`` says why not.  Use ``kg_resolve_scope`` with verified ids to get the evidence closure.
        """
        from ..retrieval.candidates import verify_candidates
        if not str(query).strip():
            raise invalid("query must not be empty")
        snap = self._snap(snapshot)
        hits = self.knowledge.search(query, snap.id, classes, self._limit(limit, 20))
        spec = verify_candidates(self.rt.closure(), [h.id for h in hits], snap.id)
        rejected = {r["id"]: r["reason"] for r in spec.meta["rejected"]}
        rows = [{**h.as_dict(), "verified": h.id in spec.ids, **({"rejected": rejected[h.id]} if h.id in rejected else {})} for h in hits]
        return Paged({"snapshot": snap.id, "query": query, "hits": rows}, parts=("hits",), offset=self._offset(offset))

    def kg_snapshots(self, limit: int | None = None, offset: int = 0) -> Paged:
        """List snapshots, newest first."""
        head = self.knowledge.head()
        rows = [s.as_dict() for s in self.knowledge.snapshots(self._limit(limit))]
        return Paged({"snapshot": head.id if head else None, "head": head.id if head else None, "snapshots": rows},
                     parts=("snapshots",), offset=self._offset(offset))

    def source_get(self, id: str, revision: str | None = None, role: str | None = None, snapshot: str | None = None,
                   offset: int = 0) -> Paged:
        """Return a pinned source document exactly as stored (latest revision unless one is given); text is cut by characters."""
        snap = self._snap(snapshot, required=False)
        doc = self.knowledge.get_source(id, revision, role)
        meta = {"id": doc.id, "revision": doc.revision, "role": doc.role, "content_hash": doc.content_hash, "meta": doc.meta,
                "registered_at": doc.registered_at}
        return Paged({"snapshot": snap.id if snap else None, "source": meta, "length": len(doc.text), "text": doc.text},
                     text="text", offset=self._offset(offset))

    # -------------------------------------------------------------- runs, decisions, traces
    def _gate_view(self, task: dict | None) -> dict:
        return {"status": "not_started"} if task is None else copy.deepcopy(task)

    def _review(self, run_id: str, gate: str, view: dict) -> dict | None:
        """Record that the client actor read the pending decision of ``gate`` (the receipt approvals require)."""
        if view.get("status") != "awaiting_approval" or not self._can_review():
            return None
        try:
            read = self.rt.orchestrator.get_decision(run_id, gate, self.policy.actor, source="mcp")
        except NotFound:
            return None
        view.update(decision=read["decision"], decision_hash=read["decision_hash"], approval_role=read["approval_role"])
        return {"gate": gate, "decision_hash": read["decision_hash"], "actor_id": self.policy.actor, "source": "mcp"}

    def _decision_view(self, run_id: str, gates: Sequence[str], snapshot: str | None) -> Paged:
        snap = self._snap(snapshot, required=False)
        run = self.operational.get_run(run_id)
        views: dict[str, dict] = {}
        receipts: list[dict] = []
        for gate in gates:
            view = self._gate_view(self.operational.get_task(run_id, gate))
            receipt = self._review(run_id, gate, view)
            if receipt:
                receipts.append(receipt)
            views[gate] = view
        result = {"snapshot": snap.id if snap else None, "run_id": run_id, "flow": run.flow, "status": run.status, "state": run.state,
                  "pending_gate": run.result.get("pending_gate"), "run_blockers": run.result.get("blockers", []),
                  "gates": views, "review_receipts": receipts, "reviewer": self.policy.actor or None}
        return Paged(result)

    def impact_get(self, run_id: str, snapshot: str | None = None) -> Paged:
        """Return the impact decision (gate G2) of a run; reading a decision that awaits approval records a review receipt for the client actor."""
        return self._decision_view(run_id, IMPACT_GATES, snapshot)

    def readiness_get(self, run_id: str, snapshot: str | None = None) -> Paged:
        """Return the spec readiness evaluation (G4) and the authored-spec decision (G5) of a run; reading a pending decision records a review receipt for the client actor."""
        return self._decision_view(run_id, READINESS_GATES, snapshot)

    def run_list(self, flow: str | None = None, status: str | None = None, limit: int = 50, offset: int = 0,
                 snapshot: str | None = None, parent_run_id: str | None = None, external_execution_id: str | None = None,
                 external_step_id: str | None = None) -> Paged:
        """List runs (newest first), optionally filtered by flow, status, parent run or the external execution/step that started them; `next` continues the listing."""
        snap = self._snap(snapshot, required=False)
        limit = self._limit(limit, 50)
        offset = self._offset(offset)
        if parent_run_id or external_execution_id or external_step_id:
            def keep(r) -> bool:
                ext = ((r.request or {}).get("origin") or {}).get("external") or {}
                return ((not parent_run_id or r.parent_run_id == parent_run_id)
                        and (not external_execution_id or ext.get("executionId") == external_execution_id)
                        and (not external_step_id or ext.get("stepId") == external_step_id))
            matched = [r for r in self.operational.list_runs(flow=flow, status=status, limit=100000) if keep(r)]
            rows = [r.as_dict() for r in matched[offset:offset + limit]]
        else:
            rows = [r.as_dict() for r in self.operational.list_runs(flow=flow, status=status, limit=limit, offset=offset)]

        def more(page: dict) -> None:
            if page["next"] is None and len(rows) == limit:
                page["next"] = offset + limit

        filters = {"flow": flow, "status": status, "parent_run_id": parent_run_id, "external_execution_id": external_execution_id,
                   "external_step_id": external_step_id}
        return Paged({"snapshot": snap.id if snap else None, "filters": filters, "runs": rows},
                     parts=("runs",), base=offset, after=more)

    def run_get(self, run_id: str, snapshot: str | None = None, offset: int = 0) -> Paged:
        """Return a run with its gate ledger, transitions and approvals; pending decisions are included whole and recorded as read by the client actor."""
        snap = self._snap(snapshot, required=False)
        view = self.rt.orchestrator.run_view(run_id)
        offset = self._offset(offset)
        awaiting = sorted(g for g, t in view["tasks"].items() if t.get("status") == "awaiting_approval")
        receipts: list[dict] = []
        if offset == 0:
            for gate in awaiting:
                receipt = self._review(run_id, gate, view["tasks"][gate])
                if receipt:
                    receipts.append(receipt)
        children = [{"run_id": r.run_id, "flow": r.flow, "status": r.status, "created_at": r.created_at}
                    for r in self.operational.list_runs(limit=100000) if r.parent_run_id == run_id]
        result = dict(view, snapshot=snap.id if snap else None, run_id=run_id, review_receipts=receipts, reviewer=self.policy.actor or None,
                      children=children, process=dispatch.read_proc(self.rt.config.work_dir, run_id))
        return Paged(result, parts=("tasks", "transitions", "approvals"), offset=offset, pinned={"tasks": set(awaiting)})

    def trace_get(self, run_id: str | None = None, trace_id: str | None = None, snapshot: str | None = None,
                  offset: int = 0) -> Paged:
        """Return the spans of a run's trace (give run_id or trace_id) and the rendered span tree of the returned page; calls to this tool are not part of what it returns."""
        if bool(run_id) == bool(trace_id):
            raise invalid("give exactly one of run_id or trace_id")
        snap = self._snap(snapshot, required=False)
        if run_id:
            trace_id = self.operational.get_run(run_id).trace_id
        spans = [s for s in self.operational.list_spans(trace_id=trace_id) if s["name"] != "mcp:trace_get"]
        if not spans and not run_id:
            raise NotFound(f"no spans recorded for trace {trace_id}", code="TRACE_NOT_FOUND", details={"trace_id": trace_id})
        run_ids = sorted({s["run_id"] for s in spans if s.get("run_id")})
        result = {"snapshot": snap.id if snap else None, "trace_id": trace_id, "run_id": run_id or (run_ids[0] if len(run_ids) == 1 else None),
                  "span_count": len(spans), "spans": spans, "tree": ""}
        return Paged(result, parts=("spans",), offset=self._offset(offset), derive=lambda page: dict(page, tree=render_tree(page["spans"])))

    def metrics_get(self) -> Paged:
        """Return operational counters (runs, blocked gates, LLM calls and tokens) and the share of LLM-resolved skills per agent."""
        metrics = self.rt.observer.metrics
        return Paged({"snapshot": self.snapshot_used(None), "metrics": metrics.summary(), "llm_share": metrics.llm_share()},
                     parts=("metrics", "llm_share"))

    # -------------------------------------------------------------- resources
    def resource_snapshot(self, id: str) -> Paged:
        snap = self.knowledge.get_snapshot(id)
        return Paged({"snapshot": snap.id, "info": snap.as_dict(), "stats": self.knowledge.stats(snap.id)}, parts=("stats.by_class", "stats.by_relation"))

    def resource_source(self, id: str) -> Paged:
        doc = self.knowledge.get_source(id)
        revisions = [{"revision": d.revision, "role": d.role, "content_hash": d.content_hash} for d in self.knowledge.list_sources() if d.id == id]
        meta = {"id": doc.id, "revision": doc.revision, "role": doc.role, "content_hash": doc.content_hash, "meta": doc.meta,
                "registered_at": doc.registered_at}
        return Paged({"snapshot": self.snapshot_used(None), "source": meta, "revisions": revisions, "length": len(doc.text), "text": doc.text}, text="text")

    def resource_run(self, id: str) -> Paged:
        view = self.rt.orchestrator.run_view(id)
        return Paged(dict(view, snapshot=self.snapshot_used(None), run_id=id), parts=("tasks", "transitions", "approvals"))

    # -------------------------------------------------------------- write tools
    def run_start(self, flow: str, request: dict, sources: list[dict] | None = None, external: dict | None = None) -> dict:
        """Create a run of a registered flow (write tool; attributed to an agent). Use run_resume or run_dispatch to advance it.

        A source is {id, revision, role, text} or {id, revision, role, path} with a path inside the project. ``external``
        ({system, executionId, stepId, userId}) records the caller's own job, so run_list can find the run again."""
        self._require_write()
        docs = [self._source_doc(s, self.rt.config.path("paths.repo_root")) for s in sources or []]
        request = dict(request, origin=self._external_origin(external))
        run = self.rt.orchestrator.start(flow, request, sources=docs)
        self._audit_write("run_start", run.run_id, flow=flow, sources=[d.id for d in docs])
        return {"snapshot": self.snapshot_used(None), "run_id": run.run_id, "run": run.as_dict()}

    @staticmethod
    def _source_doc(raw: dict, root: Path | None = None) -> SourceDoc:
        if isinstance(raw, dict) and "text" not in raw and "path" in raw and root is not None:
            raw = dict(raw, text=McpTools._read_source_path(raw["path"], root))
        if not isinstance(raw, dict) or not {"id", "revision", "role", "text"} <= set(raw):
            raise invalid("each source needs id, revision, role and text (or path)")
        if not isinstance(raw["text"], str):
            raise invalid("source text must be a string", id=raw.get("id"))
        try:
            return SourceDoc.make(str(raw["id"]), str(raw["revision"]), str(raw["role"]), raw["text"], raw.get("meta"))
        except AttributeError as exc:  # pragma: no cover - role guaranteed to be str above
            raise invalid(str(exc)) from exc

    @staticmethod
    def _read_source_path(path: Any, root: Path) -> str:
        """Text of a source file given relative to the project; paths leaving the project (also through symlinks) are refused."""
        if not isinstance(path, str) or not path.strip():
            raise invalid("source path must be a non-empty string")
        base = Path(root).resolve()
        target = (base / path).resolve()
        if not target.is_relative_to(base):
            raise TddAgentError("source path must stay inside the project", code="SOURCE_PATH_OUTSIDE_PROJECT", details={"path": path})
        if not target.is_file():
            raise NotFound(f"source file {path} does not exist", code="SOURCE_PATH_NOT_FOUND", details={"path": path})
        return target.read_text(encoding="utf-8")

    def _external_origin(self, external: dict | None) -> dict:
        origin = self._origin()
        if external is not None:
            if not isinstance(external, dict) or set(external) - set(EXTERNAL_KEYS) or not all(isinstance(v, str) for v in external.values()):
                raise invalid(f"external must be an object with string fields among {list(EXTERNAL_KEYS)}")
            origin["external"] = dict(external)
        return origin

    def generate_start(self, external: dict | None = None, force: bool = False) -> dict:
        """Start building the knowledge graph and TDD from the project's documents, like `tdd-agent generate`, without running it (write tool).

        Returns {status: "started", run_id} (advance it with run_dispatch; the TDD file is written when the run completes) or
        {status: "up_to_date", document} when nothing changed since the last TDD."""
        self._require_write()
        from .. import project
        started = project.prepare_generation(self.rt, {}, tdd_id=None, out=None, prose=None, prose_limit=None, force=force,
                                             origin=self._external_origin(external))
        if isinstance(started, str):
            self._audit_write("generate_start", started)
            return {"status": "started", "run_id": started}
        result, _ = started
        return result

    def change_start(self, name: str, cr_path: str | None = None, cr_text: str | None = None, srs_after_path: str | None = None,
                     cr_id: str | None = None, title: str | None = None, acceptance: list[str] | None = None,
                     code_targets: list[str] | None = None, verifications: list[str] | None = None, external: dict | None = None) -> dict:
        """Start a change request like `tdd-agent change start`, without running it (write tool): advance it with run_dispatch.

        Paths are relative to the project. The run id is change-<name>."""
        self._require_write()
        from .. import project
        root = self.rt.config.path("paths.repo_root")
        cr_file = str((Path(root) / cr_path)) if cr_path else None
        if cr_path:
            self._read_source_path(cr_path, root)  # refuses paths leaving the project
        srs_after = None
        if srs_after_path:
            self._read_source_path(srs_after_path, root)
            srs_after = str(Path(root) / srs_after_path)
        result, _ = project.change_start(self.rt, name, cr_file=cr_file, cr_text=cr_text, title=title, acceptance=list(acceptance or []), cr_id=cr_id,
                                         srs_after=srs_after, purpose=None, scope=None, owner=None, code_targets=code_targets, verifications=verifications,
                                         new_in=None, allow_draft=False, advance=False, origin=self._external_origin(external))
        self._audit_write("change_start", result.get("run_id"))
        return result

    def testspec_start(self, context: dict, tdd: str | None = None, snapshot: str | None = None, external: dict | None = None) -> dict:
        """Start a test-spec run with the Solution or the Plan of a request as extra context (write tool); advance it with run_dispatch.

        ``context`` = {kind, ref, document | files | path}. ``kind`` is "solution" or "plan" and must equal [testspec] context_mode.
        solution: ``document`` is the Solution JSON. plan: ``files`` maps tasks.md (required), proposal.md, design.md and
        specs/<name>/spec.md to their text. ``path`` names a document already in the project instead. The documents are written to
        .tdd-agent/context/<kind>-<ref>/ (names and sizes are checked, nothing is written when a check fails). ``external`` makes the call
        repeatable: the same execution, step and content return the same run (reused: true)."""
        self._require_write()
        from ..agents.testspec import context as ctxdoc
        from ..agents.testspec import runner as tsrunner
        cfg = self.rt.config
        ts = cfg.section("testspec")
        if not ts["enabled"]:
            raise TddAgentError("test specs are disabled; set [testspec] enabled = true", code="TESTSPEC_DISABLED")
        if not isinstance(context, dict):
            raise invalid("context must be an object {kind, ref, document | files | path}")
        kind, mode = context.get("kind"), ts["context_mode"]
        if kind not in ("solution", "plan") or kind != mode:
            raise TddAgentError(f"context.kind {kind!r} does not match [testspec] context_mode {mode!r}", code="TESTSPEC_CONTEXT_MODE_MISMATCH",
                                details={"configured": mode, "requested": kind})
        tsrunner.preflight(cfg)    # LLM not configured: the error is returned now, no run is created
        ref = ctxdoc.check_ref(context.get("ref"))
        given = [k for k in ("document", "files", "path") if context.get(k) is not None]
        if len(given) != 1:
            raise TddAgentError("context needs exactly one of document, files, path", code="TESTSPEC_CONTEXT_FILE")
        scope = tdd or cfg.get("project.tdd_id")
        if not scope:
            raise invalid("tdd is required (or set [project] tdd_id)")
        if given == ["path"]:
            if not isinstance(context["path"], str):
                raise invalid("context.path must be a string")
            written = {"path": context["path"], "files": []}
        else:
            written = ctxdoc.write_inline(cfg, kind, ref, context)
        bundle = ctxdoc.read_bundle(cfg, kind, written["path"], ref)
        params = {"tdd": scope, "context_path": written["path"], "context_ref": ref, "context_sha256": bundle.sha256}
        if snapshot:
            params["snapshot"] = snapshot
        origin = self._external_origin(external)
        ext = origin.get("external") or {}
        if ext.get("executionId") and ext.get("stepId"):
            for r in self.operational.list_runs(flow="testspec", limit=100000):
                rext = ((r.request or {}).get("origin") or {}).get("external") or {}
                if (rext.get("executionId") == ext["executionId"] and rext.get("stepId") == ext["stepId"] and r.status != "failed"
                        and ((r.request or {}).get("params") or {}).get("context_sha256") == bundle.sha256):
                    return {"run_id": r.run_id, "reused": True, "snapshot": self.snapshot_used(None), "run": r.as_dict(),
                            "context": {**bundle.meta(), "path": written["path"], "files": written["files"], "notes": bundle.notes}}
        run = self.rt.orchestrator.start("testspec", {"mode": "run", "params": params, "origin": origin}, sources=[])
        self._audit_write("testspec_start", run.run_id, kind=kind, ref=ref, sha256=bundle.sha256[:12])
        return {"run_id": run.run_id, "reused": False, "snapshot": self.snapshot_used(None), "run": run.as_dict(),
                "context": {**bundle.meta(), "path": written["path"], "files": written["files"], "notes": bundle.notes}}

    def testspec_get(self, feature: str | None = None, document: str | None = None, offset: int = 0) -> Paged:
        """Read the test specs of the project. No argument: the effective [testspec] configuration (with context_mode) and the features.

        ``feature``: its latest version, counts, open TestGaps and the context of the last run. ``feature`` + ``document``
        (requirements | design | cases): that markdown document; text is cut by characters, repeat with offset=next."""
        from ..agents.testspec import runner as tsrunner
        cfg = self.rt.config
        ts = cfg.section("testspec")
        info = {"enabled": ts["enabled"], "llm": ts["llm"] or None, "context_mode": ts["context_mode"], "context_path": ts["context_path"],
                "specs_dir": ts["specs_dir"]}
        offset = self._offset(offset)
        c = tsrunner.tsagent_config(cfg)
        if document is not None and feature is None:
            raise invalid("document needs a feature")
        if document is not None and document not in TESTSPEC_DOCUMENTS:
            raise invalid(f"document must be one of {list(TESTSPEC_DOCUMENTS)}", document=document)
        if not Path(c.db).is_file():
            if feature is None:
                return Paged({"config": info, "features": []}, parts=("features",), offset=offset)
            raise NotFound(f"feature {feature!r} has no test specs yet", code="TESTSPEC_NOT_FOUND")
        from tsagent.errors import TsagentError
        from tsagent.store import tso as store_tso
        from tsagent.store.db import open_db
        db = open_db(c)
        try:
            if feature is None:
                rows = db.q("SELECT feature, MAX(version) AS version FROM tso_spec_version GROUP BY feature ORDER BY feature")
                feats = [self._testspec_row(cfg, c, store_tso.version_row(db, r["feature"], r["version"]), r["feature"]) for r in rows]
                return Paged({"config": info, "features": feats}, parts=("features",), offset=offset)
            try:
                version = store_tso.version_row(db, feature)
            except TsagentError as exc:
                raise NotFound(str(exc), code="TESTSPEC_NOT_FOUND") from exc
            if document is not None:
                f = Path(c.specs_root) / feature / TESTSPEC_DOCUMENTS[document]
                if not f.is_file():
                    raise NotFound(f"{TESTSPEC_DOCUMENTS[document]} of {feature!r} is not on disk", code="TESTSPEC_NOT_FOUND")
                text = f.read_text(encoding="utf-8")
                return Paged({"feature": feature, "document": document, "version": version["version"], "status": version["status"],
                              "length": len(text), "text": text}, text="text", offset=offset)
            g = store_tso.load_spec(db, feature)
            gaps = []
            for x in g.of("TestGap"):
                st = (g.meta["lifecycle"].get(x.code) or {}).get("review_status")
                if st in ("approved", "rejected"):
                    continue
                gaps.append({"code": x.code, "missing": str(x.attrs.get("missing", ""))[:300], "question": str(x.attrs.get("askedQuestion", ""))[:300],
                             "owner": x.attrs.get("owner"), "blocks": len(x.rel("blocks"))})
            row = self._testspec_row(cfg, c, version, feature)
            row.update(counts={k: len(list(g.of(k))) for k in ("TestRequirement", "TestCase", "TestGap")}, gaps=gaps, context=self._testspec_context(cfg, feature))
            return Paged({"config": info, "feature": row}, parts=("feature.gaps",), offset=offset)
        finally:
            db.close()

    @staticmethod
    def _testspec_row(cfg, c, version: dict, feature: str) -> dict:
        d = Path(c.specs_root) / feature
        files = sorted(str(f.resolve().relative_to(Path(cfg.repo_root).resolve())) for f in d.glob("*.md")) if d.is_dir() else []
        return {"feature": feature, "version": version["version"], "status": version["status"], "updated_at": version.get("created_at"), "files": files}

    @staticmethod
    def _testspec_context(cfg, feature: str) -> dict | None:
        """The ``context`` block and the provenance of the latest run that wrote this feature (from the run folders)."""
        best = None
        for f in (Path(cfg.work_dir) / "runs").glob(f"*/testspec/{feature}.summary.json"):
            if best is None or f.stat().st_mtime > best.stat().st_mtime:
                best = f
        if best is None:
            return None
        try:
            data = json.loads(best.read_text(encoding="utf-8"))
        except (OSError, ValueError):
            return None
        return {"run_dir": best.parent.parent.name, "origin_context": (data.get("origin") or {}).get("context"), "context": data.get("context")}

    def run_dispatch(self, run_id: str) -> dict:
        """Advance a run in a background process and return at once (write tool); poll run_get for its status and `process`.

        Does nothing when the run is finished (RUN_TERMINAL) or a background advance of it is alive (RUN_ALREADY_RUNNING)."""
        self._require_write()
        run = self.operational.get_run(run_id)
        if run.status in ("completed", "failed"):
            return {"run_id": run_id, "dispatched": False, "reason": "RUN_TERMINAL", "status": run.status}
        proc = dispatch.read_proc(self.rt.config.work_dir, run_id)
        if proc and proc["alive"]:
            return {"run_id": run_id, "dispatched": False, "reason": "RUN_ALREADY_RUNNING", "status": run.status, "pid": proc["pid"]}
        self._audit_write("run_dispatch", run_id)
        pid = dispatch.LAUNCH(self.rt.config.base_dir, self.rt.config.work_dir, run_id)
        return {"run_id": run_id, "dispatched": True, "pid": pid, "status": run.status}

    def run_cancel(self, run_id: str, reason: str) -> dict:
        """Stop a run (write tool): terminates its background advance and fails it and its unfinished child runs with CANCELLED_BY_CLIENT."""
        self._require_write()
        if not (reason or "").strip():
            raise invalid("a reason is required")
        run = self.operational.get_run(run_id)
        if run.status in ("completed", "failed"):
            return {"run_id": run_id, "cancelled": [], "terminated_pids": [], "status": run.status}
        self._audit_write("run_cancel", run_id, reason=reason[:200])
        family = [run_id] + [r.run_id for r in self.operational.list_runs(limit=100000) if r.parent_run_id == run_id]
        pids = [p for p in (dispatch.terminate(self.rt.config.work_dir, rid) for rid in family) if p]
        cancelled = self.rt.orchestrator.cancel(run_id, reason, self.policy.actor_label)
        return {"run_id": run_id, "cancelled": cancelled, "terminated_pids": pids, "status": self.operational.get_run(run_id).status}

    def run_resume(self, run_id: str) -> dict:
        """Advance a run until it waits for an approval or evidence, blocks, fails or completes (write tool)."""
        self._require_write()
        self.operational.get_run(run_id)
        self._audit_write("run_resume", run_id)
        run = self.rt.orchestrator.advance(run_id)
        return {"snapshot": self.snapshot_used(None), "run_id": run.run_id, "run": run.as_dict(), "pending_gate": run.result.get("pending_gate")}

    def changeset_propose(self, changeset: dict, base_snapshot: str | None = None) -> dict:
        """Dry-run an o5 ChangeSet against the head and store a proposal record; never commits to the knowledge store (write tool)."""
        self._require_write()
        head = self._snap(None)
        if base_snapshot is not None:
            self._snap(base_snapshot)
        before = self.knowledge.materialize(head.id)
        report = self._dry_run(before, changeset)
        report.update(base_snapshot=base_snapshot, head=head.id, stale_base=base_snapshot is not None and base_snapshot != head.id)
        proposal = {"proposal_id": f"prop-{context.new_id(6)}", "changeset_id": changeset.get("id"), "changeset_hash": digest(changeset),
                    "changeset": changeset, "report": report, "ok": report["ok"], "head": head.id, "base_snapshot": base_snapshot,
                    "proposed_at": now_iso(), **self._origin()}
        self.operational.add_audit("changeset_proposal", proposal)
        return {"snapshot": head.id, "proposal_id": proposal["proposal_id"], "ok": report["ok"], "committed": False, "report": report}

    @staticmethod
    def _dry_run(before: dict, changeset: dict) -> dict:
        o5 = o5_path.load()
        try:
            candidate, report = o5.apply_changeset(copy.deepcopy(before), changeset)
        except Exception as exc:
            return {"ok": False, "code": "CHANGESET_MALFORMED", "errors": [f"{type(exc).__name__}: {exc}"]}
        report = dict(report)
        if report.get("ok"):
            violations = sad_violations(before, candidate)
            if violations:
                return {"ok": False, "code": "SAD_READONLY", "sad_violations": violations, "touched": report.get("touched", [])}
            report["result_hash"] = report.pop("snapshot", None)
        return report

    def proposals_list(self, proposal_id: str | None = None, limit: int = 50, offset: int = 0) -> Paged:
        """List stored ChangeSet proposals, newest first; give proposal_id to include the full changeset."""
        self._require_write()
        rows = list(reversed(self.operational.list_audit(kind="changeset_proposal", limit=1000)))
        items = [dict(r["payload"], ts=r["ts"]) for r in rows]
        if proposal_id:
            items = [p for p in items if p["proposal_id"] == proposal_id]
            if not items:
                raise NotFound(f"proposal not found: {proposal_id}", code="PROPOSAL_NOT_FOUND")
        else:
            items = [{k: v for k, v in p.items() if k != "changeset"} for p in items]
        limit = self._limit(limit, 50)
        offset = self._offset(offset)
        window = items[offset:offset + limit]

        def more(page: dict) -> None:
            if page["next"] is None and offset + limit < len(items):
                page["next"] = offset + limit

        return Paged({"snapshot": self.snapshot_used(None), "proposals": window, "total": len(items)}, parts=("proposals",), base=offset, after=more)

    # -------------------------------------------------------------- approval
    def approval_submit(self, run_id: str, gate: str, role: str, decision_hash: str, rationale: str, advance: bool = True) -> dict:
        """Approve a pending gate as the configured client identity (never a caller-chosen actor); goes through the same approval policy as the CLI.

        ``advance=false`` returns right after recording the approval; continue the run with run_dispatch."""
        if not self.policy.approvals:
            raise TddAgentError("approvals over MCP are not enabled for this client", code="APPROVALS_DISABLED")
        request = ApprovalRequest(run_id=run_id, gate=gate, actor_id=self.policy.actor, role=role, decision_hash=decision_hash,
                                  rationale=rationale, channel="mcp")
        result = self.rt.orchestrator.submit_approval(request, advance=advance)
        if not result.accepted:
            raise ApprovalRejected(result.message, code=result.code, details={"run_id": run_id, "gate": gate, "accepted": False})
        return {"snapshot": self.snapshot_used(None), "accepted": True, "code": result.code, "run_id": run_id, "gate": gate,
                "run_status": result.run_status, "approval": result.record}
