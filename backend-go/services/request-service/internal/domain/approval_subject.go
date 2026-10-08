package domain

import (
	"crypto/sha256"
	"encoding/hex"
)

type SubjectType string

const (
	SubjectRequestType SubjectType = "request_type"
	SubjectSolution    SubjectType = "solution"
	SubjectFindings    SubjectType = "findings"
	SubjectAnswer      SubjectType = "answer"
	SubjectPlan        SubjectType = "plan"
	SubjectPhase       SubjectType = "phase"
	SubjectTaskList    SubjectType = "task_list"
	SubjectPreDeploy   SubjectType = "pre_deploy"
)

var AllSubjectTypes = []SubjectType{
	SubjectRequestType,
	SubjectSolution,
	SubjectFindings,
	SubjectAnswer,
	SubjectPlan,
	SubjectPhase,
	SubjectTaskList,
	SubjectPreDeploy,
}

func (s SubjectType) Valid() bool {
	for _, v := range AllSubjectTypes {
		if v == s {
			return true
		}
	}
	return false
}

// SubjectDigest hashes the parts of a subject an approver actually saw. Parts are joined with a unit
// separator so ("ab","c") and ("a","bc") differ; callers pass already-canonical strings.
func SubjectDigest(st SubjectType, parts ...string) string {
	h := sha256.New()
	h.Write([]byte(st))
	for _, p := range parts {
		h.Write([]byte{0x1f})
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}
