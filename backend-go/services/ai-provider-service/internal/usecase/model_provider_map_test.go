package usecase

import (
	"testing"

	"github.com/stablyai/orca-go/services/ai-provider-service/internal/domain"
)

func TestDetectProviderFromModel(t *testing.T) {
	tests := []struct {
		model  string
		want   domain.ProviderType
		wantOK bool
	}{
		{"claude-3-5-sonnet", domain.ProviderTypeAnthropic, true},
		{"gpt-4o", domain.ProviderTypeOpenAI, true},
		{"o1-preview", domain.ProviderTypeOpenAI, true},
		{"o3-mini", domain.ProviderTypeOpenAI, true},
		{"gemini-1.5-pro", domain.ProviderTypeGoogle, true},
		{"llama-3-70b", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got, ok := detectProviderFromModel(tt.model)
			if ok != tt.wantOK {
				t.Fatalf("detectProviderFromModel(%q) ok = %v, want %v", tt.model, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("detectProviderFromModel(%q) = %q, want %q", tt.model, got, tt.want)
			}
		})
	}
}
