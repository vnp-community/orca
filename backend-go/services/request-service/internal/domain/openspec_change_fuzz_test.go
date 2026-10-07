package domain

import (
	"testing"
)

func FuzzNewChangeID(f *testing.F) {
	f.Add(int64(1), "test")
	f.Add(int64(100), "Thêm tính năng đăng nhập")
	f.Add(int64(0), "")
	f.Add(int64(999), "emoji 🚀 and special #!chars")

	f.Fuzz(func(t *testing.T, number int64, title string) {
		if number < 0 {
			number = -number
		}
		got := NewChangeID(number, title)
		if !ValidChangeID(got) {
			t.Errorf("generated invalid change ID: %q for title %q", got, title)
		}
	})
}
