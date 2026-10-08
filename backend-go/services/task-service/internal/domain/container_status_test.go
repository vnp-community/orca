package domain

import "testing"

func TestDeriveContainerStatus(t *testing.T) {
	o, b, ip, r, d, c := StatusOpen, StatusBlocked, StatusInProgress, StatusReview, StatusDone, StatusCancelled
	cases := []struct {
		name     string
		children []Status
		want     Status
		ok       bool
	}{
		{"no children keeps current", nil, "", false},
		{"all cancelled", []Status{c, c}, StatusCancelled, true},
		{"all done", []Status{d, d}, StatusDone, true},
		{"cancelled ignored when done", []Status{d, c}, StatusDone, true},
		{"any in progress", []Status{o, ip, d}, StatusInProgress, true},
		{"done and review with review", []Status{d, r}, StatusReview, true},
		{"only review", []Status{r, r}, StatusReview, true},
		{"done mixed with open", []Status{d, o}, StatusInProgress, true},
		{"review mixed with blocked", []Status{r, b}, StatusInProgress, true},
		{"all blocked", []Status{b, b}, StatusBlocked, true},
		{"blocked and open", []Status{b, o}, StatusOpen, true},
		{"all open", []Status{o, o}, StatusOpen, true},
		{"open with cancelled", []Status{o, c}, StatusOpen, true},
	}
	for _, tc := range cases {
		got, ok := DeriveContainerStatus(tc.children)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: got (%q,%v) want (%q,%v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}
