package crypto

import (
	"fmt"
	"strings"
	"time"
)

type ParsedEvent struct {
	Title       string
	Description string
	Location    string
	Start       time.Time
	End         time.Time
	AllDay      bool
	Recurrence  string
	Attendees   []string
	Reminders   []string
}

func ParseVCalendar(sharedData, personalData string) (ParsedEvent, error) {
	shared := parseCalendarLines(sharedData)

	start, allDay, err := parseICalDateTime(shared["DTSTART"])
	if err != nil {
		return ParsedEvent{}, fmt.Errorf("parse DTSTART: %w", err)
	}

	end := start
	if raw := shared["DTEND"]; raw != "" {
		if parsedEnd, _, err := parseICalDateTime(raw); err == nil {
			end = parsedEnd
		}
	}

	return ParsedEvent{
		Title:       shared["SUMMARY"],
		Description: shared["DESCRIPTION"],
		Location:    shared["LOCATION"],
		Start:       start,
		End:         end,
		AllDay:      allDay,
		Recurrence:  shared["RRULE"],
		Attendees:   collectCalendarValues(sharedData, "ATTENDEE"),
		Reminders:   collectCalendarValues(personalData, "TRIGGER"),
	}, nil
}

func parseCalendarLines(data string) map[string]string {
	out := map[string]string{}
	for _, line := range unfoldLines(data) {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key := strings.ToUpper(strings.Split(k, ";")[0])
		if out[key] == "" {
			out[key] = v
		}
	}
	return out
}

func collectCalendarValues(data, key string) []string {
	key = strings.ToUpper(strings.TrimSpace(key))
	if key == "" {
		return nil
	}
	var out []string
	for _, line := range unfoldLines(data) {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.ToUpper(strings.Split(k, ";")[0]) == key {
			out = append(out, v)
		}
	}
	return out
}

func unfoldLines(data string) []string {
	if data == "" {
		return nil
	}
	raw := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if len(out) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			out[len(out)-1] += strings.TrimLeft(line, " \t")
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func parseICalDateTime(v string) (time.Time, bool, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false, fmt.Errorf("empty datetime")
	}
	if len(v) == len("20060102") {
		t, err := time.Parse("20060102", v)
		return t, true, err
	}
	for _, f := range []string{"20060102T150405Z", "20060102T150405"} {
		if t, err := time.Parse(f, v); err == nil {
			return t, false, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("invalid ical datetime: %s", v)
}
