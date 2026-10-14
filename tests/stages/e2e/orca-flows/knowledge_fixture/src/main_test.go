package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNotesRoundTrip(t *testing.T) {
	mux := newMux(&store{})
	post := httptest.NewRecorder()
	mux.ServeHTTP(post, httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"text":"hello"}`)))
	if post.Code != http.StatusCreated {
		t.Fatalf("post: %d", post.Code)
	}
	get := httptest.NewRecorder()
	mux.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/notes", nil))
	if !strings.Contains(get.Body.String(), "hello") {
		t.Fatalf("get: %s", get.Body.String())
	}
}

func TestLongNoteIsRefused(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux(&store{}).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notes", strings.NewReader(`{"text":"`+strings.Repeat("a", 281)+`"}`)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code %d", rec.Code)
	}
}
