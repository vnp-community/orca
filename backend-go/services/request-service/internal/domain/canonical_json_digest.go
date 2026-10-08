package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
)

// ErrInvalidJSONDocument marks input that is not strict JSON (bad syntax, duplicate keys, too deep).
var ErrInvalidJSONDocument = errors.New("invalid json document")

// parseStrictJSON validates through CanonicalJSON so there is one strict parser in this service:
// a digest over a document with duplicate keys would be ambiguous.
func parseStrictJSON(raw []byte) ([]byte, error) {
	canon, err := CanonicalJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSONDocument, err)
	}
	return canon, nil
}

// DigestOptions binds an Approval to the exact document and choice: sha256 over the canonical JSON
// plus the chosen index, so an edit or a different choice cannot reuse an earlier digest.
func DigestOptions(options []byte, chosen *int) (string, error) {
	canon, err := parseStrictJSON(options)
	if err != nil {
		return "", err
	}
	chosenStr := "none"
	if chosen != nil {
		chosenStr = strconv.Itoa(*chosen)
	}
	sum := sha256.Sum256([]byte(string(canon) + "|chosen=" + chosenStr))
	return hex.EncodeToString(sum[:]), nil
}
