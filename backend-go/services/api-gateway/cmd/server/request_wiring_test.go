package main

import (
	"os"
	"testing"
)

func TestDialRequestService_WithToken(t *testing.T) {
	os.Setenv("REQUEST_INTERNAL_CALLER_TOKEN", "secret-token")
	defer os.Unsetenv("REQUEST_INTERNAL_CALLER_TOKEN")

	conn, err := dialRequestService("localhost:0")
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	conn.Close()
	// Just verifies it doesn't panic and accepts the interceptors
}

func TestDialRequestService_EmptyToken(t *testing.T) {
	os.Unsetenv("REQUEST_INTERNAL_CALLER_TOKEN")

	conn, err := dialRequestService("localhost:0")
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	conn.Close()
}
