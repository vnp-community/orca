#!/usr/bin/env python3
"""Small atomic ChangeSet engine for the v0.5 contract."""
from __future__ import annotations
from copy import deepcopy
from hashlib import sha256
from envelope5 import validate_graph

def snapshot_hash(graph: dict) -> str:
    body = {"version": graph.get("version"), "elements": graph.get("elements", []), "edges": graph.get("edges", [])}
    return sha256(__import__("json").dumps(body, sort_keys=True, ensure_ascii=False).encode()).hexdigest()

def apply_changeset(graph: dict, changeset: dict) -> tuple[dict, dict]:
    expected = changeset.get("base_version")
    if expected is not None and expected != graph.get("version"):
        return graph, {"ok": False, "code": "VERSION_CONFLICT", "expected": expected, "actual": graph.get("version")}
    candidate = deepcopy(graph); elements = {x["id"]: x for x in candidate.get("elements", [])}; touched = set()
    try:
        for op in sorted(changeset.get("ops", []), key=lambda x: x.get("seq", 0)):
            kind, target = op.get("op"), op.get("target")
            if kind == "add":
                x = deepcopy(op["after"])
                if x["id"] in elements: raise ValueError("DUPLICATE_ID")
                elements[x["id"]] = x; candidate["elements"].append(x); touched.add(x["id"])
            elif kind == "modify":
                if target not in elements: raise ValueError("MISSING_TARGET")
                path = op["path"]; container = elements[target]
                parts = path.split(".")
                for part in parts[:-1]:
                    if part not in container or not isinstance(container[part], dict): raise ValueError("MISSING_PROPERTY_CONTAINER")
                    container = container[part]
                old = container.get(parts[-1])
                if old != op.get("before"): raise ValueError("PRECONDITION_FAILED")
                container[parts[-1]] = deepcopy(op.get("after")); touched.add(target)
                elements[target].setdefault("change_history", []).append({"change_set": changeset.get("id"), "base_version": expected, "path": path, "before": deepcopy(old), "after": deepcopy(op.get("after")), "provenance": deepcopy(op.get("provenance", [])), "policy_resolution": deepcopy(op.get("policy_resolution"))})
                for ev in op.get("provenance", []):
                    if ev not in elements[target].setdefault("provenance", []): elements[target]["provenance"].append(deepcopy(ev))
            elif kind == "remove":
                if target not in elements: raise ValueError("MISSING_TARGET")
                if op.get("cascade") not in ("restrict", "cascade", "detach", "redirect"): raise ValueError("CASCADE_POLICY_REQUIRED")
                incoming = [e for e in candidate.get("edges", []) if e.get("dst") == target]
                if incoming and op["cascade"] == "restrict": raise ValueError("REFERENCED_TARGET")
                candidate["edges"] = [e for e in candidate.get("edges", []) if e.get("src") != target and e.get("dst") != target]
                candidate["elements"] = [x for x in candidate["elements"] if x["id"] != target]; elements.pop(target); touched.add(target)
            elif kind == "link":
                edge = deepcopy(op["after"])
                if edge["src"] not in elements or edge["dst"] not in elements: raise ValueError("MISSING_ENDPOINT")
                candidate.setdefault("edges", []).append(edge); touched.update((edge["src"], edge["dst"]))
            elif kind == "unlink":
                edge = op["before"]; before = len(candidate.get("edges", []))
                candidate["edges"] = [e for e in candidate.get("edges", []) if e != edge]
                if len(candidate["edges"]) == before: raise ValueError("MISSING_EDGE")
            elif kind == "supersede":
                if target not in elements or op.get("after") not in elements: raise ValueError("MISSING_TARGET")
                candidate.setdefault("edges", []).append({"src": op["after"], "rel": "supersedes", "dst": target}); touched.update((target, op["after"]))
            elif kind == "merge":
                source, dest = target, op.get("after")
                if source not in elements or dest not in elements: raise ValueError("MISSING_TARGET")
                for e in candidate.get("edges", []):
                    if e["src"] == source: e["src"] = dest
                    if e["dst"] == source: e["dst"] = dest
                candidate["elements"] = [x for x in candidate["elements"] if x["id"] != source]; elements.pop(source); touched.add(dest)
            else: raise ValueError("UNSUPPORTED_OPERATION")
        result = validate_graph(candidate)
        if not result["ok"]: return graph, {"ok": False, "code": "VALIDATION_FAILED", "errors": result["errors"]}
        candidate["version"] = changeset.get("result_version", candidate.get("version"))
        # Mark direct consumers stale/review-needed in the same committed snapshot.
        adjacency = {}
        for edge in candidate.get("edges", []):
            adjacency.setdefault(edge.get("src"), set()).add(edge.get("dst")); adjacency.setdefault(edge.get("dst"), set()).add(edge.get("src"))
        affected = set(touched); frontier = list(touched)
        while frontier:
            next_frontier = []
            for current in frontier:
                for neighbor in adjacency.get(current, ()):
                    if neighbor not in affected: affected.add(neighbor); next_frontier.append(neighbor)
            frontier = next_frontier
        for x in candidate.get("elements", []):
            if x.get("id") in affected and x.get("id") not in touched and x.get("review_status") == "approved": x["review_status"] = "needs_review"
        for x in candidate.get("elements", []):
            if x.get("id") in touched and x.get("review_status") == "approved": x["review_status"] = "needs_review"
        return candidate, {"ok": True, "touched": sorted(touched), "snapshot": snapshot_hash(candidate)}
    except (KeyError, ValueError) as exc:
        return graph, {"ok": False, "code": str(exc)}
