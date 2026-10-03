package metrics

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
)

type fakeKills struct {
	m   map[string]int64
	err error
}

func (f *fakeKills) CountActiveKillSwitches(context.Context) (map[string]int64, error) {
	return f.m, f.err
}

func TestKillSwitchGaugeByScopeKeepsLastValueOnError(t *testing.T) {
	fk := &fakeKills{m: map[string]int64{"tenant": 2, "session": 1, "bogus": 9}}
	s := New(nil, slog.New(slog.NewTextHandler(io.Discard, nil))).WithKillSwitches(fk)
	if body := scrape(s); !strings.Contains(body, `orca_mcp_killswitch_active{scope="tenant"} 0`) {
		t.Fatalf("series must exist at 0 before the first sample (alerts need a value):\n%s", body)
	}
	s.Refresh(context.Background())
	body := scrape(s)
	for _, want := range []string{`orca_mcp_killswitch_active{scope="tenant"} 2`, `orca_mcp_killswitch_active{scope="session"} 1`, `orca_mcp_killswitch_active{scope="client"} 0`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "bogus") {
		t.Fatal("unknown scopes must never become label values")
	}
	fk.m, fk.err = map[string]int64{}, errors.New("db down")
	s.Refresh(context.Background())
	body = scrape(s)
	if !strings.Contains(body, `orca_mcp_killswitch_active{scope="tenant"} 2`) || !strings.Contains(body, "orca_mcp_sessions_count_errors_total 1") {
		t.Fatalf("failed sample keeps last value and counts an error:\n%s", body)
	}
	fk.err = nil
	s.Refresh(context.Background())
	if !strings.Contains(scrape(s), `orca_mcp_killswitch_active{scope="tenant"} 0`) {
		t.Fatal("cleared switches must drop back to 0")
	}
}
