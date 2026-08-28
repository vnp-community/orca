# TASK-AT-01-01: Add `project_id`, `actions` chain, and `OnFailurePolicy` to `automation.proto`

**From Solution:** SOL-AT-01
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `automation-service`
**File:** `backend-go/proto/orca/automation/v1/automation.proto`
**Depends on:** none
**Status:** `[x]` DONE — automation.proto: project_id/actions/OnFailurePolicy added to Automation+Create/UpdateRequest, step_type/step_config_json deprecated; buf generate + go build ./proto/... clean.

---

## Context

BR-AT-02 (per-project cap) and BR-AT-01 (multi-action chains) both need new
wire fields before any domain/usecase work can compile against them. This is
additive-only — `step_type`/`step_config_json` are kept but deprecated so
pre-migration rows still round-trip.

## Changes to make

In `automation.proto`, update the `Automation` message and add the new
`AutomationAction` message and `OnFailurePolicy` enum:

```protobuf
message Automation {
  string id = 1;
  string tenant_id = 2;
  string project_id = 9;      // NEW — logical FK -> project-service.projects; empty = unscoped (back-compat)
  string name = 3;
  string rrule = 4;
  // step_type/step_config_json (fields 5/6) are DEPRECATED but left on the
  // wire for one release: UpdateAutomation on a pre-migration row without
  // `actions` set still round-trips through them. New/updated automations
  // populate `actions` instead — see AutomationAction below.
  string step_config_json = 5 [deprecated = true];
  orca.workflow.v1.StepType step_type = 6 [deprecated = true];
  bool enabled = 7;
  string dtstart = 8;
  string timezone = 10;        // renumbered from 9 to make room for project_id above
  repeated AutomationAction actions = 11; // NEW — ordered chain, BR-AT-01's schema
}

// AutomationAction is one step in an automation's ordered action chain.
message AutomationAction {
  orca.workflow.v1.StepType step_type = 1;
  string step_config_json = 2;
  // on_failure controls whether RunNow's action loop continues to the next
  // action or stops the run. Default (unspecified) = STOP.
  OnFailurePolicy on_failure = 3;
}

enum OnFailurePolicy {
  ON_FAILURE_POLICY_UNSPECIFIED = 0; // = STOP
  ON_FAILURE_POLICY_STOP = 1;
  ON_FAILURE_POLICY_CONTINUE = 2;
}
```

Also add the same `project_id` (field 9) and `repeated AutomationAction
actions` fields to `CreateAutomationRequest` and `UpdateAutomationRequest`,
mirroring `Automation`'s shape (existing convention in this proto).

Check the existing field numbering in the file before applying — renumber
`timezone` only if it currently sits at field 9; if a different number is
already in use, keep the existing number for `timezone` and pick the next
free field number for `project_id` instead, adjusting this task's numbers
accordingly.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions (deprecated
fields kept, no removed/renumbered-in-place fields).
