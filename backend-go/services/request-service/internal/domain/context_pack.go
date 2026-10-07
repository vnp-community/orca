package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type Stage string

const (
	StageClassify Stage = "classify"
	StageSolution Stage = "solution"
	StagePlan     Stage = "plan"
	StageTask     Stage = "task"
	StageExecute  Stage = "execute"
	StageRisk     Stage = "risk"
)

const CPVersion = "cp/1"

type ContextPack struct {
	ID           string
	TenantID     string
	RequestID    string
	Stage        Stage
	CPVersion    string
	InputDigest  string
	Digest       string
	BudgetTokens int
	UsedTokens   int
	Items        []PackItem
	Missing      []MissingEntry
	Body         string
	CreatedAt    time.Time
}

type PackItem struct {
	EvidenceID string
	Rank       int
	Tokens     int
	Redactions int
	Truncated  bool
}

type MissingEntry struct {
	SourceID string
	Reason   string
}

type BuildInput struct {
	TenantID  string
	RequestID string
	Stage     Stage
	Budget    int
}

func ComputeInputDigest(parts ...string) string {
	h := sha256.New()
	for i, p := range parts {
		if i > 0 {
			h.Write([]byte{0x1f})
		}
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ComputePackDigest(body string, items []PackItem) string {
	h := sha256.New()
	h.Write([]byte(body))
	for _, it := range items {
		h.Write([]byte{0x1f})
		h.Write([]byte(it.EvidenceID))
	}
	return hex.EncodeToString(h.Sum(nil))
}
