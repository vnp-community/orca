package output

import (
	"encoding/json"
	"fmt"
	"os"
)

// ReportDocument prints a gateway document: as received in jsonMode, else the human summary (or indented JSON when
// the command has none), and returns the exit code.
func ReportDocument(doc any, human string, jsonMode bool) int {
	if jsonMode {
		_ = json.NewEncoder(os.Stdout).Encode(doc)
		return ExitOK
	}
	if human != "" {
		fmt.Println(human)
		return ExitOK
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
	return ExitOK
}
