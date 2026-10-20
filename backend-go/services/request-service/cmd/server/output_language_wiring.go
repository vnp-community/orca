package main

import (
	"fmt"
	"strings"

	"github.com/stablyai/orca-go/prompts/output_language"
)

// outputLanguageEnv names the setting in errors so an operator finds it without reading code.
const outputLanguageEnv = "REQUEST_DEFAULT_OUTPUT_LANGUAGE"

// parseOutputLanguage reads the system-wide AI output language. An empty value is Vietnamese; any other value must be
// a supported code, because a typo that silently fell back would give a whole deployment the wrong language.
func parseOutputLanguage(raw string) (output_language.Language, error) {
	if strings.TrimSpace(raw) == "" {
		return output_language.Vietnamese, nil
	}
	lang, err := output_language.Parse(raw)
	if err != nil {
		return output_language.Language{}, fmt.Errorf("%s %q is not supported; use one of: %s", outputLanguageEnv, raw, supportedLanguageCodes())
	}
	return lang, nil
}

func supportedLanguageCodes() string {
	supported := output_language.Supported()
	codes := make([]string, len(supported))
	for i, l := range supported {
		codes[i] = l.Code
	}
	return strings.Join(codes, ", ")
}
