// Package fleetyaml parses and validates the fleet inventory YAML schema
// documented in docs/logic/fleet/BL-FLEET-01-fleet-inventory.md into the
// server list fleetctl POSTs to api-gateway's
// POST /v1/infra/fleet/import route.
//
// This schema predates backend-go and still documents identityFile
// (SSH-key-file) auth — backend-go's domain.SshTarget only ever stores a
// Vault SSH secrets engine role pointer (see infra-fleet-service.md §9), so
// identityFile is rejected here with an actionable message rather than
// silently ignored or translated.
package fleetyaml

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// hostnamePattern is a permissive DNS-hostname check: labels of
// letters/digits/hyphens (not starting/ending with a hyphen), dot-separated.
// IP addresses are validated separately via net.ParseIP.
var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// rawConfig mirrors the on-disk YAML shape verbatim.
type rawConfig struct {
	Defaults rawServerDefaults `yaml:"defaults"`
	Projects []rawProject      `yaml:"projects"`
	Servers  []rawServer       `yaml:"servers"`
}

type rawServerDefaults struct {
	RelayGracePeriodSec int    `yaml:"relayGracePeriodSec"`
	NodeVersion         string `yaml:"nodeVersion"`
	// IdentityFile is accepted here only so Parse can reject it with the
	// actionable Vault-role message — backend-go never honors it.
	IdentityFile string `yaml:"identityFile"`
	VaultSSHRole string `yaml:"vaultSshRole"`
}

type rawProject struct {
	Name string   `yaml:"name"`
	Tags []string `yaml:"tags"`
}

type rawServer struct {
	Hostname     string   `yaml:"hostname"`
	User         string   `yaml:"user"`
	IdentityFile string   `yaml:"identityFile"`
	VaultSSHRole string   `yaml:"vaultSshRole"`
	Project      string   `yaml:"project"`
	Tags         []string `yaml:"tags"`
	Port         int      `yaml:"port"`
}

// ServerInput is the validated, backend-go-shaped server entry —
// field-for-field what usecase.FleetServerInput / the
// ImportFleetInventoryRequest.FleetServerInput proto message expect.
type ServerInput struct {
	Host         string   `json:"host"`
	User         string   `json:"user"`
	VaultSSHRole string   `json:"vaultSshRole"`
	Project      string   `json:"project,omitempty"`
	Tags         []string `json:"tags,omitempty"`
}

// ParseResult is Parse's output: the validated server list plus any
// non-fatal warnings (e.g. a dropped `port` field) surfaced to the caller
// for display, not treated as errors.
type ParseResult struct {
	Servers  []ServerInput
	Warnings []string
}

// ValidationError is a fatal, actionable parse/validation failure —
// distinct from Warnings, which never abort the import.
type ValidationError struct {
	Host   string // best-effort; may be empty for file-level errors
	Reason string
}

func (e *ValidationError) Error() string {
	if e.Host == "" {
		return e.Reason
	}
	return fmt.Sprintf("%s: %s", e.Host, e.Reason)
}

const identityFileUnsupportedMsg = "identityFile is not supported against backend-go — provision a Vault SSH role for this target and set vaultSshRole instead, see infra-fleet-service.md §9"

// Parse reads and validates the fleet inventory YAML schema, returning the
// server list ready to POST to api-gateway's fleet-import route. It fails
// fast on the first validation error (unlike the import usecase's own
// skip-and-continue semantics) since a malformed local file is a config
// bug the operator should fix before importing, not a per-row runtime
// concern.
func Parse(data []byte) (ParseResult, error) {
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return ParseResult{}, fmt.Errorf("fleetyaml: parsing YAML: %w", err)
	}

	if raw.Defaults.IdentityFile != "" {
		return ParseResult{}, &ValidationError{Reason: identityFileUnsupportedMsg}
	}

	knownProjects := make(map[string]bool, len(raw.Projects))
	for _, p := range raw.Projects {
		if p.Name == "" {
			return ParseResult{}, &ValidationError{Reason: "projects[]: name is required"}
		}
		knownProjects[p.Name] = true
	}

	var result ParseResult
	for _, sv := range raw.Servers {
		if err := validateHostname(sv.Hostname); err != nil {
			return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: err.Error()}
		}
		if sv.User == "" {
			return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: "user is required"}
		}
		if sv.IdentityFile != "" {
			return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: identityFileUnsupportedMsg}
		}
		vaultSSHRole := sv.VaultSSHRole
		if vaultSSHRole == "" {
			vaultSSHRole = raw.Defaults.VaultSSHRole
		}
		if vaultSSHRole == "" {
			return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: "vaultSshRole is required (no per-server or defaults value set)"}
		}
		if sv.Project != "" && !knownProjects[sv.Project] {
			return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: fmt.Sprintf("project %q is not declared in projects[]", sv.Project)}
		}
		if sv.Port != 0 {
			if sv.Port < 1 || sv.Port > 65535 {
				return ParseResult{}, &ValidationError{Host: sv.Hostname, Reason: fmt.Sprintf("port %d is out of range 1-65535", sv.Port)}
			}
			// domain.SshTarget/CreateSshTargetRequest have no port field —
			// dropped, not an error.
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: port %d declared but dropped — backend-go's SshTarget has no port field", sv.Hostname, sv.Port))
		}

		result.Servers = append(result.Servers, ServerInput{
			Host: sv.Hostname, User: sv.User, VaultSSHRole: vaultSSHRole,
			Project: sv.Project, Tags: sv.Tags,
		})
	}

	return result, nil
}

func validateHostname(host string) error {
	if host == "" {
		return fmt.Errorf("hostname is required")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	if !hostnamePattern.MatchString(host) || strings.Contains(host, "..") {
		return fmt.Errorf("hostname %q is not a valid DNS name or IP address", host)
	}
	return nil
}
