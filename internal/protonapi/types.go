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
