package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

var ErrBadPageToken = errors.New("domain: invalid page token")

type pageTokenStruct struct {
	T time.Time `json:"t"`
	I string    `json:"i"`
}

func EncodePageToken(updatedAt time.Time, id string) string {
	b, _ := json.Marshal(pageTokenStruct{T: updatedAt, I: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func DecodePageToken(s string) (time.Time, string, error) {
	if s == "" {
		return time.Time{}, "", nil
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, "", ErrBadPageToken
	}
	var pt pageTokenStruct
	if err := json.Unmarshal(b, &pt); err != nil {
		return time.Time{}, "", ErrBadPageToken
	}
	if pt.I == "" || pt.T.IsZero() {
		return time.Time{}, "", ErrBadPageToken
	}
	return pt.T, pt.I, nil
}
