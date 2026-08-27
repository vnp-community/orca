# TASK-AT-03-01: Add `trigger_type`/`trigger_event`/`trigger_filter_json` to `automation.proto`

**From Solution:** SOL-AT-03
**Priority:** P0 — everything else in this solution depends on generated stubs from this
**Service:** `automation-service`
**File:** `backend-go/proto/orca/automation/v1/automation.proto`
**Depends on:** none
**Status:** `[ ]` TODO

---

## Context

BL-AT-03's `trigger: {type: cron | manual | event, event?: string}` schema
has no wire representation today. This adds the three new fields and a
`TriggerType` enum, additive-only.

## Changes to make

In `automation.proto`, add to `Automation` (pick the next free field
numbers after checking SOL-AT-01's additions if that solution's proto task
has already landed; otherwise continue from the current highest field
number):

```protobuf
message Automation {
  // ... existing fields ...
  TriggerType trigger_type = 12;      // NEW
  string trigger_event = 13;          // NEW — one of the 5 event names below; empty unless trigger_type=EVENT
  string trigger_filter_json = 14;    // NEW — BR-AT-09, e.g. {"agent":"claude"}; empty = no filter (always matches)
}

enum TriggerType {
  TRIGGER_TYPE_UNSPECIFIED = 0; // = CRON, for back-compat with existing rrule-only rows
  TRIGGER_TYPE_CRON = 1;
  TRIGGER_TYPE_MANUAL = 2;      // rrule/dtstart still stored but next_run_at never advances — RunNow-only
  TRIGGER_TYPE_EVENT = 3;
}
```

Mirror the same three fields onto `CreateAutomationRequest` and
`UpdateAutomationRequest`.

## Verify

```bash
cd /opt/repos/orca/backend-go
buf generate proto
buf breaking proto --against '.git#branch=main'
go build ./proto/...
```

Expected: clean build; `buf breaking` reports only additions.
