package usecase

import (
	"errors"
	"fmt"
	"testing"
)

func TestMarkNotRetryable(t *testing.T) {
	base := errors.New("denied")
	marked := fmt.Errorf("list admins: %w", MarkNotRetryable(base))
	if !IsNotRetryable(marked) || !errors.Is(marked, base) {
		t.Fatal("the mark must survive wrapping and keep the cause visible")
	}
	if IsNotRetryable(base) || IsNotRetryable(nil) || MarkNotRetryable(nil) != nil {
		t.Fatal("unmarked and nil errors are retryable / nil")
	}
}
