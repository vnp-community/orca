package usecase

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stablyai/orca-go/common/apperrors"
)

// EncodeApprovalCursor / DecodeApprovalCursor keep one keyset (created_at, id) format for both dialects.
func EncodeApprovalCursor(createdAt time.Time, id string) string {
	return base64.URLEncoding.EncodeToString([]byte(createdAt.UTC().Format(time.RFC3339Nano) + "|" + id))
}

func DecodeApprovalCursor(token string) (time.Time, string, error) {
	invalid := apperrors.New(apperrors.KindInvalidArgument, "REQUEST_APPROVAL_PAGE_TOKEN_INVALID", "page token is invalid", nil)
	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", invalid
	}
	parts := strings.SplitN(string(raw), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", invalid
	}
	if _, err := uuid.Parse(parts[1]); err != nil {
		return time.Time{}, "", invalid
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", invalid
	}
	return t, parts[1], nil
}
