// Package bcrypt implements usecase.PasswordHasher via golang.org/x/crypto/bcrypt
// — the one place in auth-service that imports a bcrypt package, per
// auth-service.md §6 ("bcrypt lives behind this port in adapter/crypto/,
// never imported by domain/"). Package name here is bcrypt (not crypto) to
// match this service's file/module naming convention of naming a package
// after the concrete thing it wraps.
package bcrypt

import (
	"fmt"

	xbcrypt "golang.org/x/crypto/bcrypt"
)

// MinCost is the minimum bcrypt cost this service allows — auth-service.md
// §9: "Bcrypt, 12 rounds minimum ... Cost factor is a config value, not a
// compile-time constant, so it can be raised as hardware gets faster
// without a code change." New enforces this floor regardless of what a
// misconfigured cost value requests.
const MinCost = 12

// Hasher implements usecase.PasswordHasher.
type Hasher struct {
	cost int
}

// New returns a Hasher with the given bcrypt cost, raised to MinCost if the
// caller passes anything lower (including bcrypt's own low-security
// defaults) — this service never hashes below the security-review floor.
func New(cost int) *Hasher {
	if cost < MinCost {
		cost = MinCost
	}
	return &Hasher{cost: cost}
}

func (h *Hasher) Hash(password string) (string, error) {
	hash, err := xbcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: hashing password: %w", err)
	}
	return string(hash), nil
}

func (h *Hasher) Compare(hash, password string) error {
	if err := xbcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return fmt.Errorf("bcrypt: comparing password: %w", err)
	}
	return nil
}
