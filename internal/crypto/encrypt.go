package crypto

import (
	"encoding/base64"
	"fmt"

	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
)

// EncryptedSharedEvent holds the encrypted components required for the Proton API
// SharedEventContent field when creating or updating a calendar event.
type EncryptedSharedEvent struct {
	// KeyPacket is the base64-encoded encrypted session key, locked to the calendar
	// key ring. Sent as SharedKeyPacket in the Proton API request.
	KeyPacket string
	// DataPacket is the base64-encoded ciphertext of the VCALENDAR data.
	// Sent as SharedEventContent[0].Data in the Proton API request.
	DataPacket string
	// Signature is the armored PGP detached signature of the plaintext VCALENDAR,
	// created with the address private key.
	// Sent as SharedEventContent[0].Signature.
	Signature string
}

// EncryptedPersonalEvent holds the encrypted components for the personal event
// payload (reminders/alarms), encrypted to the address key ring.
type EncryptedPersonalEvent struct {
	// Data is the armored PGP message (encrypted + signed with address key ring).
	Data string
	// Signature is the armored PGP detached signature.
	Signature string
}

// EncryptSharedEvent encrypts a VCALENDAR string using a fresh session key locked
// to calKR (the calendar key ring), then signs the plaintext with addrKR.
//
// The resulting KeyPacket and DataPacket mirror the split-key format used by the
// Proton web client and expected by the Proton Calendar API.
func EncryptSharedEvent(vcalendar string, calKR, addrKR *gopenpgp.KeyRing) (EncryptedSharedEvent, error) {
	if calKR == nil {
		return EncryptedSharedEvent{}, fmt.Errorf("calendar keyring is required")
	}
	if addrKR == nil {
		return EncryptedSharedEvent{}, fmt.Errorf("address keyring is required")
	}

	// Generate a fresh AES-256 session key.
	sk, err := gopenpgp.GenerateSessionKeyAlgo("aes256")
	if err != nil {
		return EncryptedSharedEvent{}, fmt.Errorf("generate session key: %w", err)
	}

	// Encrypt the session key for the calendar key ring recipients.
	keyPacketBytes, err := calKR.EncryptSessionKey(sk)
	if err != nil {
		return EncryptedSharedEvent{}, fmt.Errorf("encrypt session key: %w", err)
	}

	// Encrypt the VCALENDAR plaintext with the session key.
	plain := gopenpgp.NewPlainMessageFromString(vcalendar)
	dataPacketBytes, err := sk.Encrypt(plain)
	if err != nil {
		return EncryptedSharedEvent{}, fmt.Errorf("encrypt data with session key: %w", err)
	}

	// Sign the plaintext with the address private key (detached signature).
	sig, err := addrKR.SignDetached(plain)
	if err != nil {
		return EncryptedSharedEvent{}, fmt.Errorf("sign shared event: %w", err)
	}
	sigArmored, err := sig.GetArmored()
	if err != nil {
		return EncryptedSharedEvent{}, fmt.Errorf("armor signature: %w", err)
	}

	return EncryptedSharedEvent{
		KeyPacket:  base64.StdEncoding.EncodeToString(keyPacketBytes),
		DataPacket: base64.StdEncoding.EncodeToString(dataPacketBytes),
		Signature:  sigArmored,
	}, nil
}

// EncryptPersonalEvent encrypts a personal VCALENDAR string (containing alarms)
// using the address key ring for both encryption and signing. Returns an armored
// PGP message and a separate detached armored signature.
//
// If vcalendar is empty, returns zero-value EncryptedPersonalEvent without error.
func EncryptPersonalEvent(vcalendar string, addrKR *gopenpgp.KeyRing) (EncryptedPersonalEvent, error) {
	if vcalendar == "" {
		return EncryptedPersonalEvent{}, nil
	}
	if addrKR == nil {
		return EncryptedPersonalEvent{}, fmt.Errorf("address keyring is required")
	}

	plain := gopenpgp.NewPlainMessageFromString(vcalendar)

	// Encrypt and sign with the address key ring.
	pgpMsg, err := addrKR.Encrypt(plain, addrKR)
	if err != nil {
		return EncryptedPersonalEvent{}, fmt.Errorf("encrypt personal event: %w", err)
	}
	armored, err := pgpMsg.GetArmored()
	if err != nil {
		return EncryptedPersonalEvent{}, fmt.Errorf("armor personal event: %w", err)
	}

	// Detached signature of the plaintext.
	sig, err := addrKR.SignDetached(plain)
	if err != nil {
		return EncryptedPersonalEvent{}, fmt.Errorf("sign personal event: %w", err)
	}
	sigArmored, err := sig.GetArmored()
	if err != nil {
		return EncryptedPersonalEvent{}, fmt.Errorf("armor personal signature: %w", err)
	}

	return EncryptedPersonalEvent{
		Data:      armored,
		Signature: sigArmored,
	}, nil
}
