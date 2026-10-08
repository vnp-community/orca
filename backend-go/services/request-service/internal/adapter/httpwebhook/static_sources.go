package httpwebhook

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// StaticSources is the interim per-(tenant, source) configuration until tenant settings exist
// (CR-REQ-025). Secrets are never in the JSON: each entry names an env var or a file.
type StaticSources struct {
	byKey map[string]Source
}

type staticEntry struct {
	TenantID   string `json:"tenant_id"`
	Source     string `json:"source"`
	ReporterID string `json:"reporter_id"`
	SecretEnv  string `json:"secret_env"`
	SecretFile string `json:"secret_file"`
}

// ParseStaticSources reads REQUEST_WEBHOOK_SOURCES:
// [{"tenant_id":"..","source":"sentry","reporter_id":"..","secret_env":"SENTRY_WEBHOOK_SECRET"}].
func ParseStaticSources(raw string, getenv func(string) string, readFile func(string) ([]byte, error)) (*StaticSources, error) {
	out := &StaticSources{byKey: map[string]Source{}}
	if strings.TrimSpace(raw) == "" {
		return out, nil
	}
	var entries []staticEntry
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES: %w", err)
	}
	for i, e := range entries {
		if _, err := uuid.Parse(e.TenantID); err != nil {
			return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES[%d]: tenant_id must be a UUID", i)
		}
		if _, err := uuid.Parse(e.ReporterID); err != nil {
			return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES[%d]: reporter_id must be a UUID", i)
		}
		if e.Source == "" {
			return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES[%d]: source is required", i)
		}
		var secret []byte
		switch {
		case e.SecretEnv != "":
			secret = []byte(getenv(e.SecretEnv))
		case e.SecretFile != "":
			b, err := readFile(e.SecretFile)
			if err != nil {
				return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES[%d]: read secret file: %w", i, err)
			}
			secret = []byte(strings.TrimSpace(string(b)))
		}
		if len(secret) == 0 {
			return nil, fmt.Errorf("REQUEST_WEBHOOK_SOURCES[%d]: secret_env or secret_file must yield a non-empty secret", i)
		}
		out.byKey[e.TenantID+"/"+e.Source] = Source{Secret: secret, ReporterID: e.ReporterID}
	}
	return out, nil
}

func (s *StaticSources) Resolve(tenantID, sourceName string) (Source, bool) {
	src, ok := s.byKey[tenantID+"/"+sourceName]
	return src, ok
}

func (s *StaticSources) Len() int { return len(s.byKey) }
