package grpcclient

import (
	"sort"
	"strings"

	"github.com/stablyai/orca-go/services/request-service/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// codeGatewayBaseRefNotFound is what git-gateway answers when git does not know the base ref. It arrives only as the prefix of
// the status message ("WORKTREE_BASE_REF_NOT_FOUND: branch \"x\" not found; available: a, b"): apperrors sends no details.
const codeGatewayBaseRefNotFound = "WORKTREE_BASE_REF_NOT_FOUND"

// gitRefMissingPhrases are what git prints for a start point it cannot resolve; git-gateway classifies them the same way, but
// only when it can also list the branches, so the raw text can still arrive as a generic create failure.
var gitRefMissingPhrases = []string{"invalid reference", "unknown revision", "not a valid object name"}

// gatewayBaseRefMissing reports whether err says the base ref does not exist, with the branches git-gateway listed, ranked so
// the ones that look like the asked name come first. Only the explicit code counts for a NotFound status (a repo that cannot be
// found is a different problem); a generic failure counts only when it carries git's own words.
func gatewayBaseRefMissing(err error, ref string) ([]string, bool) {
	st, ok := status.FromError(err)
	if !ok {
		return nil, false
	}
	msg := st.Message()
	switch {
	case st.Code() == codes.NotFound && strings.Contains(msg, codeGatewayBaseRefNotFound):
		return rankedSuggestions(msg, ref), true
	case st.Code() == codes.Internal || st.Code() == codes.Unknown:
		lower := strings.ToLower(msg)
		for _, p := range gitRefMissingPhrases {
			if strings.Contains(lower, p) {
				return nil, true
			}
		}
	}
	return nil, false
}

// rankedSuggestions reads the "available: a, b, c" tail, puts the names that contain ref first and keeps at most five.
func rankedSuggestions(msg, ref string) []string {
	_, tail, found := strings.Cut(msg, "available: ")
	if !found {
		return nil
	}
	var names []string
	for _, n := range strings.Split(tail, ", ") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	want := strings.ToLower(ref)
	sort.SliceStable(names, func(i, j int) bool {
		return strings.Contains(strings.ToLower(names[i]), want) && !strings.Contains(strings.ToLower(names[j]), want)
	})
	if len(names) > domain.MaxBaseRefSuggestions {
		names = names[:domain.MaxBaseRefSuggestions]
	}
	return names
}
