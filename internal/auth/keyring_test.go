package auth

import (
	"context"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
)

type fakeKeyringClient struct {
	addresses []proton.Address
}

func (f fakeKeyringClient) GetAddresses(context.Context) ([]proton.Address, error) {
	return f.addresses, nil
}

func TestKeyringManagerUnlockAddressKeyRing(t *testing.T) {
	t.Parallel()

	pass := []byte("test-pass")
	key, err := gopenpgp.GenerateKey("Tester", "tester@example.com", "x25519", 0)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	locked, err := key.Lock(pass)
	if err != nil {
		t.Fatalf("lock key: %v", err)
	}
	serialized, err := locked.Serialize()
	if err != nil {
		t.Fatalf("serialize key: %v", err)
	}

	km := NewKeyringManager(fakeKeyringClient{addresses: []proton.Address{{ID: "a1", Keys: proton.Keys{{PrivateKey: serialized, Active: proton.Bool(true)}}}}})

	kr, err := km.UnlockAddressKeys(context.Background(), pass)
	if err != nil {
		t.Fatalf("unlock address keys: %v", err)
	}
	if len(kr.GetKeys()) == 0 {
		t.Fatal("expected unlocked keys")
	}
}
