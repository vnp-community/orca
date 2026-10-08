package domain

import "testing"

func TestValidateConfirmation(t *testing.T) {
	cases := []struct {
		name string
		in   ConfirmationInput
		want string
	}{
		{"no type", ConfirmationInput{}, "REQUEST_TYPE_REQUIRED"},
		{"bug without size", ConfirmationInput{Type: RequestTypeBug, Urgency: UrgencyNormal}, "REQUEST_SIZE_REQUIRED"},
		{"refactor without size", ConfirmationInput{Type: RequestTypeRefactor, Urgency: UrgencyNormal}, "REQUEST_SIZE_REQUIRED"},
		{"bug with size", ConfirmationInput{Type: RequestTypeBug, Size: RequestSizeS, Urgency: UrgencyNormal}, ""},
		{"hotfix normal urgency", ConfirmationInput{Type: RequestTypeHotfix, Urgency: UrgencyNormal, Reason: "prod down"}, "REQUEST_HOTFIX_REQUIRES_URGENT"},
		{"hotfix without reason", ConfirmationInput{Type: RequestTypeHotfix, Urgency: UrgencyUrgent, Reason: "  "}, "REQUEST_REASON_REQUIRED"},
		{"hotfix ok", ConfirmationInput{Type: RequestTypeHotfix, Urgency: UrgencyUrgent, Reason: "prod down"}, ""},
		{"security without reason", ConfirmationInput{Type: RequestTypeSecurity, Urgency: UrgencyNormal}, "REQUEST_REASON_REQUIRED"},
		{"security ok", ConfirmationInput{Type: RequestTypeSecurity, Urgency: UrgencyNormal, Reason: "CVE"}, ""},
		{"task needs no size", ConfirmationInput{Type: RequestTypeTask, Urgency: UrgencyNormal}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConfirmation(c.in)
			if c.want == "" && err != nil || c.want != "" && codeOf(err) != c.want {
				t.Fatalf("want %q, got %v", c.want, err)
			}
		})
	}
}
