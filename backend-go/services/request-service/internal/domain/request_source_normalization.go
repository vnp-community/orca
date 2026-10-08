package domain

import (
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxRequestTitleRunes = 500
	MaxRequestBodyRunes  = 100000
	// source_ref and source_site are VARCHAR(255) on MySQL.
	maxSourceKeyRunes = 255
)

// NormalizeSourceRef canonicalises a source reference so the same external issue always
// yields the same idempotency key regardless of how the caller spelled it.
func NormalizeSourceRef(in SourceRef) (SourceRef, error) {
	if _, err := ParseSourceProvider(string(in.Provider)); err != nil {
		return SourceRef{}, ErrRequestSourceProviderInvalid(string(in.Provider))
	}
	out := SourceRef{Provider: in.Provider, Site: strings.TrimSpace(in.Site), Ref: strings.TrimSpace(in.Ref), URL: strings.TrimSpace(in.URL)}
	if utf8.RuneCountInString(out.Ref) > maxSourceKeyRunes || utf8.RuneCountInString(out.Site) > maxSourceKeyRunes {
		return SourceRef{}, ErrRequestSourceRefInvalid(in.Provider, "ref or site longer than 255 characters")
	}
	switch in.Provider {
	case SourceProviderManual, SourceProviderMCP:
		if out.Ref != "" {
			return SourceRef{}, ErrRequestSourceRefInvalid(in.Provider, "ref must be empty")
		}
		return out, nil
	case SourceProviderWebhook:
		if out.Site == "" {
			return SourceRef{}, ErrRequestSourceSiteRequired(in.Provider)
		}
		if out.Ref == "" {
			return SourceRef{}, ErrRequestSourceRefRequired(in.Provider)
		}
		return out, nil
	}
	if out.Ref == "" {
		return SourceRef{}, ErrRequestSourceRefRequired(in.Provider)
	}
	out.Site = canonicalSite(out.Site)
	switch in.Provider {
	case SourceProviderJira, SourceProviderLinear:
		out.Ref = strings.ToUpper(out.Ref)
	case SourceProviderGithub, SourceProviderGitlab:
		ref, err := canonicalRepoRef(in.Provider, out.Ref)
		if err != nil {
			return SourceRef{}, err
		}
		out.Ref = ref
	}
	return out, nil
}

// canonicalSite lowercases scheme and host of URL-shaped sites; anything else (a Jira
// workspace id, say) is kept as typed because lowercasing could merge distinct ids.
func canonicalSite(site string) string {
	u, err := url.Parse(site)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return site
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")
}

func canonicalRepoRef(p SourceProvider, ref string) (string, error) {
	i := strings.LastIndex(ref, "#")
	if i <= 0 {
		return "", ErrRequestSourceRefInvalid(p, "expected <repo>#<number>")
	}
	n, err := strconv.Atoi(ref[i+1:])
	if err != nil || n <= 0 {
		return "", ErrRequestSourceRefInvalid(p, "issue number must be a positive integer")
	}
	return strings.ToLower(ref[:i]) + "#" + strconv.Itoa(n), nil
}

func NormalizeTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	if n == 0 {
		return "", ErrRequestTitleRequired()
	}
	if n > MaxRequestTitleRunes {
		return "", ErrRequestTitleTooLong()
	}
	return s, nil
}

func NormalizeBody(s string) (string, error) {
	if utf8.RuneCountInString(s) > MaxRequestBodyRunes {
		return "", ErrRequestBodyTooLarge()
	}
	return s, nil
}

// IdempotencyKey mirrors request_idempotency's primary key minus the tenant.
type IdempotencyKey struct {
	Provider SourceProvider
	Site     string
	Ref      string
}

// BuildIdempotencyKey returns ok=false when the call carries nothing to deduplicate on
// (manual or MCP without a client_request_id), so such calls always create a Request.
// The manual/MCP key is scoped per reporter so two users' client ids never collide.
func BuildIdempotencyKey(ref SourceRef, reporterID, clientRequestID string) (IdempotencyKey, bool, error) {
	switch ref.Provider {
	case SourceProviderManual, SourceProviderMCP:
		id := strings.TrimSpace(clientRequestID)
		if id == "" {
			return IdempotencyKey{}, false, nil
		}
		if reporterID == "" {
			return IdempotencyKey{}, false, ErrRequestReporterRequired()
		}
		if utf8.RuneCountInString(id) > maxSourceKeyRunes {
			return IdempotencyKey{}, false, ErrRequestSourceRefInvalid(ref.Provider, "client_request_id too long")
		}
		return IdempotencyKey{Provider: ref.Provider, Site: "user:" + reporterID, Ref: id}, true, nil
	}
	if ref.Ref == "" {
		return IdempotencyKey{}, false, ErrRequestSourceRefRequired(ref.Provider)
	}
	return IdempotencyKey{Provider: ref.Provider, Site: ref.Site, Ref: ref.Ref}, true, nil
}
