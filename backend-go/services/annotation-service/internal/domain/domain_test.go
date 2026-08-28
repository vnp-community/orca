package domain

import (
	"testing"
	"time"
)

func TestNewAnchor_ValidatesInvariants(t *testing.T) {
	tests := []struct {
		name     string
		repoID   string
		filePath string
		line     int32
		wantErr  error
	}{
		{"valid", "repo-1", "main.go", 10, nil},
		{"valid zero line", "repo-1", "main.go", 0, nil},
		{"empty repo id", "", "main.go", 10, ErrEmptyRepoID},
		{"empty file path", "repo-1", "", 10, ErrEmptyFilePath},
		{"negative line", "repo-1", "main.go", -1, ErrNegativeLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAnchor(tt.repoID, "", tt.filePath, tt.line, 0, SideUnspecified, "main")
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewAnnotation_ValidatesInvariants(t *testing.T) {
	now := time.Now()
	validAnchor := Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 10, Ref: "main"}

	tests := []struct {
		name      string
		tenantID  string
		authorID  string
		anchor    Anchor
		content   string
		requestID string
		wantErr   error
	}{
		{"valid", "t1", "u1", validAnchor, "looks good", "req-1", nil},
		{"empty tenant", "", "u1", validAnchor, "looks good", "req-1", ErrEmptyTenant},
		{"empty author", "t1", "", validAnchor, "looks good", "req-1", ErrEmptyTenant},
		{"empty content", "t1", "u1", validAnchor, "", "req-1", ErrEmptyContent},
		{"empty request id", "t1", "u1", validAnchor, "looks good", "", ErrEmptyRequestID},
		{"invalid anchor", "t1", "u1", Anchor{RepoID: "", FilePath: "main.go", Line: 1}, "looks good", "req-1", ErrEmptyRepoID},
		{"negative anchor line", "t1", "u1", Anchor{RepoID: "repo-1", FilePath: "main.go", Line: -5}, "looks good", "req-1", ErrNegativeLine},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAnnotation("a1", tt.tenantID, tt.authorID, tt.anchor, tt.content, "", false, tt.requestID, now, now)
			if tt.wantErr == nil && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if tt.wantErr != nil && err != tt.wantErr {
				t.Fatalf("expected %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNewAnnotation_ReturnsPopulatedAnnotation(t *testing.T) {
	now := time.Now()
	anchor := Anchor{RepoID: "repo-1", FilePath: "main.go", Line: 42, Ref: "abc123"}

	got, err := NewAnnotation("a1", "t1", "u1", anchor, "nit: rename this", "", true, "req-1", now, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "a1" || got.TenantID != "t1" || got.AuthorID != "u1" {
		t.Errorf("unexpected identity fields: %+v", got)
	}
	if got.Anchor != anchor {
		t.Errorf("expected anchor %+v, got %+v", anchor, got.Anchor)
	}
	if got.Content != "nit: rename this" || !got.Resolved {
		t.Errorf("unexpected content/resolved: %+v", got)
	}
	if got.RequestID != "req-1" {
		t.Errorf("expected request id to round-trip, got %q", got.RequestID)
	}
}
