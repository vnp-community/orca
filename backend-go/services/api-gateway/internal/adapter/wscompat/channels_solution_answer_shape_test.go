package wscompat

import (
	"context"
	"encoding/json"
	"testing"

	"google.golang.org/grpc"

	requestv1 "github.com/stablyai/orca-go/proto/gen/go/orca/request/v1"
)

// answerSolutionClient serves one Solution of kind answer, with options_json
// exactly as request-service stores it (domain.AnswerDocument.Marshal).
type answerSolutionClient struct{ *fakeRequestClient }

const storedAnswerDocument = `{"schema_version":1,"kind":"answer","answer_markdown":"## Why\nBecause **cache**.\n\n- a\n- b","confidence":0.82,` +
	`"citations":[{"ref":"src/cache.go:42","excerpt":"func Get()"}],"limitations":["only read the cache package"],` +
	`"suggested_follow_up":{"type":"change_request","title":"Add TTL"}}`

func (c answerSolutionClient) ListSolutions(ctx context.Context, in *requestv1.ListSolutionsRequest, _ ...grpc.CallOption) (*requestv1.ListSolutionsResponse, error) {
	c.rec(ctx, "ListSolutions", in)
	return &requestv1.ListSolutionsResponse{Solutions: []*requestv1.Solution{{
		Id: "s-answer", RequestId: "r1", Kind: requestv1.SolutionKind_SOLUTION_KIND_ANSWER,
		Status: requestv1.SolutionStatus_SOLUTION_STATUS_PROPOSED, OptionsJson: storedAnswerDocument, CreatedAt: fakeStamp, Version: 1,
	}}}, nil
}

// The frontend reads answer.options.answer_markdown: the gateway must hand the
// stored document over verbatim (snake_case keys, nothing renamed or dropped).
func TestSolutionList_AnswerDocumentReachesTheClientVerbatim(t *testing.T) {
	r := NewRegistry()
	client := answerSolutionClient{&fakeRequestClient{}}
	registerRequestChannels(r, client, client, nil, false)

	res, err := callRequestChannel(t, r, testRequestIdentity, context.Background(), "solution.list", `{"requestId":"r1"}`)
	if err != nil {
		t.Fatal(err)
	}
	sols, _ := res["solutions"].([]any)
	if len(sols) != 1 {
		t.Fatalf("solutions = %v", res["solutions"])
	}
	sol := sols[0].(map[string]any)
	if sol["kind"] != "answer" || sol["id"] != "s-answer" || sol["requestId"] != "r1" {
		t.Errorf("wrapper = %v", sol)
	}
	opts, ok := sol["options"].(map[string]any)
	if !ok {
		t.Fatalf("options must be the document object, got %T", sol["options"])
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(storedAnswerDocument), &want); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(opts)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("options changed on the way out:\n got %s\nwant %s", gotJSON, wantJSON)
	}
	for _, key := range []string{"answer_markdown", "confidence", "citations", "limitations", "suggested_follow_up", "schema_version", "kind"} {
		if _, ok := opts[key]; !ok {
			t.Errorf("options.%s missing", key)
		}
	}
	for _, renamed := range []string{"answerMarkdown", "answer"} {
		if _, ok := opts[renamed]; ok {
			t.Errorf("options.%s must not exist: the document is not camel-cased", renamed)
		}
	}
	if _, ok := sol["chosenOptionId"]; ok {
		t.Error("an answer has no options[] to choose from")
	}
}
