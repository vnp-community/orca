package wscompat

import (
	"context"
	"testing"
)

const approveArgs = `{"id":"a1","expectedVersion":2,"expectedDigest":"dg","comment":"ok"`

func TestApprovalApprove_ForwardsExecutionModeAndReturnsThePinnedOne(t *testing.T) {
	fake := &fakeRequestClient{}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.approve", approveArgs+`,"executionMode":"continuous"}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, fake.last().rpc, "Approve")
	if got := fake.last().in.(interface{ GetExecutionMode() string }).GetExecutionMode(); got != "continuous" {
		t.Errorf("forwarded mode = %q", got)
	}
	mustEqual(t, res["executionMode"], "continuous")
}

// The gateway does not judge the value: request-service owns the list and answers REQUEST_EXECUTION_MODE_INVALID.
func TestApprovalApprove_ForwardsAnyModeVerbatim(t *testing.T) {
	fake := &fakeRequestClient{}
	if _, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.approve", approveArgs+`,"executionMode":" Step_By_Step "}`); err != nil {
		t.Fatal(err)
	}
	if got := fake.last().in.(interface{ GetExecutionMode() string }).GetExecutionMode(); got != " Step_By_Step " {
		t.Errorf("forwarded mode = %q", got)
	}
}

func TestApprovalApprove_WithoutAModeHasNoExecutionModeKey(t *testing.T) {
	fake := &fakeRequestClient{}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.approve", approveArgs+`}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, has := res["executionMode"]; has {
		t.Errorf("result = %v, want no executionMode key", res)
	}
}

func TestApprovalReject_RefusesAnExecutionModeWithoutCallingTheService(t *testing.T) {
	for _, mode := range []string{"continuous", "step_by_step", "x"} {
		fake := &fakeRequestClient{}
		_, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.reject",
			`{"id":"a1","expectedVersion":2,"expectedDigest":"dg","comment":"no","executionMode":"`+mode+`"}`)
		want := "REQUEST_EXECUTION_MODE_INVALID: executionMode applies to approve only"
		if err == nil || err.Error() != want {
			t.Errorf("mode %q: err = %v, want %q", mode, err, want)
		}
		if fake.count() != 0 {
			t.Errorf("mode %q: the service must not be called, got %d calls", mode, fake.count())
		}
	}
}

func TestApprovalReject_EmptyExecutionModeIsAPlainRejection(t *testing.T) {
	fake := &fakeRequestClient{}
	res, err := callRequestChannel(t, newRequestTestRegistry(fake), testRequestIdentity, context.Background(), "approval.reject",
		`{"id":"a1","expectedVersion":2,"expectedDigest":"dg","comment":"no","executionMode":""}`)
	if err != nil {
		t.Fatal(err)
	}
	mustEqual(t, fake.last().rpc, "Reject")
	if _, has := res["executionMode"]; has {
		t.Errorf("a rejection never reports a mode: %v", res)
	}
}
