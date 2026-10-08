package domain

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MaxAuditMetadataBytes matches the limit of auditclient and the outbox CHECK constraint.
const MaxAuditMetadataBytes = 4096

const maxAuditStringChars = 256

// forbiddenAuditKeys name content fields; audit metadata holds ids, kinds, states and counts only.
var forbiddenAuditKeys = map[string]bool{
	"title": true, "body": true, "content": true, "comment": true, "options": true,
	"prompt": true, "response": true, "excerpt": true,
}

// MarshalAuditMetadata validates and encodes audit metadata. A forbidden key at any depth, or a long string
// (a likely pasted body), is an error so content cannot reach the audit log by accident.
func MarshalAuditMetadata(requestID, auditID string, m map[string]any) (string, error) {
	out := make(map[string]any, len(m)+2)
	for k, v := range m {
		out[k] = v
	}
	if requestID != "" {
		out["request_id"] = requestID
	}
	if auditID != "" {
		out["audit_id"] = auditID
	}
	if err := checkAuditValue("", out); err != nil {
		return "", err
	}
	b, err := json.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("audit metadata: %w", err)
	}
	if len(b) > MaxAuditMetadataBytes {
		return "", fmt.Errorf("audit metadata: %d bytes exceeds %d", len(b), MaxAuditMetadataBytes)
	}
	return string(b), nil
}

func checkAuditValue(path string, v any) error {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if forbiddenAuditKeys[strings.ToLower(k)] {
				return fmt.Errorf("audit metadata: key %q is content and not allowed", path+k)
			}
			if err := checkAuditValue(path+k+".", child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range x {
			if err := checkAuditValue(path, child); err != nil {
				return err
			}
		}
	case string:
		if len([]rune(x)) > maxAuditStringChars {
			return fmt.Errorf("audit metadata: value at %q is longer than %d characters", strings.TrimSuffix(path, "."), maxAuditStringChars)
		}
	}
	return nil
}
