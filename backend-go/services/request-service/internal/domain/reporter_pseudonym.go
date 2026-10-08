package domain

import (
	"crypto/hmac"
	"crypto/sha256"
)

// PseudonymizeReporter derives a stable UUID from the reporter and tenant with a server-side key, so an
// erased Request keeps a valid reporter_id that no longer identifies the person (nor grants reporter access).
func PseudonymizeReporter(key []byte, tenantID, reporterID string) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(tenantID + "|" + reporterID))
	b := mac.Sum(nil)[:16]
	b[6] = (b[6] & 0x0f) | 0x40 // version 4 keeps it a well-formed UUID
	b[8] = (b[8] & 0x3f) | 0x80
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 36)
	for i, c := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out = append(out, '-')
		}
		out = append(out, hex[c>>4], hex[c&0x0f])
	}
	return string(out)
}
