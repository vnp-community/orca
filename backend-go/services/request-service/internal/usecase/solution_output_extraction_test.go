package usecase

import (
	"testing"
)

func TestExtractJSONObject(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{
			name: "simple",
			in:   `some text {"a": 1} more text`,
			want: `{"a": 1}`,
			err:  nil,
		},
		{
			name: "code fence",
			in:   "```json\n{\"nested\": {\"b\": 2}}\n```",
			want: `{"nested": {"b": 2}}`,
			err:  nil,
		},
		{
			name: "string with braces",
			in:   `{"key": "a { string }"}`,
			want: `{"key": "a { string }"}`,
			err:  nil,
		},
		{
			name: "string with escaped quotes",
			in:   `{"key": "escaped \" }"}`,
			want: `{"key": "escaped \" }"}`,
			err:  nil,
		},
		{
			name: "prose before and after",
			in:   "Đây là kết quả:\n```\n{\"a\": [1, {\"b\": \"x\"}]}\n```\nHy vọng hữu ích.",
			want: `{"a": [1, {"b": "x"}]}`,
		},
		{
			name: "stray brace in prose is skipped",
			in:   `Use {braces} carefully: {"ok": true}`,
			want: `{"ok": true}`,
		},
		{
			name: "only the outermost object is returned",
			in:   `{"a": {"b": 1}} {"c": 2}`,
			want: `{"a": {"b": 1}}`,
		},
		{
			name: "no json",
			in:   `just text`,
			want: ``,
			err:  ErrNoJSONObject,
		},
		{
			name: "unmatched brace",
			in:   `{"incomplete"`,
			want: ``,
			err:  ErrNoJSONObject,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ExtractJSONObject(tc.in)
			if err != tc.err {
				t.Fatalf("expected err %v, got %v", tc.err, err)
			}
			if string(got) != tc.want {
				t.Errorf("expected %q, got %q", tc.want, string(got))
			}
		})
	}
}
