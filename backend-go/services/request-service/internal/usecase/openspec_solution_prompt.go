package usecase

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

type PromptInput struct {
	RequestTitle string
	RequestBody  string
	ChangeID     string
}

func BuildOpenSpecSolutionPrompt(in PromptInput) string {
	body := in.RequestBody
	if utf8.RuneCountInString(body) > 12000 {
		runes := []rune(body)
		body = string(runes[:12000])
	}
	// Strip control characters (simplified)
	body = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, body)

	prompt := fmt.Sprintf(`Do not modify any files outside openspec/changes/%s/.
<request>
Title: %s
Body:
%s
</request>
`, in.ChangeID, in.RequestTitle, body)

	return prompt
}
