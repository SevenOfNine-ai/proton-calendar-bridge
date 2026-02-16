package auth

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type AddressKeyUnlocker interface {
	UnlockAddressKeys(ctx context.Context, username string, password string) ([]*crypto.KeyRing, error)
}

type CalendarPassphraseDecryptor interface {
	DecryptCalendarPassphrase(ctx context.Context, memberID string, encrypted string, keys []*crypto.KeyRing) (string, error)
}

type PlaceholderKeyring struct{}

func (PlaceholderKeyring) UnlockAddressKeys(context.Context, string, string) ([]*crypto.KeyRing, error) {
	// TODO(auth): load and unlock user address keys via gopenpgp.
	return nil, nil
}

func (PlaceholderKeyring) DecryptCalendarPassphrase(context.Context, string, string, []*crypto.KeyRing) (string, error) {
	// TODO(auth): decrypt calendar member passphrases via address keyring.
	return "", nil
}
