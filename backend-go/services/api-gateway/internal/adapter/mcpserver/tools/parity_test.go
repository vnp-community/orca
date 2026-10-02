package tools

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/services/api-gateway/internal/adapter/wscompat"
)

// productionRegistry builds the real channel inventory with nil downstreams.
func productionRegistry() *wscompat.Registry {
	r := wscompat.NewRegistry()
	wscompat.RegisterProductionChannels(r, wscompat.ChannelDeps{TaskActivityEnabled: true})
	wscompat.RegisterMcpChannels(r, wscompat.McpChannelDeps{})
	return r
}

var toolNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func specCovers(specs []*ToolSpec) map[string]*ToolSpec {
	m := map[string]*ToolSpec{}
	for _, s := range specs {
		if s.Channel != "" {
			m[s.Channel] = s
		}
		for _, c := range s.UsesChannels {
			m[c] = s
		}
	}
	return m
}

// TestChannelInventory prints the counts CI tracks and is red when a
// registered channel has neither a ToolSpec nor an exclusion with a reason.
func TestChannelInventory(t *testing.T) {
	reg := productionRegistry()
	chans := reg.Channels()
	specs := AllSpecs()
	ex, err := LoadExclusions()
	if err != nil {
		t.Fatal(err)
	}
	covers := specCovers(specs)
	kinds := map[wscompat.ChannelKind]int{}
	namespaces := map[string]bool{}
	var uncovered, both []string
	covered, excluded := 0, 0
	for _, ch := range chans {
		kinds[ch.Kind]++
		namespaces[namespaceOf(ch.Name)] = true
		_, hasSpec := covers[ch.Name]
		_, isEx := ex.Find(ch.Name)
		switch {
		case hasSpec && isEx:
			both = append(both, ch.Name)
		case hasSpec:
			covered++
		case isEx:
			excluded++
		default:
			uncovered = append(uncovered, ch.Name)
		}
	}
	fmt.Printf("MCP_CHANNEL_INVENTORY total=%d unary=%d stream=%d streamChannel=%d binary=%d namespaces=%d covered=%d excluded=%d uncovered=%d\n",
		len(chans), kinds[wscompat.ChannelUnary], kinds[wscompat.ChannelStream], kinds[wscompat.ChannelStreamChannel],
		kinds[wscompat.ChannelBinaryStream], len(namespaces), covered, excluded, len(uncovered))
	if len(both) > 0 {
		t.Errorf("channels both exposed as tools and excluded: %v", both)
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		t.Errorf("%d channels have neither a ToolSpec nor an exclusion in excluded_channels.yaml:\n  %s", len(uncovered), strings.Join(uncovered, "\n  "))
	}
}

func TestToolParity(t *testing.T) {
	reg := productionRegistry()
	byName := map[string]wscompat.ChannelKind{}
	for _, ch := range reg.Channels() {
		byName[ch.Name] = ch.Kind
	}
	specs := AllSpecs()
	ex, _ := LoadExclusions()

	// Dead exclusions: every pattern must match a registered channel.
	for _, e := range ex {
		hit := false
		for n := range byName {
			if e.Match(n) {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("exclusion %q matches no registered channel", e.Pattern)
		}
	}

	names := map[string]bool{}
	seenChannel := map[string]bool{}
	for _, s := range specs {
		if names[s.Name] {
			t.Errorf("duplicate tool name %s", s.Name)
		}
		names[s.Name] = true
		if !toolNameRE.MatchString(s.Name) {
			t.Errorf("%s: invalid tool name", s.Name)
		}
		if s.Name != ChannelToToolName(s.Channel) && s.NameReason == "" {
			t.Errorf("%s: name differs from channel %s without NameReason", s.Name, s.Channel)
		}
		if s.Kind == KindChannel {
			if seenChannel[s.Channel] {
				t.Errorf("channel %s has two tool specs", s.Channel)
			}
			seenChannel[s.Channel] = true
			k, ok := byName[s.Channel]
			if !ok {
				t.Errorf("%s: channel %s is not registered", s.Name, s.Channel)
			} else if k != wscompat.ChannelUnary && k != wscompat.ChannelStreamChannel {
				t.Errorf("%s: channel %s is %s; only unary/streamChannel can back a tool", s.Name, s.Channel, k)
			}
		}
		for _, c := range s.UsesChannels {
			if _, ok := byName[c]; !ok {
				t.Errorf("%s: composite uses unregistered channel %s", s.Name, c)
			}
			if _, isEx := ex.Find(c); isEx {
				t.Errorf("%s: composite uses excluded channel %s", s.Name, c)
			}
		}
		if s.Kind == KindComposite && (s.Compose == nil || s.Risk == "") {
			t.Errorf("%s: composite needs Compose and a Risk", s.Name)
		}
		if _, isEx := ex.Find(s.Channel); isEx {
			t.Errorf("%s: channel %s is also excluded", s.Name, s.Channel)
		}
		if s.Description == "" || len(s.Description) > 1024 {
			t.Errorf("%s: description must be 1..1024 chars", s.Name)
		}
		low := strings.ToLower(s.Description)
		for _, bad := range []string{"ignore previous", "http://", "https://", "@"} {
			if strings.Contains(low, bad) {
				t.Errorf("%s: description contains forbidden %q", s.Name, bad)
			}
		}
		if s.Pack < 1 || s.Pack > 4 {
			t.Errorf("%s: pack %d", s.Name, s.Pack)
		}
		if s.Annotations.ReadOnly && s.Risk != "read" && s.Risk != "admin" {
			t.Errorf("%s: ReadOnly with risk %s", s.Name, s.Risk)
		}
		if s.Annotations.Destructive && s.Risk != "destructive" && s.Risk != "admin" {
			t.Errorf("%s: Destructive with risk %s", s.Name, s.Risk)
		}
		if s.Risk == "read" != (s.Pack == 1) {
			t.Errorf("%s: risk %s does not fit pack %d", s.Name, s.Risk, s.Pack)
		}
		if err := s.prepare(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}
