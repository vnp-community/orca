package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
)

// PKCE (RFC 7636). Only S256 exists in this server: "plain" and a missing
// challenge are rejected before any code is issued.
const (
	PKCEMethodS256 = "S256"
	pkceMinLen     = 43
	pkceMaxLen     = 128
)

var (
	ErrPKCEMethodUnsupported = errors.New("domain: code_challenge_method must be S256")
	ErrPKCEChallengeInvalid  = errors.New("domain: code_challenge must be 43-128 unreserved characters")
	ErrPKCEVerifierInvalid   = errors.New("domain: code_verifier must be 43-128 unreserved characters")
)

func isPKCEUnreserved(s string) bool {
	if len(s) < pkceMinLen || len(s) > pkceMaxLen {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '.', c == '_', c == '~':
		default:
			return false
		}
	}
	return true
}

// ValidatePKCEChallenge requires method S256 and a well-formed challenge.
func ValidatePKCEChallenge(challenge, method string) error {
	if method != PKCEMethodS256 {
		return ErrPKCEMethodUnsupported
	}
	if !isPKCEUnreserved(challenge) {
		return ErrPKCEChallengeInvalid
	}
	return nil
}

// VerifyPKCES256 reports whether base64url(sha256(verifier)) == challenge,
// in constant time. A malformed verifier never matches.
func VerifyPKCES256(verifier, challenge string) bool {
	if !isPKCEUnreserved(verifier) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(want), []byte(challenge)) == 1
}
