"""Drives one Request through the public WebSocket channels, the way the UI does."""
from __future__ import annotations

import json

from acceptance_context import AcceptanceContext, wait_until

AI_FAILURE_REASONS = ("classifier error", "no dev server connected")


def create(ctx: AcceptanceContext, title: str, body: str = "", client_request_id: str = "", project_id: str = "") -> dict:
    reply = ctx.ws.call("request.create", {"projectId": project_id or ctx.state["project_id"], "title": f"{ctx.cfg.run_prefix} {title}",
                                           "body": body or "Created by the orca e2e acceptance suite.", "clientRequestId": client_request_id})
    req = reply["request"]
    ctx.on_cleanup(f"cancel request {req['id']}", lambda rid=req["id"]: cancel_if_open(ctx, rid))
    return {**reply, "request": req}


def get(ctx: AcceptanceContext, request_id: str) -> dict:
    return ctx.ws.call("request.get", {"id": request_id})["request"]


def cancel_if_open(ctx: AcceptanceContext, request_id: str) -> None:
    req = get(ctx, request_id)
    if req["status"] not in ("completed", "cancelled"):
        ctx.ws.call("request.cancel", {"id": request_id, "reason": "e2e cleanup", "expectedVersion": req["version"]})


def settle_after_classify(ctx: AcceptanceContext, request_id: str, timeout: float = 40.0) -> dict:
    """classify is asynchronous: wait until the Request leaves `classifying` (it must land in awaiting_type_confirmation)."""
    ctx.ws.call("request.classify", {"id": request_id})
    return wait_until(lambda: (lambda r: r if r["status"] not in ("new", "classifying") else None)(get(ctx, request_id)),
                      timeout=timeout, what="classification to finish")


def confirm(ctx: AcceptanceContext, req: dict, typ: str, size: str = "M", urgency: str = "normal") -> dict:
    return ctx.ws.call("request.confirmType", {"id": req["id"], "type": typ, "size": size, "urgency": urgency, "reason": "e2e",
                                               "expectedVersion": req["version"]})["request"]


def open_clarification(ctx: AcceptanceContext, request_id: str) -> dict | None:
    items = ctx.ws.call("clarification.list", {"requestId": request_id, "status": "open"})["clarifications"]
    return items[0] if items else None


def _value_for(q: dict, text: str | None) -> object:
    if text is not None and q["kind"] == "text" and q["questionKey"] == "acceptance_criteria":
        return text  # an explicit answer wins over the service's suggested default
    if q.get("suggestedDefault") is not None:
        return q["suggestedDefault"]
    kind, key = q["kind"], q["questionKey"]
    if kind == "boolean":
        return True
    if kind in ("single_choice", "multi_choice"):
        opts = q.get("options") or []
        first = opts[0]["value"] if opts else "other"
        return [first] if kind == "multi_choice" else first
    if "hours" in key:
        return "8"  # kind is 'text' but the service validates a number written as a string
    return text or "The change is visible and verified by the e2e suite."


def answer(ctx: AcceptanceContext, clar: dict, complete: bool = True, text: str | None = None) -> dict:
    answers = [{"questionId": q["id"], "valueJson": json.dumps(_value_for(q, text)), "acceptDefault": False} for q in clar["questions"]]
    return ctx.ws.call("clarification.answer", {"clarificationId": clar["id"], "answers": answers, "complete": complete, "expectedVersion": clar["version"]})


def to_first_stage(ctx: AcceptanceContext, req: dict, typ: str, size: str = "M", urgency: str = "normal") -> dict:
    """create -> classify -> confirm -> (answer readiness Clarification). Returns the Request after those steps."""
    cur = settle_after_classify(ctx, req["id"])
    assert cur["status"] == "awaiting_type_confirmation", f"after classify the Request is {cur['status']!r}"
    cur = confirm(ctx, cur, typ, size, urgency)
    if cur["status"] == "awaiting_information":
        clar = open_clarification(ctx, cur["id"])
        assert clar, "Request is awaiting_information but no open Clarification exists"
        answer(ctx, clar)
        cur = wait_until(lambda: (lambda r: r if r["status"] != "awaiting_information" else None)(get(ctx, cur["id"])), timeout=15, what="Request to leave awaiting_information")
    return cur
