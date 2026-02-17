package crypto

import (
	"encoding/base64"
	"testing"

	proton "github.com/ProtonMail/go-proton-api"
	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
)

func TestEventDecryptorDecryptEvent(t *testing.T) {
	t.Parallel()

	calKey, err := gopenpgp.GenerateKey("Calendar", "calendar@example.com", "x25519", 0)
	if err != nil {
		t.Fatalf("generate calendar key: %v", err)
	}
	calKR, err := gopenpgp.NewKeyRing(calKey)
	if err != nil {
		t.Fatalf("calendar keyring: %v", err)
	}

	addrKey, err := gopenpgp.GenerateKey("Address", "address@example.com", "x25519", 0)
	if err != nil {
		t.Fatalf("generate address key: %v", err)
	}
	addrKR, err := gopenpgp.NewKeyRing(addrKey)
	if err != nil {
		t.Fatalf("address keyring: %v", err)
	}

	sharedPayload := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Standup\nDTSTART:20260216T090000Z\nDTEND:20260216T093000Z\nEND:VEVENT\nEND:VCALENDAR"
	personalPayload := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nBEGIN:VALARM\nTRIGGER:-PT15M\nEND:VALARM\nEND:VEVENT\nEND:VCALENDAR"

	sharedEnc, err := calKR.Encrypt(gopenpgp.NewPlainMessageFromString(sharedPayload), nil)
	if err != nil {
		t.Fatalf("encrypt shared: %v", err)
	}
	sharedSplit, err := sharedEnc.SplitMessage()
	if err != nil {
		t.Fatalf("split shared message: %v", err)
	}

	personalEnc, err := calKR.Encrypt(gopenpgp.NewPlainMessageFromString(personalPayload), nil)
	if err != nil {
		t.Fatalf("encrypt personal: %v", err)
	}
	personalArmored, err := personalEnc.GetArmored()
	if err != nil {
		t.Fatalf("armor personal message: %v", err)
	}

	event := proton.CalendarEvent{
		SharedKeyPacket: base64.StdEncoding.EncodeToString(sharedSplit.GetBinaryKeyPacket()),
		SharedEvents: []proton.CalendarEventPart{{
			Type: proton.CalendarEventTypeEncrypted,
			Data: base64.StdEncoding.EncodeToString(sharedSplit.GetBinaryDataPacket()),
		}},
		PersonalEvents: []proton.CalendarEventPart{{
			Type: proton.CalendarEventTypeEncrypted,
			Data: personalArmored,
		}},
	}

	decryptor := &EventDecryptor{}
	dec, err := decryptor.DecryptEvent(event, calKR, addrKR)
	if err != nil {
		t.Fatalf("decrypt event: %v", err)
	}

	if dec.SharedData != sharedPayload {
		t.Fatalf("unexpected shared payload: %q", dec.SharedData)
	}
	if dec.PersonalData != personalPayload {
		t.Fatalf("unexpected personal payload: %q", dec.PersonalData)
	}
}
