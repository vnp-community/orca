package domain

import "testing"

func lane(state TestSpecStatus, code string) *TestSpecRun {
	return &TestSpecRun{State: state, ReasonCode: code}
}

func TestDecideJoin(t *testing.T) {
	cases := []struct {
		name     string
		gate     TestSpecGate
		run      *TestSpecRun
		timedOut bool
		verdict  JoinVerdict
		warn     bool
	}{
		{"no lane goes", TestSpecGateWait, nil, false, JoinGo, false},
		{"off ignores a stopped lane", TestSpecGateOff, lane(TestSpecBlocked, "X"), false, JoinGo, false},
		{"completed goes", TestSpecGateRequire, lane(TestSpecCompleted, ""), false, JoinGo, false},
		{"skipped goes", TestSpecGateRequire, lane(TestSpecSkipped, TestSpecNoDevServer), false, JoinGo, false},
		{"wait holds a generating lane", TestSpecGateWait, lane(TestSpecRunning, ""), false, JoinWait, false},
		{"require holds a starting lane", TestSpecGateRequire, lane(TestSpecStarting, ""), false, JoinWait, false},
		{"wait holds a pending lane", TestSpecGateWait, lane(TestSpecPending, ""), false, JoinWait, false},
		{"warn never holds", TestSpecGateWarn, lane(TestSpecRunning, ""), false, JoinGo, true},
		{"wait goes on after a blocked lane, with a warning", TestSpecGateWait, lane(TestSpecBlocked, TestSpecLLMMissing), false, JoinGo, true},
		{"wait goes on after a failed lane, with a warning", TestSpecGateWait, lane(TestSpecFailed, TestSpecUnreachable), false, JoinGo, true},
		{"require stops after a blocked lane", TestSpecGateRequire, lane(TestSpecBlocked, TestSpecLLMMissing), false, JoinStop, false},
		{"require stops after a cancelled lane", TestSpecGateRequire, lane(TestSpecCancelled, ""), false, JoinStop, false},
		{"wait gives up on a lane past the timeout", TestSpecGateWait, lane(TestSpecRunning, ""), true, JoinGo, true},
		{"require stops on a lane past the timeout", TestSpecGateRequire, lane(TestSpecRunning, ""), true, JoinStop, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := DecideJoin(c.gate, c.run, c.timedOut)
			if d.Verdict != c.verdict || d.Warn != c.warn {
				t.Fatalf("got %+v, want %s warn=%v", d, c.verdict, c.warn)
			}
		})
	}
	if d := DecideJoin(TestSpecGateRequire, lane(TestSpecRunning, ""), true); d.Cause != TestSpecDeadline {
		t.Fatalf("a timed-out wait names the deadline, got %q", d.Cause)
	}
}

func TestParseTestSpecGate(t *testing.T) {
	if g, err := ParseTestSpecGate(""); err != nil || g != TestSpecGateWait {
		t.Fatalf("the default is wait, got %q %v", g, err)
	}
	if _, err := ParseTestSpecGate("sometimes"); err == nil {
		t.Fatal("an unknown gate is refused")
	}
}

func TestTheLaneStartsFromPlanningOn(t *testing.T) {
	for st, want := range map[RequestStatus]bool{
		RequestStatusAnalyzing: false, RequestStatusPlanning: true,
		RequestStatusAwaitingPlanApproval: true, RequestStatusExecuting: true, RequestStatusCompleted: false,
	} {
		if TestSpecLaneStatus(st) != want {
			t.Errorf("%s: want %v", st, want)
		}
	}
}
