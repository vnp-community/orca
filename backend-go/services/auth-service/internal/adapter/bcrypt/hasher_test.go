package bcrypt

import "testing"

func TestHasher_HashAndCompareRoundTrip(t *testing.T) {
	h := New(MinCost)
	hash, err := h.Hash("correct-password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := h.Compare(hash, "correct-password"); err != nil {
		t.Errorf("expected the correct password to compare successfully: %v", err)
	}
	if err := h.Compare(hash, "wrong-password"); err == nil {
		t.Error("expected the wrong password to fail comparison")
	}
}

func TestNew_EnforcesMinimumCost(t *testing.T) {
	h := New(4) // well below MinCost
	if h.cost != MinCost {
		t.Errorf("expected cost to be floored at %d, got %d", MinCost, h.cost)
	}
}
