package usecase

import (
	"context"
	"time"

	"github.com/stablyai/orca-go/common/apperrors"
	"github.com/stablyai/orca-go/services/credential-broker-service/internal/domain"
)

// appendAudit writes one access-audit row and is the ONLY call site for
// AuditRepository.Append in this package — every usecase (write, resolve,
// rotate, revoke) routes through here so the "never best-effort" rule from
// credential-broker-service.md §8 is enforced in exactly one place. Most
// importantly, resolve_credential.go calls this strictly BEFORE returning
// the resolved value to its caller — see that file's doc comment and this
// package's tests for the ordering assertion.
func appendAudit(ctx context.Context, auditRepo AuditRepository, credentialID, accessor string, action domain.Action, now time.Time) error {
	entry, err := domain.NewAccessAuditEntry(credentialID, accessorOrUnknown(accessor), action, now)
	if err != nil {
		return apperrors.New(apperrors.KindInternal, "CREDENTIAL_AUDIT_INVALID", err.Error(), err)
	}
	if err := auditRepo.Append(ctx, entry); err != nil {
		// The metadata/Vault mutation this audit entry describes may have
		// already succeeded, but this usecase still returns an error here —
		// per §8, an unaudited access is treated as a failed operation, not
		// a degraded-but-acceptable one.
		return apperrors.New(apperrors.KindInternal, "CREDENTIAL_AUDIT_WRITE_FAILED", "failed to append access audit entry", err)
	}
	return nil
}

// accessorOrUnknown guards against ever writing an empty
// accessor_service value — NewAccessAuditEntry would reject it anyway, but
// failing an audit-writing operation because the *caller identity itself*
// couldn't be resolved would make debugging a real incident (a compromised
// or misconfigured caller) harder, not easier. "unknown" is itself a
// meaningful, auditable value.
func accessorOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
