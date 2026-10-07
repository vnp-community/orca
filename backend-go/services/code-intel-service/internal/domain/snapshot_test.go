package domain

import (
	"strings"
	"testing"
)

func TestSnapshot_ParamsHash(t *testing.T) {
	// struct map keys order stability is handled by json.Marshal
	type params struct {
		A int
		B string
	}
	
	p1 := params{A: 1, B: "test"}
	
	hash1, err := CanonicalParamsHash(p1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if hash1 == "" {
		t.Errorf("expected non-empty hash")
	}
}

func TestSnapshot_ETag(t *testing.T) {
	contentHash := "abcdef"
	cETag := ContentETag(contentHash)
	
	if !strings.HasPrefix(cETag, "\"") || !strings.HasSuffix(cETag, "\"") {
		t.Errorf("ContentETag must be quoted, got: %s", cETag)
	}
	
	client1 := ClientETag(cETag, "head1", false)
	client2 := ClientETag(cETag, "head2", false)
	client3 := ClientETag(cETag, "head1", true)
	
	if client1 == client2 {
		t.Errorf("expected different ClientETags for different heads")
	}
	if client1 == client3 {
		t.Errorf("expected different ClientETags for different stale values")
	}
	if !strings.HasPrefix(client1, "\"") || !strings.HasSuffix(client1, "\"") {
		t.Errorf("ClientETag must be quoted, got: %s", client1)
	}
	
	// test ValidateIfNoneMatch
	valid := "\"abcdef1234567890\""
	if err := ValidateIfNoneMatch(valid); err != nil {
		t.Errorf("expected nil error for valid if_none_match, got %v", err)
	}
	
	invalid := strings.Repeat("a", 82)
	if err := ValidateIfNoneMatch(invalid); err == nil {
		t.Errorf("expected error for if_none_match > 80 chars")
	}
}
