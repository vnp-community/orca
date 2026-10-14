package domain

import "strings"

// TaskDescriptionLine returns the text after key on the first line of a task description that starts with it ("Verify:"),
// or "" when the line is absent or empty. Descriptions of an OpenSpec plan carry Serves, Code, Verify and Done when.
func TaskDescriptionLine(description, key string) string {
	for _, line := range strings.Split(description, "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), key); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// HasVerifyStep says whether the task names how it is verified.
func HasVerifyStep(description string) bool { return TaskDescriptionLine(description, "Verify:") != "" }
