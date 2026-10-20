// Package output_language names the languages the AI may be told to write in. The prompts read Language through their
// Input; a zero Language emits no instruction, which keeps the prompts that predate it byte for byte the same.
package output_language

import (
	"fmt"
	"strings"
)

// Language is a code and the English name the prompt instruction uses.
type Language struct {
	Code string
	Name string
}

// The closed set of languages. Adding one is a change here only: the templates read Name.
var (
	Vietnamese = Language{Code: "vi", Name: "Vietnamese"}
	English    = Language{Code: "en", Name: "English"}
)

// Supported lists every language in a fixed order.
func Supported() []Language {
	return []Language{Vietnamese, English}
}

// Parse matches raw against the language codes, ignoring case and surrounding space. An empty or unknown value is an
// error that lists the valid codes; the caller decides what an empty value means.
func Parse(raw string) (Language, error) {
	code := strings.ToLower(strings.TrimSpace(raw))
	for _, l := range Supported() {
		if l.Code == code {
			return l, nil
		}
	}
	return Language{}, fmt.Errorf("output language %q is not supported; use one of: %s", raw, codes())
}

func codes() string {
	supported := Supported()
	out := make([]string, len(supported))
	for i, l := range supported {
		out[i] = l.Code
	}
	return strings.Join(out, ", ")
}
