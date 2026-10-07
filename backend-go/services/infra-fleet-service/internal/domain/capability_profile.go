package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"
)

var (
	ErrNotFound        = errors.New("domain: not found")
	ErrProfileTooLarge = errors.New("domain: profile too large")
)

type ProfileSource string

const (
	ProfileSourceProbe         ProfileSource = "probe"
	ProfileSourceHandshakeOnly ProfileSource = "handshake_only"
)

type CapabilityProfile struct {
	DevServerID       string
	TenantID          string
	Source            ProfileSource
	AgentBuildVersion string
	ProtocolVersion   int
	Features          []string
	ProfileJSON       []byte
	Fingerprint       string
	ProbedAt          time.Time
}

func (p CapabilityProfile) Degraded() bool {
	return p.Source == ProfileSourceHandshakeOnly
}

func (p CapabilityProfile) HasFeature(name string) bool {
	for _, f := range p.Features {
		if f == name {
			return true
		}
	}
	return false
}

func NormalizeFeatures(features []string) []string {
	if len(features) == 0 {
		return []string{}
	}
	set := make(map[string]struct{})
	var result []string
	for _, f := range features {
		if f == "" {
			continue
		}
		if _, ok := set[f]; !ok {
			set[f] = struct{}{}
			result = append(result, f)
			if len(result) >= 64 {
				break
			}
		}
	}
	sort.Strings(result)
	if result == nil {
		result = []string{}
	}
	return result
}

func ComputeFingerprint(profileJSON []byte) (string, error) {
	if len(profileJSON) == 0 {
		profileJSON = []byte("{}")
	}
	var data map[string]interface{}
	if err := json.Unmarshal(profileJSON, &data); err != nil {
		return "", err
	}

	delete(data, "probedAt")
	if hostRaw, ok := data["host"]; ok {
		if host, ok := hostRaw.(map[string]interface{}); ok {
			delete(host, "memFreeMb")
			delete(host, "diskFreeMb")
			delete(host, "loadAvg1")
			data["host"] = host
		}
	}

	canonicalJSON, err := json.Marshal(data)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(canonicalJSON)
	return hex.EncodeToString(hash[:]), nil
}
