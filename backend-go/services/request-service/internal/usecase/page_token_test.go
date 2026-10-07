package usecase

import (
	"errors"
	"testing"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

func TestPageToken_RoundTrip(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	id := "req-123"

	encoded := EncodePageToken(now, id)
	if encoded == "" {
		t.Fatal("expected non-empty token")
	}

	decodedTime, decodedID, err := DecodePageToken(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !decodedTime.Equal(now) {
		t.Errorf("expected %v, got %v", now, decodedTime)
	}
	if decodedID != id {
		t.Errorf("expected %v, got %v", id, decodedID)
	}
}

func TestPageToken_InvalidBase64(t *testing.T) {
	_, _, err := DecodePageToken("invalid-base64!")
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "REQUEST_INVALID_PAGE_TOKEN" {
		t.Errorf("expected REQUEST_INVALID_PAGE_TOKEN, got %v", err)
	}
}

func TestPageToken_InvalidJSON(t *testing.T) {
	// encode valid base64 but invalid JSON
	_, _, err := DecodePageToken("aW52YWxpZC1qc29u") // "invalid-json"
	if err == nil {
		t.Fatal("expected error")
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) || appErr.Code != "REQUEST_INVALID_PAGE_TOKEN" {
		t.Errorf("expected REQUEST_INVALID_PAGE_TOKEN, got %v", err)
	}
}

func TestPageToken_EmptyMeansFirstPage(t *testing.T) {
	decodedTime, decodedID, err := DecodePageToken("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decodedTime.IsZero() {
		t.Errorf("expected zero time, got %v", decodedTime)
	}
	if decodedID != "" {
		t.Errorf("expected empty string, got %v", decodedID)
	}
}

func TestListFilter_Normalize(t *testing.T) {
	tests := []struct {
		in      int
		want    int
		wantErr bool
	}{
		{in: -1, wantErr: true},
		{in: 0, want: 50},
		{in: 100, want: 100},
		{in: 201, want: 200},
	}

	for _, tt := range tests {
		f := ListFilter{PageSize: tt.in}
		got, err := f.Normalize()
		if tt.wantErr {
			if err == nil {
				t.Errorf("Normalize(%d) expected error", tt.in)
			}
		} else {
			if err != nil {
				t.Errorf("Normalize(%d) unexpected error: %v", tt.in, err)
			}
			if got.PageSize != tt.want {
				t.Errorf("Normalize(%d) got %d, want %d", tt.in, got.PageSize, tt.want)
			}
		}
	}
}
