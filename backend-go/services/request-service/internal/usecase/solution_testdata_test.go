package usecase

import (
	"os"
	"regexp"
	"strconv"
	"testing"
)

func domainTestdata(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile("../domain/testdata/" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func validSolutionReply(t *testing.T) string {
	return domainTestdata(t, "solution_options/valid_two_options.json")
}

func fenced(s string) string { return "Đây là các phương án:\n```json\n" + s + "\n```\nHết." }

// oneOptionReply is invalid for change_request (needs two options).
func oneOptionReply() string {
	return `{"schema_version":1,"options":[{"id":"opt-1","title":"t","summary":"s","approach":"a","effort":{"size":"S"},"recommended":true}],"recommendation":{"option_id":"opt-1","reason":"r"}}`
}

func analysisReply(t *testing.T, name string) string {
	return domainTestdata(t, "analysis_documents/"+name)
}

// withLeaks plants a private key and a token in an excerpt; whitespace-tolerant because golden files get reformatted.
func withLeaks(doc string) string {
	re := regexp.MustCompile(`"excerpt"\s*:\s*"addr := p\.Address\.Street"`)
	return re.ReplaceAllLiteralString(doc, `"excerpt":"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----\ntoken ghp_abcdefghijklmnopqrstuvwxyz0123456789"`)
}

func setBool(doc, key string, v bool) string {
	re := regexp.MustCompile(`"` + key + `"\s*:\s*(true|false)`)
	return re.ReplaceAllLiteralString(doc, `"`+key+`":`+strconv.FormatBool(v))
}
