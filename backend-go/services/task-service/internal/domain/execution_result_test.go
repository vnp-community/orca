package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const testNonce = "abcdef0123456789XYZ"

func block(nonce, body string) string {
	return fmt.Sprintf("ORCA_RESULT_BEGIN %s\n%s\nORCA_RESULT_END %s\n", nonce, body, nonce)
}

const okBody = `{"schema_version":1,"status":"done","summary":"ok","files_changed":["a.go"],"checks_run":[{"id":"c1","exit":0}],"outputs":{"files":["a.go"]}}`

func TestFindResultBlock_Table(t *testing.T) {
	big := strings.Repeat("x", MaxResultBlockBytes)
	cases := []struct {
		name, stdout, nonce, want, code string
	}{
		{"right nonce", "log\n" + block(testNonce, `{"a":1}`), testNonce, `{"a":1}`, ""},
		{"wrong nonce", block("zzzzzzzzzzzzzzzzzz", `{"a":1}`), testNonce, "", ResultCodeBlockMissing},
		{"last block wins", block(testNonce, `{"a":1}`) + block(testNonce, `{"a":2}`), testNonce, `{"a":2}`, ""},
		{"begin without end", "ORCA_RESULT_BEGIN " + testNonce + "\n{}", testNonce, "", ResultCodeBlockMissing},
		{"end before begin", "ORCA_RESULT_END " + testNonce + "\nORCA_RESULT_BEGIN " + testNonce + "\n{}", testNonce, "", ResultCodeBlockMissing},
		{"empty nonce", block("", `{}`), "", "", ResultCodeBlockMissing},
		{"short nonce", block("abc", `{}`), "abc", "", ResultCodeBlockMissing},
		{"marker without nonce ignored", "ORCA_RESULT_BEGIN\n{\"x\":1}\nORCA_RESULT_END\n" + block(testNonce, `{"a":1}`), testNonce, `{"a":1}`, ""},
		{"quoted inline marker not a line", "say ORCA_RESULT_BEGIN " + testNonce + " then {} ORCA_RESULT_END " + testNonce + "\n", testNonce, "", ResultCodeBlockMissing},
		{"nested begin uses nearest", "ORCA_RESULT_BEGIN " + testNonce + "\nORCA_RESULT_BEGIN " + testNonce + "\n{\"in\":1}\nORCA_RESULT_END " + testNonce, testNonce, `{"in":1}`, ""},
		{"vietnamese", block(testNonce, `{"s":"Nguyễn"}`), testNonce, `{"s":"Nguyễn"}`, ""},
		{"crlf", "ORCA_RESULT_BEGIN " + testNonce + "\r\n{\"a\":1}\r\nORCA_RESULT_END " + testNonce + "\r\n", testNonce, `{"a":1}`, ""},
		{"exactly 256KiB", block(testNonce, big), testNonce, big, ""},
		{"256KiB+1", block(testNonce, big+"x"), testNonce, "", ResultCodeTooLarge},
		{"empty stdout", "", testNonce, "", ResultCodeBlockMissing},
	}
	for _, c := range cases {
		body, code := FindResultBlock(c.stdout, c.nonce)
		if string(body) != c.want || code != c.code {
			t.Errorf("%s: got (%.40q, %q), want (%.40q, %q)", c.name, body, code, c.want, c.code)
		}
	}
}

func TestParseExecutionResult_FallsBackToStdoutWhenNoParsed(t *testing.T) {
	p := ParseExecutionResult(block(testNonce, okBody), testNonce, nil)
	if p.Status != ParseStatusOK || p.Result == nil || p.Result.Status != ResultDone || len(p.Raw) == 0 {
		t.Fatalf("unexpected: %+v", p)
	}
}

func TestParseExecutionResult_UsesAgentParsedWhenPresent(t *testing.T) {
	p := ParseExecutionResult("garbage without any block", testNonce, &AgentParsed{OK: true, Value: json.RawMessage(okBody)})
	if p.Status != ParseStatusOK {
		t.Fatalf("agent value must be used: %+v", p)
	}
}

func TestParseExecutionResult_AgentCodeMissingMapsToMissing(t *testing.T) {
	p := ParseExecutionResult("", testNonce, &AgentParsed{Code: ResultCodeBlockMissing})
	if p.Status != ParseStatusMissing {
		t.Fatalf("got %+v", p)
	}
	p = ParseExecutionResult("", testNonce, &AgentParsed{Code: "RESULT_BLOCK_NOT_OBJECT"})
	if p.Status != ParseStatusInvalid || p.Code != "RESULT_BLOCK_NOT_OBJECT" {
		t.Fatalf("got %+v", p)
	}
}

func TestParseExecutionResult_InvalidJSON(t *testing.T) {
	p := ParseExecutionResult(block(testNonce, `{"a":`), testNonce, nil)
	if p.Status != ParseStatusInvalid || p.Code != ResultCodeInvalidJSON {
		t.Fatalf("got %+v", p)
	}
	p = ParseExecutionResult(block(testNonce, `[1]`), testNonce, nil)
	if p.Code != ResultCodeNotObject {
		t.Fatalf("got %+v", p)
	}
}

func TestParseExecutionResult_SchemaInvalid(t *testing.T) {
	long := strings.Repeat("x", MaxResultSummaryBytes+1)
	many := strings.Repeat(`"a",`, MaxResultListEntries) + `"a"`
	bigOut := `"` + strings.Repeat("y", MaxResultOutputsBytes) + `"`
	cases := map[string]string{
		"version":         `{"schema_version":2,"status":"done"}`,
		"status":          `{"schema_version":1,"status":"weird"}`,
		"summary":         `{"schema_version":1,"status":"done","summary":"` + long + `"}`,
		"files over 200":  `{"schema_version":1,"status":"done","files_changed":[` + many + `]}`,
		"outputs size":    `{"schema_version":1,"status":"done","outputs":{"k":` + bigOut + `}}`,
		"check no id":     `{"schema_version":1,"status":"done","checks_run":[{"id":"","exit":0}]}`,
		"needs_info no q": `{"schema_version":1,"status":"needs_info"}`,
	}
	for name, body := range cases {
		p := ParseExecutionResult(block(testNonce, body), testNonce, nil)
		if p.Status != ParseStatusInvalid || p.Code != ResultCodeSchema {
			t.Errorf("%s: got %+v", name, p)
		}
	}
	// Boundaries that must pass.
	edge := `{"schema_version":1,"status":"done","summary":"` + strings.Repeat("x", MaxResultSummaryBytes) + `","files_changed":[` + strings.Repeat(`"a",`, MaxResultListEntries-1) + `"a"]}`
	if p := ParseExecutionResult(block(testNonce, edge), testNonce, nil); p.Status != ParseStatusOK {
		t.Errorf("boundary values rejected: %+v", p)
	}
}

func TestParseExecutionResult_UnknownTopLevelKeyRejected(t *testing.T) {
	p := ParseExecutionResult(block(testNonce, `{"schema_version":1,"status":"done","surprise":1}`), testNonce, nil)
	if p.Status != ParseStatusInvalid || p.Code != ResultCodeSchema {
		t.Fatalf("got %+v", p)
	}
	// Per-check extras stay tolerated.
	p = ParseExecutionResult(block(testNonce, `{"schema_version":1,"status":"done","checks_run":[{"id":"c","exit":0,"ms":5}]}`), testNonce, nil)
	if p.Status != ParseStatusOK {
		t.Fatalf("nested extra rejected: %+v", p)
	}
}

func TestParseExecutionResult_NeedsInfoWithoutQuestionsInvalid(t *testing.T) {
	p := ParseExecutionResult(block(testNonce, `{"schema_version":1,"status":"needs_info","questions":["which db?"]}`), testNonce, nil)
	if p.Status != ParseStatusOK || p.Result.Status != ResultNeedsInfo {
		t.Fatalf("got %+v", p)
	}
}

func TestOutputsByName_TypeChecks(t *testing.T) {
	r := ExecutionResult{Outputs: map[string]json.RawMessage{
		"files": json.RawMessage(`["a","b"]`), "count": json.RawMessage(`3`), "note": json.RawMessage(`"hi"`),
		"api": json.RawMessage(`{"x":1}`), "any": json.RawMessage(`[1,{"a":2}]`), "extra": json.RawMessage(`1`),
		"badnum": json.RawMessage(`"3"`), "badlist": json.RawMessage(`"a"`),
	}}
	declared := map[string]string{"files": "file_list", "count": "number", "note": "text", "api": "api_schema", "any": "json",
		"badnum": "number", "badlist": "file_list", "absent": "text"}
	kept, missing := OutputsByName(r, declared)
	if len(kept) != 5 || kept["extra"] != nil {
		t.Fatalf("kept = %v", kept)
	}
	if strings.Join(missing, ",") != "absent,badlist,badnum" {
		t.Fatalf("missing = %v", missing)
	}
}

func FuzzFindResultBlock(f *testing.F) {
	f.Add("ORCA_RESULT_BEGIN "+testNonce+"\n{}\nORCA_RESULT_END "+testNonce, testNonce)
	f.Add("\x00\xff", "")
	f.Fuzz(func(t *testing.T, stdout, nonce string) {
		_, _ = FindResultBlock(stdout, nonce)
		_ = ParseExecutionResult(stdout, nonce, nil)
	})
}

func TestFindResultBlock_LargeStdoutDoesNotPanic(t *testing.T) {
	big := strings.Repeat("noise line\n", 5_000_000/11)
	if body, _ := FindResultBlock(big+block(testNonce, `{"a":1}`), testNonce); string(body) != `{"a":1}` {
		t.Fatalf("block at the end of big stdout not found")
	}
}
