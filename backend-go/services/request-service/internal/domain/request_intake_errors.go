package domain

import (
	"fmt"

	"github.com/stablyai/orca-go/common/apperrors"
)

func ErrRequestProjectRequired() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PROJECT_REQUIRED", "project id is required", nil)
}

func ErrRequestProjectInvalid() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_PROJECT_INVALID", "project id must be a UUID", nil)
}

func ErrRequestSourceProviderInvalid(p string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOURCE_PROVIDER_INVALID", fmt.Sprintf("invalid source provider: %s", p), nil)
}

func ErrRequestSourceRefRequired(p SourceProvider) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOURCE_REF_REQUIRED", fmt.Sprintf("source ref is required for provider %s", p), nil)
}

func ErrRequestSourceRefInvalid(p SourceProvider, why string) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOURCE_REF_INVALID", fmt.Sprintf("invalid source ref for provider %s: %s", p, why), nil)
}

func ErrRequestSourceSiteRequired(p SourceProvider) error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_SOURCE_SITE_REQUIRED", fmt.Sprintf("source site is required for provider %s", p), nil)
}

func ErrRequestTitleTooLong() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_TITLE_TOO_LONG", "request title exceeds 500 characters", nil)
}

func ErrRequestBodyTooLarge() error {
	return apperrors.New(apperrors.KindInvalidArgument, "REQUEST_BODY_TOO_LARGE", "request body exceeds 100000 characters", nil)
}

func ErrRequestSourceNotFound(p SourceProvider, ref string) error {
	return apperrors.New(apperrors.KindNotFound, "REQUEST_SOURCE_NOT_FOUND", fmt.Sprintf("source issue not found: %s %s", p, ref), nil)
}

func ErrRequestSourceFetchFailed(p SourceProvider, cause error) error {
	return apperrors.New(apperrors.KindInternal, "REQUEST_SOURCE_FETCH_FAILED", fmt.Sprintf("could not read source issue from %s", p), cause)
}
