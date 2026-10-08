package domain

import "strings"

type ConfirmationInput struct {
	Type    RequestType
	Size    RequestSize
	Urgency Urgency
	Reason  string
}

// typeRequiresSize lists the types whose flow branches on size (phases only when size=L);
// mirrors docs/crs/v6/README.md section 3.4 so a missing size cannot be confirmed away.
func typeRequiresSize(t RequestType) bool {
	return t == RequestTypeBug || t == RequestTypeRefactor
}

// ValidateConfirmation applies the human confirmation rules of CR-REQ-005 section 2.3.
func ValidateConfirmation(in ConfirmationInput) error {
	if in.Type == "" {
		return ErrRequestTypeRequired()
	}
	if typeRequiresSize(in.Type) && in.Size == "" {
		return ErrRequestSizeRequired(in.Type)
	}
	if in.Type == RequestTypeHotfix && in.Urgency != UrgencyUrgent {
		return ErrRequestHotfixRequiresUrgent()
	}
	if (in.Type == RequestTypeHotfix || in.Type == RequestTypeSecurity) && strings.TrimSpace(in.Reason) == "" {
		return ErrTypeReasonRequired(in.Type)
	}
	return nil
}
