package apiclient

import "context"

type LoginResult struct {
	JWT       string
	ExpiresAt string
}

// Login calls POST /auth/cli-token.
func Login(ctx context.Context, c *Client, email, password string) (LoginResult, error) {
	var resp struct {
		JWT       string `json:"jwt"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := c.do(ctx, "POST", "/auth/cli-token", map[string]string{
		"email": email, "password": password,
	}, &resp); err != nil {
		return LoginResult{}, err
	}
	return LoginResult{JWT: resp.JWT, ExpiresAt: resp.ExpiresAt}, nil
}
