package domain

import (
	"context"
	"errors"
	"testing"
	"unicode/utf8"
)

type transientErr struct{}

func (transientErr) Error() string   { return "peer gone" }
func (transientErr) Transient() bool { return true }

func TestFailureClass_Valid(t *testing.T) {
	for _, c := range []FailureClass{FailureRetryable, FailureNeedsInfo, FailureSpecDefect, FailureEnvDefect, FailureAgentDefect} {
		if !c.Valid() {
			t.Errorf("%s invalid", c)
		}
	}
	if FailureClass("").Valid() || FailureClass("x").Valid() {
		t.Error("bad class accepted")
	}
}

func TestParseStatus_Valid(t *testing.T) {
	if !ParseStatusOK.Valid() || !ParseStatusMissing.Valid() || !ParseStatusInvalid.Valid() || ParseStatus("x").Valid() {
		t.Error("ParseStatus.Valid wrong")
	}
}

func TestClassifyRunFailure_Table(t *testing.T) {
	zero, one := 0, 1
	res := func(s ResultStatus) ParsedExecution {
		return ParsedExecution{Status: ParseStatusOK, Result: &ExecutionResult{Status: s}}
	}
	cases := []struct {
		name     string
		timedOut bool
		exit     *int
		p        ParsedExecution
		relay    error
		class    FailureClass
		code     string
	}{
		{"timeout", true, nil, ParsedExecution{}, nil, FailureRetryable, "TIMED_OUT"},
		{"relay transient", false, nil, ParsedExecution{}, transientErr{}, FailureRetryable, "RELAY_UNAVAILABLE"},
		{"relay deadline", false, nil, ParsedExecution{}, context.DeadlineExceeded, FailureRetryable, "RELAY_UNAVAILABLE"},
		{"relay other", false, nil, ParsedExecution{}, errors.New("no connection"), FailureEnvDefect, "RELAY_FAILED"},
		{"missing block", false, &zero, ParsedExecution{Status: ParseStatusMissing}, nil, FailureAgentDefect, ResultCodeBlockMissing},
		{"invalid block", false, &zero, ParsedExecution{Status: ParseStatusInvalid, Code: ResultCodeSchema}, nil, FailureAgentDefect, ResultCodeSchema},
		{"needs_info", false, &zero, res(ResultNeedsInfo), nil, FailureNeedsInfo, "RESULT_NEEDS_INFO"},
		{"blocked", false, &zero, res(ResultBlocked), nil, FailureNeedsInfo, "RESULT_BLOCKED"},
		{"failed", false, &zero, res(ResultFailed), nil, FailureAgentDefect, "RESULT_FAILED"},
		{"exit nonzero with done", false, &one, res(ResultDone), nil, FailureAgentDefect, "EXIT_NONZERO_WITH_DONE"},
		{"exit unknown", false, nil, res(ResultDone), nil, FailureAgentDefect, "EXIT_MISSING"},
		{"clean", false, &zero, res(ResultDone), nil, "", ""},
	}
	for _, c := range cases {
		class, code := ClassifyRunFailure(c.timedOut, c.exit, c.p, c.relay)
		if class != c.class || code != c.code {
			t.Errorf("%s: got (%s,%s) want (%s,%s)", c.name, class, code, c.class, c.code)
		}
	}
}

func TestTailUTF8_CutsOnRuneBoundary(t *testing.T) {
	s := "ab" + "ễ" + "😀" + "cd" // ễ is 3 bytes, 😀 is 4
	for max := 0; max <= len(s)+2; max++ {
		got := TailUTF8(s, max)
		if !utf8.ValidString(got) || len(got) > max {
			t.Fatalf("max=%d: %q invalid or too long", max, got)
		}
	}
	if got := TailUTF8(s, 6); got != "😀cd" {
		t.Fatalf("got %q", got)
	}
}

func TestTailUTF8_ShortStringUnchanged(t *testing.T) {
	if got := TailUTF8("xin chào", 100); got != "xin chào" {
		t.Fatal(got)
	}
	if got := TailUTF8("a\x00b\xff", 100); got != "ab" {
		t.Fatalf("NUL and invalid bytes must go, got %q", got)
	}
}

func TestStorableJSON(t *testing.T) {
	if StorableJSON([]byte(`{"a":1}`)) == nil || StorableJSON([]byte(`{"a":`)) != nil || StorableJSON([]byte(`{"a":"\u0000"}`)) != nil || StorableJSON(nil) != nil {
		t.Fatal("StorableJSON wrong")
	}
}
