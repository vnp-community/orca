// Package nacl implements usecase.DeviceKeyExchanger over
// golang.org/x/crypto/nacl/box — the same NaCl crypto_box construction
// TweetNaCl.js implements client-side (BL-MB-01's security model), so no
// protocol redesign is needed on either side.
package nacl

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/nacl/box"
)

type KeyExchanger struct{}

func New() *KeyExchanger { return &KeyExchanger{} }

// GenerateEphemeralKeypair returns a fresh X25519 keypair for one pairing
// session — never reused across sessions.
func (KeyExchanger) GenerateEphemeralKeypair() (pub, priv []byte, err error) {
	pubKey, privKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("nacl: generating ephemeral keypair: %w", err)
	}
	return pubKey[:], privKey[:], nil
}

// SharedSecret computes the box.Precompute shared secret from this side's
// private key and the peer's public key — the desktop-side half of
// BL-MB-01's handshake diagram; the mobile side computes the byte-identical
// value via TweetNaCl's box.before.
func (KeyExchanger) SharedSecret(priv, peerPub []byte) ([]byte, error) {
	if len(priv) != 32 || len(peerPub) != 32 {
		return nil, fmt.Errorf("nacl: key must be 32 bytes")
	}
	var privArr, pubArr [32]byte
	copy(privArr[:], priv)
	copy(pubArr[:], peerPub)
	var shared [32]byte
	box.Precompute(&shared, &pubArr, &privArr)
	return shared[:], nil
}
