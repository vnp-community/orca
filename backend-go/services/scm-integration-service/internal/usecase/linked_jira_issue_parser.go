package usecase

import (
	"regexp"
	"strings"
)

const linkedIssueProviderJira = "jira"

var (
	jiraKeyPattern = regexp.MustCompile(`[A-Z][A-Z0-9]+-[0-9]+`)
	// "Fixes ENG-1", "closes: ENG-1" — only the first key after the keyword counts.
	closingKeywordKeyPattern = regexp.MustCompile(`(?i)\b(?:fix(?:e[sd])?|close[sd]?|resolve[sd]?)\b:?\s+([A-Z][A-Z0-9]+-[0-9]+)`)
)

// Prefixes of well-known "LETTERS-DIGITS" tokens that are not Jira projects;
// a false positive would transition an unrelated issue.
var nonJiraKeyPrefixes = map[string]struct{}{
	"UTF": {}, "SHA": {}, "ISO": {}, "CVE": {}, "CWE": {}, "RFC": {}, "MD": {}, "AES": {},
	"RSA": {}, "TLS": {}, "SSL": {}, "HTTP": {}, "IPV": {}, "PEP": {}, "GH": {}, "ES": {},
	"UTC": {}, "IEEE": {}, "WCAG": {}, "ECMA": {}, "BASE": {}, "PR": {}, "CRC": {}, "USB": {},
}

// ParseLinkedJiraIssue finds the Jira issue a pull request refers to. Sources
// are tried in priority order (head branch, title, body); in the body a key
// after a closing keyword wins over a bare mention. Returns ("", "") if none.
func ParseLinkedJiraIssue(headBranch, title, body string) (provider, ref string) {
	if key := firstJiraKey(headBranch); key != "" {
		return linkedIssueProviderJira, key
	}
	if key := firstJiraKey(title); key != "" {
		return linkedIssueProviderJira, key
	}
	for _, m := range closingKeywordKeyPattern.FindAllStringSubmatch(body, -1) {
		if key := firstJiraKey(m[1]); key != "" {
			return linkedIssueProviderJira, key
		}
	}
	if key := firstJiraKey(body); key != "" {
		return linkedIssueProviderJira, key
	}
	return "", ""
}

func firstJiraKey(text string) string {
	for _, loc := range jiraKeyPattern.FindAllStringIndex(text, -1) {
		start, end := loc[0], loc[1]
		if start > 0 && isASCIIAlphanumeric(text[start-1]) {
			continue
		}
		if end < len(text) {
			next := text[end]
			if isASCIIAlphanumeric(next) {
				continue
			}
			// "ENG-12-3" / "CVE-2024-1234": a second numeric segment is not an issue number.
			if next == '-' && end+1 < len(text) && text[end+1] >= '0' && text[end+1] <= '9' {
				continue
			}
		}
		key := text[start:end]
		if _, denied := nonJiraKeyPrefixes[key[:strings.IndexByte(key, '-')]]; denied {
			continue
		}
		return key
	}
	return ""
}

func isASCIIAlphanumeric(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}
