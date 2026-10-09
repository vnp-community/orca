"""Acceptance oracle for the 11 request types: README v6 section 3.4 / CR-REQ-003 table 2.1, written down independently of the Go registry.

first_after_confirm is the state a Request must reach once the type is confirmed and any readiness Clarification is answered.
"""
from __future__ import annotations

FULL = ["new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval",
        "planning", "awaiting_plan_approval", "executing", "completed"]

EXPECTED: dict[str, dict] = {
    "change_request": dict(first="analyzing", path=FULL, human=False, analysis="solution", gate="solution", plan="plan", start="plan", completes=False),
    "bug": dict(first="analyzing", path=FULL, human=False, analysis="diagnosis", gate="solution", plan="plan", start="plan", completes=False),
    "hotfix": dict(first="analyzing", path=["new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_plan_approval", "executing", "completed"],
                   human=True, analysis="diagnosis", gate="", plan="single_task", start="pre_deploy", completes=False, urgency="urgent"),
    "task": dict(first="planning", path=["new", "classifying", "awaiting_type_confirmation", "planning", "awaiting_plan_approval", "executing", "completed"],
                 human=False, analysis="none", gate="", plan="task_list", start="task_list", completes=False),
    "spike": dict(first="analyzing", path=["new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval", "completed"],
                  human=False, analysis="findings", gate="findings", plan="none", start="", completes=True),
    "question": dict(first="analyzing", path=["new", "classifying", "awaiting_type_confirmation", "analyzing", "awaiting_analysis_approval", "completed"],
                     human=False, analysis="answer", gate="answer", plan="none", start="", completes=True),
    "refactor": dict(first="analyzing", path=FULL, human=False, analysis="solution", gate="solution", plan="plan", start="plan", completes=False),
    "security": dict(first="analyzing", path=FULL, human=True, analysis="diagnosis", gate="solution", plan="plan", start="pre_deploy", completes=False),
    "performance": dict(first="analyzing", path=FULL, human=False, analysis="diagnosis", gate="solution", plan="plan", start="plan", completes=False),
    "docs": dict(first="planning", path=["new", "classifying", "awaiting_type_confirmation", "planning", "awaiting_plan_approval", "executing", "completed"],
                 human=False, analysis="none", gate="", plan="task_list", start="task_list", completes=False),
    "ops_request": dict(first="planning", path=["new", "classifying", "awaiting_type_confirmation", "planning", "awaiting_plan_approval", "executing", "completed"],
                        human=False, analysis="none", gate="", plan="plan", start="plan", completes=False),
}

# Sizes that exercise the Phase rule: change_request always splits, bug/refactor only at L, the rest never.
PHASES = {"change_request": (True, True), "bug": (False, True), "refactor": (False, True)}  # (size M, size L)
