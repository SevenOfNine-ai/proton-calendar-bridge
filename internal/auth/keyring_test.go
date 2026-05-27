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

func TestKeyringManagerUnlockAddressKeysEmptyPassword(t *testing.T) {
	t.Parallel()

	km := NewKeyringManager(fakeKeyringClient{addresses: []proton.Address{}})
	if _, err := km.UnlockAddressKeys(context.Background(), []byte("")); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestKeyringManagerUnlockAddressKeysNilClient(t *testing.T) {
	t.Parallel()

	km := NewKeyringManager(nil)
	if _, err := km.UnlockAddressKeys(context.Background(), []byte("pass")); err == nil {
		t.Fatal("expected error for nil client")
	}
}

func TestKeyringManagerUnlockAddressKeysNoAddresses(t *testing.T) {
	t.Parallel()

	km := NewKeyringManager(fakeKeyringClient{addresses: []proton.Address{}})
	if _, err := km.UnlockAddressKeys(context.Background(), []byte("pass")); err == nil {
		t.Fatal("expected error for no addresses")
	}
}

func TestKeyringManagerUnlockAddressKeysAddressesNoKeys(t *testing.T) {
	t.Parallel()

	km := NewKeyringManager(fakeKeyringClient{addresses: []proton.Address{{ID: "a1", Keys: proton.Keys{}}}})
	if _, err := km.UnlockAddressKeys(context.Background(), []byte("pass")); err == nil {
		t.Fatal("expected error when addresses have no keys")
	}
}

func TestKeyringManagerUnlockAddressKeyRingEmptyInput(t *testing.T) {
	t.Parallel()

	km := &KeyringManager{}
	if _, err := km.UnlockAddressKeyRing(proton.Keys{}, []byte("pass")); err == nil {
		t.Fatal("expected error for empty keys")
	}
	if _, err := km.UnlockAddressKeyRing(proton.Keys{}, []byte("")); err == nil {
		t.Fatal("expected error for empty password")
	}
}

func TestNewKeyringManager(t *testing.T) {
	t.Parallel()

	km := NewKeyringManager(fakeKeyringClient{addresses: []proton.Address{}})
	if km == nil {
		t.Fatal("expected non-nil keyring manager")
	}
	if km.client == nil {
		t.Fatal("expected non-nil client")
	}
}
