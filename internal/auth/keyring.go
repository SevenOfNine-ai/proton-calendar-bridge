package auth

import (
	"context"
	"fmt"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type keyringClient interface {
	GetAddresses(ctx context.Context) ([]protonapi.Address, error)
}

type KeyringManager struct {
	client keyringClient
}

func NewKeyringManager(client keyringClient) *KeyringManager {
	return &KeyringManager{client: client}
}

func (km *KeyringManager) UnlockAddressKeys(ctx context.Context, keyPassword []byte) (*crypto.KeyRing, error) {
	if km == nil || km.client == nil {
		return nil, fmt.Errorf("keyring client is not configured")
	}
	if len(keyPassword) == 0 {
		return nil, fmt.Errorf("key password is required")
	}

	addresses, err := km.client.GetAddresses(ctx)
	if err != nil {
		return nil, fmt.Errorf("get addresses: %w", err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("no proton addresses returned")
	}

	kr, err := crypto.NewKeyRing(nil)
	if err != nil {
		return nil, fmt.Errorf("create keyring: %w", err)
	}

	unlocked := 0
	for _, addr := range addresses {
		if len(addr.Keys) == 0 {
			continue
		}
		addrKR, err := km.UnlockAddressKeyRing(addr.Keys, keyPassword)
		if err != nil {
			continue
		}
		for _, key := range addrKR.GetKeys() {
			if err := kr.AddKey(key); err != nil {
				return nil, fmt.Errorf("add key for address %s: %w", addr.ID, err)
			}
			unlocked++
		}
	}

	if unlocked == 0 {
		return nil, fmt.Errorf("failed to unlock any address keys")
	}
	return kr, nil
}

func (km *KeyringManager) UnlockAddressKeyRing(addressKeys protonapi.Keys, keyPassword []byte) (*crypto.KeyRing, error) {
	if len(addressKeys) == 0 {
		return nil, fmt.Errorf("address keys are required")
	}
	if len(keyPassword) == 0 {
		return nil, fmt.Errorf("key password is required")
	}
	kr, err := addressKeys.Unlock(keyPassword, nil)
	if err != nil {
		return nil, fmt.Errorf("unlock address keys: %w", err)
	}
	if kr == nil || len(kr.GetKeys()) == 0 {
		return nil, fmt.Errorf("no unlocked address keys")
	}
	return kr, nil
}
