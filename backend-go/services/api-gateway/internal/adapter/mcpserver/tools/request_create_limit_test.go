package tools

import (
	"fmt"
	"testing"
	"time"
)

func TestRequestCreateLimiter_BucketRefillsHourly(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newRequestCreateLimiter(20, func() time.Time { return now })
	for i := 0; i < 20; i++ {
		if ok, _ := l.allow("t", "u", "c"); !ok {
			t.Fatalf("call %d refused", i+1)
		}
	}
	ok, wait := l.allow("t", "u", "c")
	if ok || wait < 2*time.Minute+50*time.Second || wait > 3*time.Minute+10*time.Second {
		t.Fatalf("21st: ok=%v wait=%v, want refusal with about 3m", ok, wait)
	}
	now = now.Add(3 * time.Minute)
	if ok, _ := l.allow("t", "u", "c"); !ok {
		t.Error("one token must be back after 3 minutes")
	}
	if ok, _ := l.allow("t", "u", "c"); ok {
		t.Error("only one token should have refilled")
	}
}

func TestRequestCreateLimiter_KeysAreIndependent(t *testing.T) {
	l := newRequestCreateLimiter(1, time.Now)
	for _, k := range [][3]string{{"t1", "u", "c"}, {"t2", "u", "c"}, {"t1", "v", "c"}, {"t1", "u", "other"}} {
		if ok, _ := l.allow(k[0], k[1], k[2]); !ok {
			t.Errorf("first call of %v refused", k)
		}
	}
	if ok, _ := l.allow("t1", "u", "c"); ok {
		t.Error("second call of the same key must be refused")
	}
}

func TestRequestCreateLimiter_DisabledWhenZeroOrNil(t *testing.T) {
	var nilLimiter *requestCreateLimiter
	if ok, _ := nilLimiter.allow("t", "u", "c"); !ok {
		t.Error("nil limiter must allow")
	}
	l := newRequestCreateLimiter(-1, time.Now)
	for i := 0; i < 100; i++ {
		if ok, _ := l.allow("t", "u", "c"); !ok {
			t.Fatal("disabled limiter refused")
		}
	}
}

func TestRequestCreateLimiter_MemoryIsBounded(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := newRequestCreateLimiter(5, func() time.Time { return now })
	for i := 0; i < maxRateBuckets*2; i++ {
		l.allow("t", fmt.Sprintf("u%d", i), "c")
	}
	if len(l.buckets) > maxRateBuckets {
		t.Fatalf("buckets = %d, cap %d", len(l.buckets), maxRateBuckets)
	}
}

func TestApplyEnv_RequestCreatePerHour(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "MCP_REQUEST_CREATE_PER_HOUR" {
				return v
			}
			return ""
		}
	}
	c, err := DefaultConfig().ApplyEnv(env(""))
	if err != nil || c.withDefaults().RequestCreatePerHour != 20 {
		t.Fatalf("default = %d err=%v, want 20", c.withDefaults().RequestCreatePerHour, err)
	}
	c, err = DefaultConfig().ApplyEnv(env("7"))
	if err != nil || c.RequestCreatePerHour != 7 {
		t.Fatalf("7 -> %d err=%v", c.RequestCreatePerHour, err)
	}
	c, err = DefaultConfig().ApplyEnv(env("0"))
	if err != nil || c.withDefaults().RequestCreatePerHour >= 0 {
		t.Fatalf("0 must disable, got %d err=%v", c.withDefaults().RequestCreatePerHour, err)
	}
	for _, bad := range []string{"x", "-1", "1.5"} {
		if _, err := DefaultConfig().ApplyEnv(env(bad)); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}
