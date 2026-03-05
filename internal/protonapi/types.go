package protonapi

import proton "github.com/ProtonMail/go-proton-api"

type Auth = proton.Auth

type Calendar = proton.Calendar
type CalendarType = proton.CalendarType
type CalendarFlag = proton.CalendarFlag

const (
	CalendarTypeNormal = proton.CalendarTypeNormal
)

type CalendarKey = proton.CalendarKey
type CalendarKeys = proton.CalendarKeys

type CalendarMember = proton.CalendarMember
type CalendarPermissions = proton.CalendarPermissions

type CalendarPassphrase = proton.CalendarPassphrase
type MemberPassphrase = proton.MemberPassphrase

type CalendarEvent = proton.CalendarEvent
type CalendarEventPart = proton.CalendarEventPart
type CalendarEventType = proton.CalendarEventType

const (
	CalendarEventTypeClear     = proton.CalendarEventTypeClear
	CalendarEventTypeEncrypted = proton.CalendarEventTypeEncrypted
	CalendarEventTypeSigned    = proton.CalendarEventTypeSigned
)

type Address = proton.Address
type AddressStatus = proton.AddressStatus

type Key = proton.Key
type Keys = proton.Keys

// CalendarEventPartReq is the request body element for one encrypted/signed
// content section when creating or updating a Proton Calendar event.
type CalendarEventPartReq struct {
	// MemberID identifies the calendar member (required for PersonalEventContent).
	MemberID string `json:"MemberID,omitempty"`
	// Type is a bitmask: 0=clear, 1=encrypted, 2=signed, 3=encrypted+signed.
	Type int `json:"Type"`
	// Data is the base64-encoded data packet (for split encryption) or armored
	// PGP message (for personal events).
	Data string `json:"Data"`
	// Signature is the armored PGP detached signature of the plaintext.
	Signature string `json:"Signature,omitempty"`
	// Author is the email address of the signer.
	Author string `json:"Author,omitempty"`
}

// CreateCalendarEventReq is the request body for creating a new calendar event
// via POST /calendar/v1/{calendarID}/events.
type CreateCalendarEventReq struct {
	// MajorVersion of the calendar API schema (always 1).
	MajorVersion int `json:"MajorVersion"`
	// UID is the event's RFC 5545 UID. Must be globally unique.
	UID string `json:"UID"`
	// IsOrganizer indicates this client is the event organizer (1=true).
	IsOrganizer int `json:"IsOrganizer"`
	// Permissions bitmask for the event (3 = owner read+write).
	Permissions int `json:"Permissions"`
	// SharedKeyPacket is the base64-encoded encrypted session key for SharedEventContent.
	SharedKeyPacket string `json:"SharedKeyPacket"`
	// CalendarKeyPacket is typically empty for personal calendars.
	CalendarKeyPacket string `json:"CalendarKeyPacket,omitempty"`
	// SharedEventContent contains the encrypted shared VCALENDAR payload.
	SharedEventContent []CalendarEventPartReq `json:"SharedEventContent"`
	// CalendarEventContent is used for calendar-level metadata (typically empty).
	CalendarEventContent []CalendarEventPartReq `json:"CalendarEventContent,omitempty"`
	// PersonalEventContent contains the encrypted personal payload (alarms).
	PersonalEventContent []CalendarEventPartReq `json:"PersonalEventContent,omitempty"`
	// AttendeesEventContent contains per-attendee encrypted data (optional).
	AttendeesEventContent []CalendarEventPartReq `json:"AttendeesEventContent,omitempty"`
	// StartTime is the Unix timestamp of the event start.
	StartTime int64 `json:"StartTime"`
	// StartTimezone is the IANA timezone name for the start time.
	StartTimezone string `json:"StartTimezone"`
	// EndTime is the Unix timestamp of the event end.
	EndTime int64 `json:"EndTime"`
	// EndTimezone is the IANA timezone name for the end time.
	EndTimezone string `json:"EndTimezone"`
	// FullDay is 1 if this is an all-day event, 0 otherwise.
	FullDay int `json:"FullDay"`
	// RRule is the RFC 5545 recurrence rule string (without the "RRULE:" prefix).
	RRule string `json:"RRule,omitempty"`
	// Attendees is the list of event attendees.
	Attendees []interface{} `json:"Attendees,omitempty"`
}
