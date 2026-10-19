package wscompat

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"google.golang.org/grpc"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

type rpcService struct {
	desc  *grpc.ServiceDesc
	table map[string]rpcDisposition
}

func requestRPCServices() []rpcService {
	return []rpcService{
		{&requestv1.RequestService_ServiceDesc, requestServiceRPCChannels},
		{&requestv1.ApprovalService_ServiceDesc, approvalServiceRPCChannels},
		{&requestv1.ApprovalPolicyAdminService_ServiceDesc, approvalPolicyAdminRPCChannels},
	}
}

func registeredRequestChannels() map[string]bool {
	r := NewRegistry()
	registerRequestChannels(r, nil, nil, nil, true)
	out := map[string]bool{}
	for _, c := range r.Channels() {
		out[c.Name] = true
	}
	return out
}

// requestChannelPrefixes are the channel families request-service owns; a registered
// channel in these families must be justified by an RPC entry.
var requestChannelPrefixes = []string{"request.", "solution.", "approval.", "clarification.", "backlog."}

// A channel that does not call an RPC of its own (events, streams, generic
// backlog views over ListBacklog) is listed here with the reason.
var requestChannelsWithoutRPC = map[string]string{
	"request.subscribe": "push stream fed by the event bus, no unary RPC",
	"backlog.tasks":     "task view over ListBacklog (backlog.requests is the mapped entry)",
	"backlog.execute":   "starts task execution through the existing backlog service path",
}

func checkRequestRPCParity(services []rpcService, registered map[string]bool) []string {
	var problems []string
	claimed := map[string]bool{}
	for _, s := range services {
		seen := map[string]bool{}
		for _, m := range s.desc.Methods {
			seen[m.MethodName] = true
			d, ok := s.table[m.MethodName]
			switch {
			case !ok:
				problems = append(problems, fmt.Sprintf("%s.%s has no channel and no exclusion entry", s.desc.ServiceName, m.MethodName))
			case d.channel == "" && strings.TrimSpace(d.exclude) == "":
				problems = append(problems, fmt.Sprintf("%s.%s is excluded without a reason", s.desc.ServiceName, m.MethodName))
			case d.channel != "" && !registered[d.channel]:
				problems = append(problems, fmt.Sprintf("%s.%s maps to channel %q which is not registered", s.desc.ServiceName, m.MethodName, d.channel))
			case d.channel != "":
				claimed[d.channel] = true
			}
		}
		for _, st := range s.desc.Streams {
			seen[st.StreamName] = true
			if _, ok := s.table[st.StreamName]; !ok {
				problems = append(problems, fmt.Sprintf("%s.%s (stream) has no entry", s.desc.ServiceName, st.StreamName))
			}
		}
		for name := range s.table {
			if !seen[name] {
				problems = append(problems, fmt.Sprintf("table lists %s.%s which is not an RPC", s.desc.ServiceName, name))
			}
		}
	}
	for ch := range registered {
		if claimed[ch] || requestChannelsWithoutRPC[ch] != "" || !hasRequestPrefix(ch) {
			continue
		}
		problems = append(problems, fmt.Sprintf("channel %q is registered but no RPC maps to it", ch))
	}
	sort.Strings(problems)
	return problems
}

func hasRequestPrefix(ch string) bool {
	for _, p := range requestChannelPrefixes {
		if strings.HasPrefix(ch, p) {
			return true
		}
	}
	return false
}

func TestRequestRPCChannelParity(t *testing.T) {
	if problems := checkRequestRPCParity(requestRPCServices(), registeredRequestChannels()); len(problems) > 0 {
		t.Fatalf("Request RPC <-> WS channel parity broken:\n  %s", strings.Join(problems, "\n  "))
	}
}

func TestRequestRPCChannelParityCatchesDrift(t *testing.T) {
	registered := registeredRequestChannels()

	t.Run("new RPC without an entry", func(t *testing.T) {
		desc := requestv1.RequestService_ServiceDesc
		desc.Methods = append(append([]grpc.MethodDesc{}, desc.Methods...), grpc.MethodDesc{MethodName: "BrandNewRPC"})
		got := checkRequestRPCParity([]rpcService{{&desc, requestServiceRPCChannels}}, registered)
		if !containsProblem(got, "BrandNewRPC has no channel and no exclusion entry") {
			t.Fatalf("drift not reported: %v", got)
		}
	})
	t.Run("mapped channel not registered", func(t *testing.T) {
		got := checkRequestRPCParity(requestRPCServices(), withoutChannel(registered, "clarification.answer"))
		if !containsProblem(got, `maps to channel "clarification.answer" which is not registered`) {
			t.Fatalf("missing channel not reported: %v", got)
		}
	})
	t.Run("registered channel without RPC", func(t *testing.T) {
		extra := withoutChannel(registered, "")
		extra["request.ghost"] = true
		if got := checkRequestRPCParity(requestRPCServices(), extra); !containsProblem(got, `"request.ghost" is registered but no RPC maps to it`) {
			t.Fatalf("orphan channel not reported: %v", got)
		}
	})
	t.Run("exclusion needs a reason", func(t *testing.T) {
		table := map[string]rpcDisposition{}
		for k, v := range requestServiceRPCChannels {
			table[k] = v
		}
		table["EraseRequest"] = rpcDisposition{}
		got := checkRequestRPCParity([]rpcService{{&requestv1.RequestService_ServiceDesc, table}}, registered)
		if !containsProblem(got, "EraseRequest is excluded without a reason") {
			t.Fatalf("reasonless exclusion not reported: %v", got)
		}
	})
}

func withoutChannel(in map[string]bool, drop string) map[string]bool {
	out := map[string]bool{}
	for k := range in {
		if k != drop {
			out[k] = true
		}
	}
	return out
}

func containsProblem(problems []string, want string) bool {
	for _, p := range problems {
		if strings.Contains(p, want) {
			return true
		}
	}
	return false
}
