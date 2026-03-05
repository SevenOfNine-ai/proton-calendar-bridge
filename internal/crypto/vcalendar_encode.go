package crypto

import (
	"fmt"
	"strings"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
)

// EncodeSharedVCalendar builds the VCALENDAR/VEVENT iCal text for the shared
// event payload (title, times, location, description, recurrence, attendees).
// This is the inverse of ParseVCalendar for the shared data portion.
func EncodeSharedVCalendar(m domain.EventMutation, uid string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//proton-calendar-bridge//EN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	fmt.Fprintf(&b, "DTSTART%s\r\n", formatICalDateTime(m.Start, m.AllDay))
	fmt.Fprintf(&b, "DTEND%s\r\n", formatICalDateTime(m.End, m.AllDay))
	if m.Title != "" {
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", escapeICalText(m.Title))
	}
	if m.Description != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", escapeICalText(m.Description))
	}
	if m.Location != "" {
		fmt.Fprintf(&b, "LOCATION:%s\r\n", escapeICalText(m.Location))
	}
	if m.Recurrence != "" {
		fmt.Fprintf(&b, "RRULE:%s\r\n", m.Recurrence)
	}
	for _, attendee := range m.Attendees {
		fmt.Fprintf(&b, "ATTENDEE:%s\r\n", attendee)
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// EncodePersonalVCalendar builds the personal VCALENDAR payload containing
// VALARM (reminder) blocks. Returns empty string if no reminders.
func EncodePersonalVCalendar(reminders []string, uid string) string {
	if len(reminders) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//proton-calendar-bridge//EN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", uid)
	for _, trigger := range reminders {
		b.WriteString("BEGIN:VALARM\r\n")
		b.WriteString("ACTION:DISPLAY\r\n")
		fmt.Fprintf(&b, "TRIGGER:%s\r\n", trigger)
		b.WriteString("END:VALARM\r\n")
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// formatICalDateTime formats a time.Time for use in DTSTART/DTEND properties.
// Returns ":VALUE=DATE:YYYYMMDD" for all-day events, or
// ":TZID=UTC:YYYYMMDDTHHmmssZ" style for timed events.
func formatICalDateTime(t time.Time, allDay bool) string {
	if allDay {
		return fmt.Sprintf(";VALUE=DATE:%s", t.UTC().Format("20060102"))
	}
	return fmt.Sprintf(":%s", t.UTC().Format("20060102T150405Z"))
}

// escapeICalText escapes special characters in iCal TEXT values per RFC 5545 §3.3.11.
func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
