package pushgateway

import (
	"encoding/asn1"
	"fmt"
	"math/big"
)

// This file's derECDSASignatureToRawJWS is intentionally duplicated from
// internal/usecase/vapid.go rather than imported: usecase/ has an
// identical helper for the same reason (Vault Transit's default
// marshaling_algorithm=asn1), but internal/adapter/pushgateway (this
// package) does its own local crypto/ecdsa.SignASN1 directly — no Vault
// call involved for APNs/FCM provider-token signing (unlike VAPID, this
// key material is resolved as plaintext PEM via PushCredentialResolver,
// not signed remotely) — so importing usecase's unexported helper isn't
// possible (unexported, different package) and wouldn't make sense
// architecturally anyway (this is adapter-local crypto, not a usecase
// concern). Same ~15 lines, kept in sync by inspection if RFC 7518 ever
// changes, which it won't.

type apnsFcmECDSASignature struct {
	R, S *big.Int
}

const ecdsaP256CoordinateLen = 32

// derECDSASignatureToRawJWS converts crypto/ecdsa.SignASN1's ASN.1 DER
// output into the raw, fixed-length r||s form a JWS ES256 signature
// requires (RFC 7518 §3.4).
func derECDSASignatureToRawJWS(der []byte) ([]byte, error) {
	var sig apnsFcmECDSASignature
	if _, err := asn1.Unmarshal(der, &sig); err != nil {
		return nil, fmt.Errorf("pushgateway: parse ASN.1 ECDSA signature: %w", err)
	}
	raw := make([]byte, 2*ecdsaP256CoordinateLen)
	sig.R.FillBytes(raw[:ecdsaP256CoordinateLen])
	sig.S.FillBytes(raw[ecdsaP256CoordinateLen:])
	return raw, nil
}
