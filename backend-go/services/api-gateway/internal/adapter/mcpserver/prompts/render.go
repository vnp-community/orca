package prompts

import (
	"bytes"
	"regexp"
	"strings"
)

type renderData struct {
	Args map[string]string
	URIs []string
}

// renderBuiltin renders the embedded template for locale (fallback en) and
// appends the fixed trailer.
func renderBuiltin(def builtinDef, locale string, args map[string]string) (text string, uris []string, err error) {
	if def.URIs != nil {
		uris = def.URIs(args)
	}
	t := templates[def.Name+"."+locale]
	if t == nil {
		locale, t = "en", templates[def.Name+".en"]
	}
	var b bytes.Buffer
	if err := t.Execute(&b, renderData{Args: args, URIs: uris}); err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(b.String()) + "\n\n" + trailer[locale], uris, nil
}

var placeholder = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

// renderCustom substitutes {{name}} placeholders only; values are inserted
// verbatim and never re-scanned, so an argument cannot inject a placeholder.
func renderCustom(template string, args map[string]string, locale string) string {
	out := placeholder.ReplaceAllStringFunc(template, func(m string) string {
		return args[m[2:len(m)-2]]
	})
	return strings.TrimSpace(out) + "\n\n" + trailer[locale]
}
