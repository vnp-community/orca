package wscompat

import (
	"context"
	"encoding/json"
	"strings"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/status"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// requestFeatureBaseRef is published by request.flowStatus so a frontend shows the base branch field only when this
// gateway really serves it (CR-REQ-039). The gateway announces it: request-service does not know its own edge.
const requestFeatureBaseRef = "request.baseRef"

// requestFeatureApproveRunAll says approval.approve forwards executionMode (CR-REQ-043). An older gateway ignores the
// unknown key and would turn "approve and run all steps" into a plain approval, so a frontend reads this first.
const requestFeatureApproveRunAll = "approval.runAll"

// requestFeatures lists the optional capabilities this gateway serves; a new gateway adds a name, never removes one.
func requestFeatures() []string {
	return []string{requestFeatureBaseRef, requestFeatureApproveRunAll, requestFeaturePlanAuto, requestFeaturePlanRevisions}
}

// withBaseRefState adds what request-service works out when it reads a Request (the lock) and, for the two writes,
// what the ref check found. Other channels leave these keys out: their responses do not look up worktrees, so a
// "false" there would claim an unlocked ref that may be locked.
func (v RequestView) withBaseRefState(r *requestv1.Request, unverified *bool) RequestView {
	locked, reason := r.GetBaseRefLocked(), r.GetBaseRefLockReason()
	v.BaseRefLocked, v.BaseRefLockReason, v.BaseRefUnverified = &locked, &reason, unverified
	return v
}

// registerBaseRef adds request.setBaseRef (CONTRACT 2.11). It has no MCP tool on purpose: an agent must not move the
// base of the worktrees it will run in (see excluded_channels.yaml).
func (c requestChannels) registerBaseRef(r *Registry) {
	c.handle(r, "request.setBaseRef", requestChannelOpts{timeout: requestRPCTimeout}, c.setBaseRef)
}

func (c requestChannels) setBaseRef(ctx context.Context, _ Identity, args []json.RawMessage) (any, error) {
	in, err := decodeRequestArgs[struct {
		ID              string  `json:"id"`
		BaseRef         *string `json:"baseRef"`
		ExpectedVersion int64   `json:"expectedVersion"`
	}](args)
	if err != nil {
		return nil, err
	}
	// A missing key is an error, not "": a typo in the key name must not silently reset the ref to the default.
	if in.BaseRef == nil {
		return nil, invalidRequestArg(`baseRef is required (send "" to use the repository default)`)
	}
	resp, err := c.req.SetRequestBaseRef(ctx, &requestv1.SetRequestBaseRefRequest{
		RequestId: in.ID, BaseRef: *in.BaseRef, ExpectedVersion: in.ExpectedVersion,
	})
	if err != nil {
		return nil, err
	}
	unverified := resp.GetBaseRefUnverified()
	return requestEnvelope{RequestViewOf(resp.GetRequest(), false).withBaseRefState(resp.GetRequest(), &unverified)}, nil
}

// maxBaseRefSuggestions mirrors the service's bound so a misbehaving upstream cannot grow the error.
const maxBaseRefSuggestions = 5

// baseRefErrorSuffix carries the machine-readable part of REQUEST_BASE_REF_LOCKED (the lock reason) and
// REQUEST_BASE_REF_NOT_FOUND (suggested refs) as the ` {json}` trailer already used for retryAfterMs. The message
// stays the human text. The echoed ref is not repeated in the trailer: the client sent it.
func baseRefErrorSuffix(err error, msg string) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	for _, d := range st.Details() {
		info, ok := d.(*errdetails.ErrorInfo)
		if !ok || !strings.HasPrefix(msg, info.GetReason()+": ") {
			continue
		}
		out := map[string]any{}
		switch info.GetReason() {
		case "REQUEST_BASE_REF_LOCKED":
			if reason := info.GetMetadata()["reason"]; reason != "" {
				out["reason"] = reason
			}
		case "REQUEST_BASE_REF_NOT_FOUND":
			if s := suggestionList(info.GetMetadata()["suggestions"]); len(s) > 0 {
				out["suggestions"] = s
			}
		}
		if len(out) == 0 {
			return ""
		}
		b, merr := json.Marshal(out)
		if merr != nil {
			return ""
		}
		return " " + string(b)
	}
	return ""
}

func suggestionList(joined string) []string {
	var out []string
	for _, s := range strings.Split(joined, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
		if len(out) == maxBaseRefSuggestions {
			break
		}
	}
	return out
}
