package usecase

import (
	"context"
	"encoding/json"

	"github.com/stablyai/orca-go/common/apperrors"
)

type GetJWKSOutput struct {
	JWKSJSON string
}

// GetJWKS publishes the signing key's public half as an RFC 7517 JWK Set —
// unauthenticated by RPC convention (see proto/orca/auth/v1/auth.proto's
// GetJWKS doc comment), so this usecase takes no actor/context checks.
type GetJWKS struct {
	signer TokenSigner
}

func NewGetJWKS(signer TokenSigner) *GetJWKS {
	return &GetJWKS{signer: signer}
}

func (uc *GetJWKS) Execute(ctx context.Context) (GetJWKSOutput, error) {
	jwks, err := uc.signer.PublicJWKS(ctx)
	if err != nil {
		return GetJWKSOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_GET_JWKS_FAILED", "failed to load JWKS", err)
	}
	b, err := json.Marshal(jwks)
	if err != nil {
		return GetJWKSOutput{}, apperrors.New(apperrors.KindInternal, "AUTH_GET_JWKS_MARSHAL_FAILED", "failed to marshal JWKS", err)
	}
	return GetJWKSOutput{JWKSJSON: string(b)}, nil
}
