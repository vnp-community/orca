package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/scm-integration-service/internal/domain"
)

func TestParseMergeEvent_GitHubClosedAndMerged(t *testing.T) {
	parsed, isMerge := parseMergeEvent(domain.ScmProviderGitHub, githubMergedPRPayload("octocat/hello-world", 7))
	if !isMerge {
		t.Fatal("expected a closed+merged pull_request event to be recognized as a merge")
	}
	if parsed.Repo != "octocat/hello-world" || parsed.PRNumber != 7 {
		t.Fatalf("unexpected parsed event: %+v", parsed)
	}
}

func TestParseMergeEvent_GitHubClosedButNotMerged(t *testing.T) {
	body := []byte(`{"action":"closed","number":7,"pull_request":{"number":7,"merged":false}}`)
	_, isMerge := parseMergeEvent(domain.ScmProviderGitHub, body)
	if isMerge {
		t.Fatal("expected a closed-but-not-merged pull_request event to NOT be a merge")
	}
}

func TestParseMergeEvent_GitHubOpenedIsNotAMerge(t *testing.T) {
	_, isMerge := parseMergeEvent(domain.ScmProviderGitHub, githubOpenedPRPayload("o/r", 1))
	if isMerge {
		t.Fatal("expected an opened event to NOT be a merge")
	}
}

func TestParseMergeEvent_GitLabMerge(t *testing.T) {
	body := []byte(`{
		"object_kind": "merge_request",
		"project": {"path_with_namespace": "group/project"},
		"object_attributes": {"action": "merge", "iid": 15}
	}`)
	parsed, isMerge := parseMergeEvent(domain.ScmProviderGitLab, body)
	if !isMerge {
		t.Fatal("expected a merge_request event with action=merge to be recognized as a merge")
	}
	if parsed.Repo != "group/project" || parsed.PRNumber != 15 {
		t.Fatalf("unexpected parsed event: %+v", parsed)
	}
}

func TestParseMergeEvent_GitLabNonMergeAction(t *testing.T) {
	body := []byte(`{"object_kind":"merge_request","object_attributes":{"action":"open","iid":15}}`)
	_, isMerge := parseMergeEvent(domain.ScmProviderGitLab, body)
	if isMerge {
		t.Fatal("expected a non-merge action to NOT be a merge")
	}
}

func TestParseMergeEvent_MalformedBodyIsNotAMerge(t *testing.T) {
	_, isMerge := parseMergeEvent(domain.ScmProviderGitHub, []byte("not json"))
	if isMerge {
		t.Fatal("expected a malformed body to NOT be a merge")
	}
}

func TestParseMergeEvent_UnsupportedProviderIsNotAMerge(t *testing.T) {
	_, isMerge := parseMergeEvent(domain.ScmProviderBitbucket, githubMergedPRPayload("o/r", 1))
	if isMerge {
		t.Fatal("expected an unsupported provider to NOT be a merge")
	}
}
