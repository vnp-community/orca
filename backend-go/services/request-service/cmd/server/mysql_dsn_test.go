package main

import (
	"net/url"
	"strings"
	"testing"
)

func TestToMySQLDriverDSN(t *testing.T) {
	cases := []struct {
		name    string
		dsn     string
		wantErr string
		want    []string
	}{
		{"mysql scheme", "mysql://u:p@db:3306/request", "", []string{"u:p@tcp(db:3306)/request?", "parseTime=true", "loc=UTC"}},
		{"tidb scheme", "tidb://u:p@db:4000/request", "", []string{"tcp(db:4000)/request?"}},
		{"keeps existing query", "mysql://u:p@db:3306/request?charset=utf8mb4", "", []string{"charset=utf8mb4", "parseTime=true"}},
		{"http scheme rejected", "http://db/request", "unrecognized DSN scheme", nil},
		{"missing host", "mysql:///request", "missing host", nil},
		{"missing database", "mysql://u:p@db:3306/", "missing database", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := toMySQLDriverDSN(c.dsn)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("want error %q, got %v", c.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			for _, w := range c.want {
				if !strings.Contains(got, w) {
					t.Errorf("dsn %q missing %q", got, w)
				}
			}
		})
	}
}

func TestToMySQLDriverDSN_PinsSessionTimeZoneToUTC(t *testing.T) {
	got, err := toMySQLDriverDSN("mysql://u:p@db:3306/request")
	if err != nil {
		t.Fatal(err)
	}
	q, err := url.ParseQuery(got[strings.Index(got, "?")+1:])
	if err != nil {
		t.Fatal(err)
	}
	if q.Get("time_zone") != "'+00:00'" {
		t.Errorf("time_zone = %q, want '+00:00' so TIMESTAMP columns do not depend on server tz", q.Get("time_zone"))
	}
}
