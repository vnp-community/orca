package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"

	gitgatewayv1 "github.com/stablyai/orca-go/proto/gen/go/orca/gitgateway/v1"
)

func TestRedactionGolden(t *testing.T) {
	cases := map[string]string{
		"https://u:ghp_abcdefghijklmnopqrstuvwxyz0123456789@github.com/o/r.git": "https://***@github.com/o/r.git",
		"https://oauth2:glpat-abcdefghijklmnopqrst@gitlab.com/o/r.git":          "https://***@gitlab.com/o/r.git",
		"git@github.com:o/r.git":                                               "git@github.com:o/r.git",
		"token ghp_abcdefghijklmnopqrstuvwxyz0123456789 end":                   "token *** end",
		"github_pat_11ABCDEFG0abcdefghijklmnop_qrstuvwxyz":                     "***",
		"key AKIAABCDEFGHIJKLMNOP x":                                           "key *** x",
		"Authorization: Bearer abcdefghijklmnop1234567890":                     "Authorization: Bearer ***",
		"xoxb-1234567890-abcdefghij":                                           "***",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIE\n-----END RSA PRIVATE KEY-----": "***",
		"plain text with ghp_short":                                            "plain text with ghp_short",
	}
	for in, want := range cases {
		if got := redactString(in); got != want {
			t.Errorf("redact(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSecretKeysAreMasked(t *testing.T) {
	obj, err := normalizeResult(map[string]any{
		"name": "x", "credentialRef": "vault:abc", "nested": map[string]any{"apiKey": "k", "tokenCount": 3, "password": "p"},
		"list": []any{map[string]any{"access_token": "t"}},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(obj)
	s := string(b)
	for _, leak := range []string{"vault:abc", `"k"`, `"p"`, `"t"`} {
		if strings.Contains(s, leak) {
			t.Errorf("leaked %s in %s", leak, s)
		}
	}
	if !strings.Contains(s, `"tokenCount":3`) || !strings.Contains(s, `"name":"x"`) {
		t.Errorf("benign fields were altered: %s", s)
	}
}

func TestNormalizeSnakeToCamelAndProto(t *testing.T) {
	obj, _ := normalizeResult(map[string]any{"worktree_id": "w", "inner": map[string]any{"head_oid": "a"}}, nil)
	b, _ := json.Marshal(obj)
	if string(b) != `{"inner":{"headOid":"a"},"worktreeId":"w"}` {
		t.Errorf("got %s", b)
	}
	// proto message: protojson camelCase
	obj, err := normalizeResult(&gitgatewayv1.ReadFilePreviewResponse{Content: []byte("hi"), Truncated: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if obj["truncated"] != true || obj["content"] != "aGk=" {
		t.Errorf("proto result %v", obj)
	}
	// arrays are wrapped
	obj, _ = normalizeResult([]string{"a"}, nil)
	if _, ok := obj["items"]; !ok {
		t.Errorf("array not wrapped: %v", obj)
	}
}

func TestRemoteURLTokenRedactedInResult(t *testing.T) {
	obj, _ := normalizeResult(map[string]any{"url": "https://me:ghp_abcdefghijklmnopqrstuvwxyz0123456789@github.com/o/r/commit/1"}, nil)
	if obj["url"] != "https://***@github.com/o/r/commit/1" {
		t.Errorf("got %v", obj["url"])
	}
}

func TestTruncationKeepsValidJSON(t *testing.T) {
	items := make([]any, 500)
	for i := range items {
		items[i] = map[string]any{"i": i, "text": strings.Repeat("é", 400)}
	}
	spec := &ToolSpec{MaxResultBytes: 4096}
	obj, err := normalizeResult(items, spec)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(obj)
	if len(b) > 4096 {
		t.Fatalf("size %d exceeds limit", len(b))
	}
	if !json.Valid(b) || obj["truncated"] != true {
		t.Errorf("not valid/truncated: %s", b[:80])
	}
	// one huge string is cut on a rune boundary
	obj, _ = normalizeResult(map[string]any{"diff": strings.Repeat("日本語", 5000)}, &ToolSpec{MaxResultBytes: 2048})
	b, _ = json.Marshal(obj)
	if len(b) > 2048 || !json.Valid(b) {
		t.Errorf("string truncation failed: %d", len(b))
	}
	// small results are untouched
	obj, _ = normalizeResult(map[string]any{"a": 1}, spec)
	if _, ok := obj["truncated"]; ok {
		t.Error("small result must not be marked truncated")
	}
}

func TestUntrustedLabelAndFilePreview(t *testing.T) {
	obj, _ := normalizeResult(map[string]any{"title": "x"}, &ToolSpec{Untrusted: true})
	if obj["untrusted"] != true {
		t.Error("missing untrusted label")
	}
	s := specByName(t, "files_read")
	text, _ := normalizeResult(&gitgatewayv1.ReadFilePreviewResponse{Content: []byte("hello")}, s)
	if text["text"] != "hello" || text["content"] != nil {
		t.Errorf("text preview %v", text)
	}
	bin, _ := normalizeResult(&gitgatewayv1.ReadFilePreviewResponse{Content: []byte{0xff, 0xfe, 0x00}}, s)
	if bin["binary"] != true || bin["content"] != nil {
		t.Errorf("binary preview %v", bin)
	}
	_ = wscompat.Identity{}
}

func FuzzNormalizeResult(f *testing.F) {
	f.Add(`{"a":[1,2,{"b":"c"}]}`)
	f.Add(`"x"`)
	f.Fuzz(func(t *testing.T, in string) {
		var v any
		if json.Unmarshal([]byte(in), &v) != nil {
			return
		}
		obj, err := normalizeResult(v, &ToolSpec{MaxResultBytes: 512})
		if err != nil {
			return
		}
		if b, err := json.Marshal(obj); err != nil || !json.Valid(b) {
			t.Fatalf("invalid output for %q", in)
		}
	})
}
