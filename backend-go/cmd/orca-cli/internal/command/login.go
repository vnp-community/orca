package command

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/stablyai/orca-go/cmd/orca-cli/internal/apiclient"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/config"
	"github.com/stablyai/orca-go/cmd/orca-cli/internal/output"
)

// LoginResult never carries the token: it is saved to the 0600 credentials file and nowhere else.
type LoginResult struct {
	APIURL    string `json:"apiUrl"`
	ExpiresAt string `json:"expiresAt,omitempty"`
}

// ExecuteLogin is `orca login`'s testable core: exchange email and password for a bearer JWT and save it.
// save is config.Save in production.
func ExecuteLogin(ctx context.Context, apiURL, email, password string, save func(config.Credentials) error, jsonMode bool) int {
	if email == "" || password == "" {
		return output.ReportError(&apiclient.CLIError{StatusCode: 400, Code: "INVALID_ARGUMENT",
			Message: "--email and a password (ORCA_PASSWORD or --password-stdin) are required"}, jsonMode)
	}
	res, err := apiclient.Login(ctx, apiclient.New(apiURL, ""), email, password)
	if err != nil {
		return output.ReportError(err, jsonMode)
	}
	creds := config.Credentials{APIURL: apiURL, JWT: res.JWT}
	if t, perr := time.Parse(time.RFC3339, res.ExpiresAt); perr == nil {
		creds.ExpiresAt = t
	}
	if err := save(creds); err != nil {
		return output.ReportError(err, jsonMode)
	}
	return output.ReportDocument(LoginResult{APIURL: apiURL, ExpiresAt: res.ExpiresAt}, "logged in to "+apiURL, jsonMode)
}

func newLoginCmd(jsonOutput *bool, exitCode *int) *cobra.Command {
	var apiURL, email string
	var passwordStdin bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "sign in to the Orca gateway and save a token for the other commands",
		Long: "Exchanges the account's email and password for a bearer token and saves it in the user config directory " +
			"(mode 0600). The password comes from ORCA_PASSWORD or, with --password-stdin, standard input; it is never a flag.",
		RunE: func(cmd *cobra.Command, args []string) error {
			password := os.Getenv("ORCA_PASSWORD")
			if passwordStdin {
				raw, err := readAllStdin()
				if err != nil {
					*exitCode = output.ReportError(err, *jsonOutput)
					return nil
				}
				password = strings.TrimRight(string(raw), "\r\n")
			}
			creds, _ := config.Load()
			url := config.ResolveAPIURL(creds, defaultAPIURL)
			if apiURL != "" {
				url = apiURL
			}
			*exitCode = ExecuteLogin(cmd.Context(), url, email, password, config.Save, *jsonOutput)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiURL, "api-url", "", "gateway base URL (default: ORCA_API_URL, the saved one, or "+defaultAPIURL+")")
	cmd.Flags().StringVar(&email, "email", os.Getenv("ORCA_EMAIL"), "account email (or ORCA_EMAIL)")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from standard input")
	return cmd
}
