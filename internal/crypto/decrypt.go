package crypto

import (
	"encoding/base64"
	"fmt"
	"strings"

	gopenpgp "github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/sevenofnine/proton-calendar-bridge/internal/protonapi"
)

type EventDecryptor struct{}

type DecryptedEvent struct {
	SharedData   string
	PersonalData string
	CalendarData string
}

func (d *EventDecryptor) DecryptEvent(event protonapi.CalendarEvent, calKR, addrKR *gopenpgp.KeyRing) (DecryptedEvent, error) {
	if calKR == nil {
		return DecryptedEvent{}, fmt.Errorf("calendar keyring is required")
	}
	if addrKR == nil {
		return DecryptedEvent{}, fmt.Errorf("address keyring is required")
	}

	shared, err := decryptEventParts(event.SharedEvents, calKR, addrKR, event.SharedKeyPacket)
	if err != nil {
		return DecryptedEvent{}, fmt.Errorf("decrypt shared parts: %w", err)
	}

	personal, err := decryptEventParts(event.PersonalEvents, calKR, addrKR, "")
	if err != nil {
		return DecryptedEvent{}, fmt.Errorf("decrypt personal parts: %w", err)
	}

	calendar, err := decryptEventParts(event.CalendarEvents, calKR, addrKR, "")
	if err != nil {
		return DecryptedEvent{}, fmt.Errorf("decrypt calendar parts: %w", err)
	}

	return DecryptedEvent{SharedData: shared, PersonalData: personal, CalendarData: calendar}, nil
}

func decryptEventParts(parts []protonapi.CalendarEventPart, calKR, addrKR *gopenpgp.KeyRing, sharedKeyPacket string) (string, error) {
	var kp []byte
	if sharedKeyPacket != "" {
		decoded, err := base64.StdEncoding.DecodeString(sharedKeyPacket)
		if err != nil {
			return "", fmt.Errorf("decode shared key packet: %w", err)
		}
		kp = decoded
	}

	out := make([]string, 0, len(parts))
	for _, part := range parts {
		plain, err := decodePart(part, calKR, addrKR, kp)
		if err != nil {
			return "", err
		}
		if plain != "" {
			out = append(out, plain)
		}
	}

	return strings.Join(out, "\n"), nil
}

// decodePart mirrors upstream CalendarEventPart.Decode behavior while returning
// the decoded payload. Upstream uses a value receiver, so part.Data mutations are
// not observable by callers.
func decodePart(part protonapi.CalendarEventPart, calKR, addrKR *gopenpgp.KeyRing, kp []byte) (string, error) {
	data := part.Data

	if part.Type&protonapi.CalendarEventTypeEncrypted != 0 {
		var enc *gopenpgp.PGPMessage
		if kp != nil {
			raw, err := base64.StdEncoding.DecodeString(part.Data)
			if err != nil {
				return "", err
			}
			enc = gopenpgp.NewPGPSplitMessage(kp, raw).GetPGPMessage()
		} else {
			msg, err := gopenpgp.NewPGPMessageFromArmored(part.Data)
			if err != nil {
				return "", err
			}
			enc = msg
		}

		dec, err := calKR.Decrypt(enc, nil, gopenpgp.GetUnixTime())
		if err != nil {
			return "", err
		}
		data = dec.GetString()
	}

	if part.Type&protonapi.CalendarEventTypeSigned != 0 {
		sig, err := gopenpgp.NewPGPSignatureFromArmored(part.Signature)
		if err != nil {
			return "", err
		}
		if err := addrKR.VerifyDetached(gopenpgp.NewPlainMessageFromString(data), sig, gopenpgp.GetUnixTime()); err != nil {
			return "", err
		}
	}

	return data, nil
}
