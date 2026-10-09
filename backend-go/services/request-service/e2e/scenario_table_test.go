//go:build e2e

package e2e

import (
	"fmt"
	"testing"
	"time"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// Group is the flow family a scenario belongs to (CR-REQ-025 section 2.3); every family needs one green scenario.
type Group string

const (
	GroupFull     Group = "full"
	GroupShort    Group = "short"
	GroupHotfix   Group = "hotfix"
	GroupSpike    Group = "spike_question"
	GroupCrossCut Group = "cross_cutting"
)

// world is what a stage works on.
type world struct {
	k       *kit
	request *requestv1.Request
	size    string
	typ     string
	// approvals lists the subject types of the approvals opened so far.
	approvals []string
}

// Stage is one step of a scenario with what must hold afterwards.
type Stage struct {
	Name string
	Run  func(t *testing.T, w *world)
	// WantStatus is checked (polled) after Run; empty skips the check.
	WantStatus string
	// StatusTimeout bounds that wait; zero means the kit's default.
	StatusTimeout time.Duration
	// WantEvents are outbox subjects that must exist for the tenant after the stage.
	WantEvents []string
	// WantAudits are audit actions the audit log must have received (allowed) after the stage.
	WantAudits []string
	// WantApprovals are subject types of approvals that must exist for the Request after the stage.
	WantApprovals []string
}

type Scenario struct {
	ID     string
	Group  Group
	Type   string
	Size   string
	Stages []Stage
}

// callOK runs an RPC from a stage and fails the stage on any error.
func (w *world) callOK(err error, rpc string) {
	w.k.t.Helper()
	if err != nil {
		w.k.t.Fatalf("%s: %v", rpc, err)
	}
}

func runScenario(t *testing.T, sc Scenario) {
	k := newKit(t)
	k.enableFlow()
	w := &world{k: k, size: sc.Size, typ: sc.Type}
	for i, st := range sc.Stages {
		st := st
		t.Run(fmt.Sprintf("%02d_%s", i+1, st.Name), func(t *testing.T) {
			sub := *k
			sub.t = t
			w.k = &sub
			st.Run(t, w)
			checkStage(t, w, st)
		})
		if t.Failed() {
			return // a later stage starts from the status this one failed to reach
		}
	}
}

func checkStage(t *testing.T, w *world, st Stage) {
	t.Helper()
	k := w.k
	if st.WantStatus != "" && w.request != nil {
		w.request = k.waitStatusWithin(w.request.GetId(), st.WantStatus, st.StatusTimeout)
	}
	for _, subject := range st.WantEvents {
		subject := subject
		eventually(t, 15*time.Second, "outbox event "+subject, func() (bool, string) {
			return contains(k.outboxSubjects(), subject), fmt.Sprintf("have %v", k.outboxSubjects())
		})
	}
	for _, action := range st.WantAudits {
		action := action
		eventually(t, 10*time.Second, "audit action "+action, func() (bool, string) {
			return hasAction(k.audit(), action, "allowed"), fmt.Sprintf("have %v", k.audit())
		})
	}
	if len(st.WantApprovals) > 0 && w.request != nil {
		resp, err := k.appr.ListApprovals(k.asReporter(), &requestv1.ListApprovalsRequest{RequestId: w.request.GetId()})
		if err != nil {
			t.Fatalf("ListApprovals: %v", err)
		}
		var got []string
		for _, a := range resp.GetApprovals() {
			got = append(got, subjectName(a.GetSubjectType()))
		}
		for _, want := range st.WantApprovals {
			if !contains(got, want) {
				t.Fatalf("approval %q missing, have %v", want, got)
			}
		}
		w.approvals = got
	}
}

func subjectName(s requestv1.ApprovalSubjectType) string {
	name := s.String()
	const prefix = "APPROVAL_SUBJECT_TYPE_"
	if len(name) > len(prefix) {
		name = name[len(prefix):]
	}
	return lower(name)
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// allScenarios is the single table both the runner and the type-matrix test read.
func allScenarios() []Scenario {
	var all []Scenario
	all = append(all, flowScenarios()...)
	all = append(all, crossCuttingScenarios()...)
	return all
}
