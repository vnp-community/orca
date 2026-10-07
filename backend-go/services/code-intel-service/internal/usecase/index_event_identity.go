package usecase

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// CodeIntelNamespaceUUID is the fixed UUID namespace for generating deterministic CodeIntel event IDs.
var CodeIntelNamespaceUUID = uuid.MustParse("e0b5f1a0-7b2d-4c31-9f5e-18d6a8f30001")

// DeterministicEventID computes a stable UUID v5 for an index event (§5).
// Keys: tenant|binding|tools|commit|indexedAt|kind.
func DeterministicEventID(tenant, binding string, tools []string, commit, indexedAt, kind string) string {
	toolsStr := strings.Join(tools, ",")
	raw := fmt.Sprintf("%s|%s|%s|%s|%s|%s", tenant, binding, toolsStr, commit, indexedAt, kind)
	return uuid.NewSHA1(CodeIntelNamespaceUUID, []byte(raw)).String()
}
