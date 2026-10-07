package domain

import (
	"testing"
)

func TestOpenSpecChange_StateMachine(t *testing.T) {
	c := OpenSpecChange{Status: ChangeStatusPreparing}

	if err := c.MarkArchived("abc"); err == nil {
		t.Errorf("preparing -> archived should fail")
	}

	if err := c.MarkReady(); err != nil {
		t.Errorf("preparing -> ready should succeed")
	}
	if c.Status != ChangeStatusReady {
		t.Errorf("status should be ready")
	}

	if err := c.MarkReady(); err == nil {
		t.Errorf("ready -> ready should fail")
	}

	if err := c.MarkArchived("def"); err != nil {
		t.Errorf("ready -> archived should succeed")
	}
	if c.Commit != "def" {
		t.Errorf("commit should be def")
	}

	c2 := OpenSpecChange{Status: ChangeStatusPreparing}
	if err := c2.Abandon(); err != nil {
		t.Errorf("preparing -> abandoned should succeed")
	}

	c3 := OpenSpecChange{Status: ChangeStatusReady}
	if err := c3.Abandon(); err != nil {
		t.Errorf("ready -> abandoned should succeed")
	}

	c4 := OpenSpecChange{Status: ChangeStatusArchived}
	if err := c4.Abandon(); err == nil {
		t.Errorf("archived -> abandoned should fail")
	}
}

func TestNewChangeID_Cases(t *testing.T) {
	cases := []struct {
		number int64
		title  string
		want   string
	}{
		{142, "Thêm đăng nhập bằng Google", "req-142-them-dang-nhap-bang-google"},
		{7, "Đ", "req-7-d"},
		{8, "😊", "req-8-request"},
		{9, "!@#$%^&*", "req-9-request"},
		{10, "", "req-10-request"},
		{11, "a - b - c", "req-11-a-b-c"},
		{12, "01234567890123456789012345678901234567890123456789", "req-12-0123456789012345678901234567890123456789"}, // exact 40 slug
	}

	for _, tc := range cases {
		got := NewChangeID(tc.number, tc.title)
		if got != tc.want {
			t.Errorf("NewChangeID(%d, %q) = %q, want %q", tc.number, tc.title, got, tc.want)
		}
		if !ValidChangeID(got) {
			t.Errorf("result %q is invalid", got)
		}
	}
}
