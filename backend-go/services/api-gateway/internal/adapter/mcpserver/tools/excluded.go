package tools

import (
	_ "embed"
	"fmt"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

//go:embed excluded_channels.yaml
var excludedYAML []byte

// Exclusion keeps matching channels out of the tool catalog, with a reason the
// parity test requires so the decision is reviewable.
type Exclusion struct {
	Pattern            string `yaml:"pattern"`
	Reason             string `yaml:"reason"`
	Category           string `yaml:"category"`
	ListedAsHardDenied bool   `yaml:"listedAsHardDenied"`
}

// Exclusions is the parsed excluded_channels.yaml.
type Exclusions []Exclusion

// HardDenied is an excluded channel the admin Tools tab shows as locked.
type HardDenied struct {
	Name, Channel, Namespace, Reason string
}

// LoadExclusions parses the embedded list.
func LoadExclusions() (Exclusions, error) {
	var ex Exclusions
	if err := yaml.Unmarshal(excludedYAML, &ex); err != nil {
		return nil, fmt.Errorf("excluded_channels.yaml: %w", err)
	}
	for _, e := range ex {
		if e.Pattern == "" || len(e.Reason) < 20 {
			return nil, fmt.Errorf("excluded_channels.yaml: %q needs a pattern and a reason of at least 20 chars", e.Pattern)
		}
	}
	return ex, nil
}

// Match reports whether channel is excluded (glob; "*" crosses dots, so
// "credentials.*" covers "credentials.a.b").
func (e Exclusion) Match(channel string) bool {
	if !strings.Contains(e.Pattern, "*") {
		return e.Pattern == channel
	}
	ok, _ := path.Match(strings.ReplaceAll(e.Pattern, ".", "/"), strings.ReplaceAll(channel, ".", "/"))
	if ok {
		return true
	}
	// path.Match's "*" stops at "/"; allow a trailing ".*" to span deeper names.
	if strings.HasSuffix(e.Pattern, ".*") {
		return strings.HasPrefix(channel, strings.TrimSuffix(e.Pattern, "*"))
	}
	return false
}

// Find returns the first exclusion covering channel.
func (ex Exclusions) Find(channel string) (Exclusion, bool) {
	for _, e := range ex {
		if e.Match(channel) {
			return e, true
		}
	}
	return Exclusion{}, false
}

// HardDenied expands listedAsHardDenied patterns against the real inventory
// (a pattern with no match is listed under its own name).
func (ex Exclusions) HardDenied(channels []wscompat.ChannelInfo) []HardDenied {
	var out []HardDenied
	for _, e := range ex {
		if !e.ListedAsHardDenied {
			continue
		}
		n := 0
		for _, ch := range channels {
			if e.Match(ch.Name) {
				out = append(out, HardDenied{ChannelToToolName(ch.Name), ch.Name, namespaceOf(ch.Name), e.Reason})
				n++
			}
		}
		if n == 0 {
			out = append(out, HardDenied{ChannelToToolName(e.Pattern), e.Pattern, namespaceOf(e.Pattern), e.Reason})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
