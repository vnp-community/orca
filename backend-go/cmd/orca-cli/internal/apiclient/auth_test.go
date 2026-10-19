package apiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogin_ReadsTheExpiryInEitherShape(t *testing.T) {
	for name, body := range map[string]string{
		"protobuf timestamp": `{"jwt":"tok","expires_at":{"seconds":1790000000,"nanos":5}}`,
		"rfc3339 string":     `{"jwt":"tok","expires_at":"2026-09-21T08:53:20Z"}`,
	} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()
			res, err := Login(context.Background(), New(srv.URL, ""), "a@b.c", "pw")
			if err != nil || res.JWT != "tok" || res.ExpiresAt == "" {
				t.Fatalf("res=%+v err=%v", res, err)
			}
		})
	}
}
