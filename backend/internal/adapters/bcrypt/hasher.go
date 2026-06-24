package bcrypt

import (
	"errors"

	xbcrypt "golang.org/x/crypto/bcrypt"

	"filemepls/internal/domain"
	"filemepls/internal/ports"
)

var _ ports.PasswordHasher = (*Hasher)(nil)

// Hasher implements ports.PasswordHasher using bcrypt.
type Hasher struct {
	cost int
}

func New() *Hasher {
	return &Hasher{cost: xbcrypt.DefaultCost}
}

func NewWithCost(cost int) *Hasher {
	return &Hasher{cost: cost}
}

func (h *Hasher) Hash(plain string) (string, error) {
	hash, err := xbcrypt.GenerateFromPassword([]byte(plain), h.cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (h *Hasher) Verify(hash, plain string) error {
	err := xbcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
	if errors.Is(err, xbcrypt.ErrMismatchedHashAndPassword) {
		return domain.ErrInvalidPassword
	}
	return err
}
