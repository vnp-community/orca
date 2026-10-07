package usecase

import (
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
)

type pageToken struct {
	CreatedAt int64  `json:"c"`
	ID        string `json:"i"`
}

func EncodePageToken(createdAt time.Time, id string) string {
	b, _ := json.Marshal(pageToken{
		CreatedAt: createdAt.UnixMicro(),
		ID:        id,
	})
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodePageToken(token string) (time.Time, string, error) {
	if token == "" {
		return time.Time{}, "", nil
	}

	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_PAGE_TOKEN", "invalid page token encoding", err)
	}

	var pt pageToken
	if err := json.Unmarshal(b, &pt); err != nil {
		return time.Time{}, "", apperrors.New(apperrors.KindInvalidArgument, "REQUEST_INVALID_PAGE_TOKEN", "invalid page token payload", err)
	}

	return time.UnixMicro(pt.CreatedAt).UTC(), pt.ID, nil
}
