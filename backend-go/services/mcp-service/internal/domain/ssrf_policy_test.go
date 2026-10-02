package domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.8.8.8", "::1", "::ffff:127.0.0.1", "0.0.0.0", "::",
		"10.0.0.1", "172.16.5.4", "192.168.1.1", "fd00:ec2::254", "fc00::1",
		"169.254.169.254", "169.254.0.1", "fe80::1", "::ffff:169.254.169.254",
		"100.64.0.1", "100.100.100.200", "100.127.255.255",
		"224.0.0.1", "ff02::1", "255.255.255.255", "240.0.0.1", "198.18.0.1",
		"64:ff9b::7f00:1", "2002:7f00:1::1", "192.0.0.8", "0.1.2.3",
	}
	for _, s := range blocked {
		if !IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s must be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "1.1.1.1", "93.184.216.34", "2606:4700:4700::1111", "100.63.255.255", "100.128.0.1", "172.32.0.1", "::ffff:8.8.8.8"}
	for _, s := range allowed {
		if IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s must be allowed", s)
		}
	}
	if !IsBlockedIP(netip.Addr{}) {
		t.Error("the zero address must be blocked")
	}
}

func TestValidateExternalURL(t *testing.T) {
	pol := ExternalURLPolicy{HTTPAllowlist: []string{"localhost:8080"}}
	ok := []string{"https://mcp.example.com/mcp", "https://mcp.example.com:443/x", "http://localhost:8080/mcp"}
	for _, u := range ok {
		if _, err := ValidateExternalURL(u, pol); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
	bad := map[string]string{
		"http://mcp.example.com/mcp":            CodeServerSSRFBlocked,
		"http://localhost:9999/mcp":             CodeServerSSRFBlocked,
		"ftp://mcp.example.com/":                CodeServerSSRFBlocked,
		"https://127.0.0.1/mcp":                 CodeServerSSRFBlocked,
		"https://[::1]/mcp":                     CodeServerSSRFBlocked,
		"https://169.254.169.254/latest":        CodeServerSSRFBlocked,
		"https://mcp.example.com:8443/mcp":      CodeServerSSRFBlocked,
		"https://user:pw@mcp.example.com/mcp":   CodeServerInvalid,
		"https://mcp.example.com/mcp?token=abc": CodeServerInvalid,
		"https://mcp.example.com/#frag":         CodeServerInvalid,
		"not a url":                             CodeServerInvalid,
		"https:///nohost":                       CodeServerInvalid,
	}
	for u, code := range bad {
		_, err := ValidateExternalURL(u, pol)
		if err == nil || !contains(err.Error(), code) {
			t.Errorf("%s: want %s, got %v", u, code, err)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestToolsDigest_StableUnderReorderAndChangesOnDescription(t *testing.T) {
	a := []ToolInfo{{Name: "a", Description: "x", InputSchema: []byte(`{"b":1,"a":2}`)}, {Name: "b", Description: "y"}}
	b := []ToolInfo{{Name: "b", Description: "y"}, {Name: "a", Description: "x", InputSchema: []byte(`{"a":2,"b":1}`)}}
	if ComputeToolsDigest(a) != ComputeToolsDigest(b) {
		t.Fatal("reordering tools or schema keys must not change the digest")
	}
	c := []ToolInfo{{Name: "a", Description: "x!", InputSchema: []byte(`{"b":1,"a":2}`)}, {Name: "b", Description: "y"}}
	if ComputeToolsDigest(a) == ComputeToolsDigest(c) {
		t.Fatal("a description change must change the digest")
	}
}

func TestValidateStdioCommand(t *testing.T) {
	good := [][]string{{"npx", "-y", "@scope/pkg@1.2.3"}, {"uvx", "tool@0.4.1", "--flag"}, {"node", "server.js"}}
	for _, g := range good {
		if err := ValidateStdioCommand(g[0], g[1:]); err != nil {
			t.Errorf("%v: %v", g, err)
		}
	}
	bad := [][]string{
		{"/usr/bin/evil"}, {"sh", "-c", "curl x|sh"}, {"node", "--eval", "1"}, {"npx", "pkg@latest"}, {"npx", "pkg"},
		{"npx", "-y"}, {"python", "-c", "x"}, {"cmd", "/c", "dir"}, {"node", "s.js", "--token=ghp_abcdefghijklmnopqrstuvwxyz0123"},
		{"bad name"}, {""},
	}
	for _, g := range bad {
		if err := ValidateStdioCommand(g[0], g[1:]); err == nil {
			t.Errorf("%v must be rejected", g)
		}
	}
}

func TestSecretValueNeverFormats(t *testing.T) {
	v := NewSecretValue("hunter2-secret")
	j, _ := json.Marshal(map[string]any{"v": v})
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("x", slog.Any("v", v))
	all := fmt.Sprintf("%v|%+v|%#v|%s|%q", v, v, v, v, v) + string(j) + buf.String() + fmt.Sprintf("%v", &v)
	if strings.Contains(all, "hunter2") {
		t.Fatalf("secret leaked: %s", all)
	}
	if string(v.Reveal()) != "hunter2-secret" {
		t.Fatal("Reveal must return the value")
	}
}
