package main

import (
	"fmt"
	"net/url"
	"strings"
)

func toMySQLDriverDSN(dsn string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("parse dsn: %w", err)
	}
	if u.Scheme != "mysql" && u.Scheme != "tidb" {
		return "", fmt.Errorf("unrecognized DSN scheme: %q", u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host in DSN")
	}

	dbName := strings.TrimPrefix(u.Path, "/")
	if dbName == "" {
		return "", fmt.Errorf("missing database name in DSN")
	}

	user := ""
	if u.User != nil {
		user = u.User.String() + "@"
	}

	q := u.Query()
	q.Set("parseTime", "true")
	q.Set("loc", "UTC")
	q.Set("multiStatements", "false")

	return fmt.Sprintf("%stcp(%s)/%s?%s", user, u.Host, dbName, q.Encode()), nil
}
