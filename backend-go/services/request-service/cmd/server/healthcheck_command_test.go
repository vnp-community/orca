package main

import (
	"bytes"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stablyai/orca-go/common/health"
)

func portOf(t *testing.T, srv *httptest.Server) int {
	t.Helper()
	_, p, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	n, _ := strconv.Atoi(p)
	return n
}

func TestRunHealthcheck(t *testing.T) {
	h := health.New()
	srv := httptest.NewServer(h.Handler())
	defer srv.Close()

	var out bytes.Buffer
	if code := runHealthcheck(portOf(t, srv), &out); code != 0 || out.Len() != 0 {
		t.Fatalf("ready service: exit %d, output %q", code, out.String())
	}

	h.Register("opa_bundle", func() error { return errors.New("bundle gone") })
	out.Reset()
	if code := runHealthcheck(portOf(t, srv), &out); code != 1 || !strings.Contains(out.String(), "opa_bundle") {
		t.Fatalf("not-ready service: exit %d, output %q", code, out.String())
	}
}

func TestRunHealthcheck_NothingListening(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	port := portOf(t, srv)
	srv.Close()
	var out bytes.Buffer
	if code := runHealthcheck(port, &out); code != 1 || out.Len() == 0 {
		t.Fatalf("exit %d, output %q", code, out.String())
	}
}
