"""EXT: ways out of (and back into) the flow: backlog, reopen, cancel, change type, children; terminal states are final."""
from __future__ import annotations

import request_flow_driver as drv
from acceptance_context import AcceptanceContext, wait_until
from acceptance_recorder import Blocked
from ws_rpc_client import RpcError


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws
    if not ctx.state.get("project_id"):
        rec.run("EXT-00", "A project exists for the exit tests", lambda: (_ for _ in ()).throw(Blocked("no project (see DEV-04)")))
        return
    held: dict = {}

    def fresh(label: str, to_type: bool = True) -> dict:
        req = drv.create(ctx, f"exit {label}")["request"]
        cur = drv.settle_after_classify(ctx, req["id"])
        return cur

    def backlog_needs_reason():
        cur = fresh("backlog")
        held["backlog"] = cur
        try:
            ws.call("request.returnToBacklog", {"id": cur["id"], "stage": "classification", "category": "other", "reason": "", "expectedVersion": cur["version"]})
        except RpcError:
            return
        raise AssertionError("returnToBacklog without a reason was accepted")
    rec.run("EXT-01", "Returning to the backlog requires a reason", backlog_needs_reason)

    def backlog_ok():
        cur = held.get("backlog")
        if not cur:
            raise Blocked("EXT-01 did not create a Request")
        back = ws.call("request.returnToBacklog", {"id": cur["id"], "stage": "classification", "category": "other", "reason": "e2e: not now",
                                                    "expectedVersion": drv.get(ctx, cur["id"])["version"]})["request"]
        assert back["status"] == "request_backlog", f"status {back['status']!r}"
    rec.run("EXT-02", "A Request returned with a reason lands in request_backlog", backlog_ok)

    def reopen_reclassifies():
        cur = held.get("backlog")
        if not cur:
            raise Blocked("EXT-01 did not create a Request")
        now = drv.get(ctx, cur["id"])
        if now["status"] != "request_backlog":
            raise Blocked(f"EXT-02 left the Request in {now['status']!r}")
        reopened = ws.call("request.reopen", {"id": now["id"], "note": "e2e", "expectedVersion": now["version"]})["request"]
        assert reopened["status"] in ("classifying", "awaiting_type_confirmation"), f"reopened into {reopened['status']!r}, expected classification"
    rec.run("EXT-03", "Reopening a backlog Request returns it to classification", reopen_reclassifies)

    def reopen_only_from_backlog():
        cur = fresh("reopen-guard")
        try:
            ws.call("request.reopen", {"id": cur["id"], "note": "e2e", "expectedVersion": cur["version"]})
        except RpcError:
            return
        raise AssertionError("reopen was accepted for a Request that is not in the backlog")
    rec.run("EXT-04", "Reopen is only allowed from request_backlog", reopen_only_from_backlog)

    def cancel_needs_reason():
        cur = fresh("cancel")
        held["cancel"] = cur
        try:
            ws.call("request.cancel", {"id": cur["id"], "reason": "", "expectedVersion": cur["version"]})
        except RpcError:
            return
        raise AssertionError("cancel without a reason was accepted")
    rec.run("EXT-05", "Cancelling requires a reason", cancel_needs_reason)

    def cancel_ok():
        cur = held.get("cancel")
        if not cur:
            raise Blocked("EXT-05 did not create a Request")
        done = ws.call("request.cancel", {"id": cur["id"], "reason": "e2e: obsolete", "expectedVersion": drv.get(ctx, cur["id"])["version"]})["request"]
        assert done["status"] == "cancelled", f"status {done['status']!r}"
    rec.run("EXT-06", "A cancelled Request has status 'cancelled'", cancel_ok)

    def cancelled_is_terminal():
        cur = held.get("cancel")
        if not cur:
            raise Blocked("EXT-05 did not create a Request")
        now = drv.get(ctx, cur["id"])
        if now["status"] != "cancelled":
            raise Blocked("EXT-06 did not cancel the Request")
        for label, call in (("confirmType", lambda: drv.confirm(ctx, now, "task")),
                            ("reopen", lambda: ws.call("request.reopen", {"id": now["id"], "note": "x", "expectedVersion": now["version"]})),
                            ("returnToBacklog", lambda: ws.call("request.returnToBacklog", {"id": now["id"], "stage": "classification", "category": "other", "reason": "x", "expectedVersion": now["version"]}))):
            try:
                call()
            except RpcError:
                continue
            raise AssertionError(f"{label} succeeded on a cancelled Request")
    rec.run("EXT-07", "A cancelled Request accepts no further transitions", cancelled_is_terminal)

    def change_type_roundtrip():
        cur = fresh("change-type")
        first = drv.confirm(ctx, cur, "bug", "S")
        again = ws.call("request.changeType", {"id": first["id"], "toType": "change_request", "size": "S", "urgency": "normal", "reason": "e2e re-type",
                                                "expectedVersion": drv.get(ctx, first["id"])["version"]})["request"]
        assert again["status"] == "awaiting_type_confirmation", f"changeType left the Request in {again['status']!r}"
        history = ws.call("request.typeHistory", {"id": again["id"]})["changes"]
        assert history, "the type change is not recorded in the type history"
    rec.run("EXT-08", "Changing the type sends the Request back to confirmation and is recorded", change_type_roundtrip)

    def disallowed_type_change():
        cur = fresh("type-change-rule")
        first = drv.confirm(ctx, cur, "bug", "S")
        try:
            ws.call("request.changeType", {"id": first["id"], "toType": "docs", "size": "S", "urgency": "normal", "reason": "e2e",
                                            "expectedVersion": drv.get(ctx, first["id"])["version"]})
        except RpcError as exc:
            assert exc.has_code("REQUEST_TYPE_CHANGE_NOT_ALLOWED"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("bug -> docs is not a widening move but was accepted")
    rec.run("EXT-11", "Only widening type changes are allowed (bug -> docs is refused)", disallowed_type_change)

    def child_rules():
        cur = fresh("child")
        try:
            ws.call("request.spawnChild", {"id": cur["id"], "linkReason": "spawned_by_spike", "title": "child", "body": "x", "typeHint": "task", "clientRequestId": ctx.cfg.run_prefix + "-child"})
        except RpcError:
            return
        raise AssertionError("a spike-spawned child was created from a Request that is not a spike")
    rec.run("EXT-09", "Child Requests obey the parent-type rules", child_rules)

    def unknown_request():
        try:
            ws.call("request.get", {"id": "00000000-0000-0000-0000-000000000000"})
        except RpcError as exc:
            assert exc.has_code("NOT_FOUND"), f"wrong error: {exc.message[:200]}"
            return
        raise AssertionError("an unknown Request id returned a result")
    rec.run("EXT-10", "An unknown Request id is REQUEST_NOT_FOUND", unknown_request)
