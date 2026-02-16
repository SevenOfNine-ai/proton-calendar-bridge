package crypto

import "testing"

func TestParseVCalendar(t *testing.T) {
	t.Parallel()

	shared := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nSUMMARY:Team Sync\nDESCRIPTION:Weekly status\nLOCATION:Room 42\nDTSTART:20260216T090000Z\nDTEND:20260216T100000Z\nRRULE:FREQ=WEEKLY;BYDAY=MO\nATTENDEE:mailto:a@example.com\nATTENDEE:mailto:b@example.com\nEND:VEVENT\nEND:VCALENDAR"
	personal := "BEGIN:VCALENDAR\nBEGIN:VEVENT\nBEGIN:VALARM\nTRIGGER:-PT10M\nEND:VALARM\nEND:VEVENT\nEND:VCALENDAR"

	parsed, err := ParseVCalendar(shared, personal)
	if err != nil {
		t.Fatalf("parse vcalendar: %v", err)
	}

	if parsed.Title != "Team Sync" || parsed.Location != "Room 42" || parsed.Recurrence == "" {
		t.Fatalf("unexpected parsed fields: %+v", parsed)
	}
	if len(parsed.Attendees) != 2 {
		t.Fatalf("unexpected attendees: %+v", parsed.Attendees)
	}
	if len(parsed.Reminders) != 1 || parsed.Reminders[0] != "-PT10M" {
		t.Fatalf("unexpected reminders: %+v", parsed.Reminders)
	}
}
