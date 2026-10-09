package health

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestProbe(t *testing.T) {
	ready := New()
	srv := httptest.NewServer(ready.Handler())
	defer srv.Close()
	ctx := context.Background()

	if err := Probe(ctx, srv.Client(), srv.URL+"/readyz"); err != nil {
		t.Fatalf("a ready service: %v", err)
	}

	ready.Register("opa_bundle", func() error { return errors.New("bundle gone") })
	err := Probe(ctx, srv.Client(), srv.URL+"/readyz")
	if err == nil || !strings.Contains(err.Error(), "503") || !strings.Contains(err.Error(), "opa_bundle") {
		t.Fatalf("a failing check must name itself, got %v", err)
	}
}

func TestProbe_UnreachableAndCancelled(t *testing.T) {
	srv := httptest.NewServer(New().Handler())
	url := srv.URL + "/readyz"
	client := srv.Client()
	srv.Close()
	if err := Probe(context.Background(), client, url); err == nil {
		t.Fatal("a stopped service must not probe as ready")
	}

	live := httptest.NewServer(New().Handler())
	defer live.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)
	if err := Probe(ctx, live.Client(), live.URL+"/readyz"); err == nil {
		t.Fatal("an expired context must fail the probe")
	}
}
