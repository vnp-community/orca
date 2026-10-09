"""NFC: an unknown id is a NotFound answer on every service, never an internal error or a silent success."""
from __future__ import annotations

from acceptance_context import AcceptanceContext
from ws_rpc_client import RpcError

UNKNOWN_ID = "00000000-0000-4000-8000-000000000000"

# (test id, channel, args, accepted not-found markers). Markers are what the gateway puts in the error text.
CASES = [
    ("NFC-01", "task.get", {"id": UNKNOWN_ID}, ("NOT_FOUND", "TASK_NOT_FOUND")),
    ("NFC-02", "task.delete", {"id": UNKNOWN_ID}, ("NOT_FOUND", "TASK_NOT_FOUND")),
    ("NFC-03", "project.get", {"projectId": UNKNOWN_ID}, ("NOT_FOUND", "PROJECT_NOT_FOUND")),
    ("NFC-08", "project.get", {"projectId": "not-a-uuid"}, ("NOT_FOUND", "PROJECT_NOT_FOUND")),
    ("NFC-04", "workflow.getExecution", {"executionId": UNKNOWN_ID}, ("NOT_FOUND", "EXECUTION_NOT_FOUND")),
    ("NFC-05", "request.get", {"id": UNKNOWN_ID}, ("REQUEST_NOT_FOUND",)),
    ("NFC-06", "clarification.get", {"clarificationId": UNKNOWN_ID}, ("REQUEST_CLARIFICATION_NOT_FOUND", "REQUEST_NOT_FOUND")),
    ("NFC-07", "approval.get", {"id": UNKNOWN_ID}, ("REQUEST_APPROVAL_NOT_FOUND", "REQUEST_NOT_FOUND")),
]
INTERNAL_MARKERS = ("INTERNAL", "Internal", "panic", "nil pointer", "SQLSTATE", "rpc error: code = Unknown")


def run(ctx: AcceptanceContext) -> None:
    rec, ws = ctx.rec, ctx.ws

    def expect_not_found(channel: str, args: dict, markers: tuple):
        def check():
            try:
                ws.call(channel, args)
            except RpcError as exc:
                text = exc.message
                assert not any(m in text for m in INTERNAL_MARKERS), f"{channel} leaked an internal failure for an unknown id: {text[:200]}"
                assert any(m in text for m in markers), f"{channel} answered {text[:200]!r}; an unknown id must be NotFound ({' / '.join(markers)})"
                return
            raise AssertionError(f"{channel} succeeded for an id that does not exist")
        return check

    for test_id, channel, args, markers in CASES:
        rec.run(test_id, f"{channel} with an unknown id is NotFound", expect_not_found(channel, args, markers))
