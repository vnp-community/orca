package fcm

import (
	"net/http"
	"testing"
)

func TestIsTokenInvalid(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
		body       string
		want       bool
	}{
		{"404 UNREGISTERED is invalid", http.StatusNotFound, `{"error":{"code":404,"status":"UNREGISTERED"}}`, true},
		{"404 with empty body is still invalid (documented default)", http.StatusNotFound, ``, true},
		{"404 with unparseable body is still invalid (documented default)", http.StatusNotFound, `not json`, true},
		{"404 with a different explicit status is NOT invalid", http.StatusNotFound, `{"error":{"code":404,"status":"NOT_FOUND"}}`, false},
		{"400 INVALID_ARGUMENT is NOT invalid", http.StatusBadRequest, `{"error":{"code":400,"status":"INVALID_ARGUMENT"}}`, false},
		{"401 UNAUTHENTICATED is NOT invalid", http.StatusUnauthorized, `{"error":{"code":401,"status":"UNAUTHENTICATED"}}`, false},
		{"5xx UNAVAILABLE is NOT invalid (transient)", http.StatusServiceUnavailable, `{"error":{"code":503,"status":"UNAVAILABLE"}}`, false},
		{"429 QUOTA_EXCEEDED is NOT invalid (transient)", http.StatusTooManyRequests, `{"error":{"code":429,"status":"RESOURCE_EXHAUSTED"}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isTokenInvalid(tc.statusCode, []byte(tc.body))
			if got != tc.want {
				t.Errorf("isTokenInvalid(%d, %q) = %v, want %v", tc.statusCode, tc.body, got, tc.want)
			}
		})
	}
}
