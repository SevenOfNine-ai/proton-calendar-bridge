package provider

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type ICSProvider struct {
	url    string
	client HTTPDoer
	now    func() time.Time
}

func NewICSProvider(url string, client HTTPDoer) *ICSProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &ICSProvider{url: url, client: client, now: time.Now}
}

func (p *ICSProvider) Name() string { return "ics" }

func (p *ICSProvider) ListCalendars(context.Context) ([]domain.Calendar, error) {
	return []domain.Calendar{{ID: "ics-default", Name: "Proton ICS (read-only)", ReadOnly: true}}, nil
}

func (p *ICSProvider) ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]domain.Event, error) {
	if calendarID == "" {
		calendarID = "ics-default"
	}
	body, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	events, err := parseICS(body, calendarID)
	if err != nil {
		return nil, err
	}
	if from.IsZero() && to.IsZero() {
		return events, nil
	}
	filtered := make([]domain.Event, 0, len(events))
	for _, e := range events {
		if !from.IsZero() && e.End.Before(from) {
			continue
		}
		if !to.IsZero() && e.Start.After(to) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, nil
}

func (p *ICSProvider) CreateEvent(context.Context, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "create_event"}
}

func (p *ICSProvider) UpdateEvent(context.Context, string, domain.EventMutation) (domain.Event, error) {
	return domain.Event{}, NotSupportedError{Operation: "update_event"}
}

func (p *ICSProvider) DeleteEvent(context.Context, string) error {
	return NotSupportedError{Operation: "delete_event"}
}

func (p *ICSProvider) fetch(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch ics: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("fetch ics: unexpected status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

func parseICS(r io.Reader, calendarID string) ([]domain.Event, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 512*1024)

	type rawEvent struct {
		id, title, desc, location, dtstart, dtend string
	}

	var raws []rawEvent
	var current *rawEvent
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "BEGIN:VEVENT":
			current = &rawEvent{}
		case line == "END:VEVENT":
			if current != nil {
				raws = append(raws, *current)
			}
			current = nil
		case current != nil:
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			key := strings.ToUpper(strings.Split(k, ";")[0])
			switch key {
			case "UID":
				current.id = v
			case "SUMMARY":
				current.title = v
			case "DESCRIPTION":
				current.desc = v
			case "LOCATION":
				current.location = v
			case "DTSTART":
				current.dtstart = v
			case "DTEND":
				current.dtend = v
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan ics: %w", err)
	}

	events := make([]domain.Event, 0, len(raws))
	for _, raw := range raws {
		if raw.id == "" || raw.dtstart == "" {
			continue
		}
		start, allDay, err := parseICSTime(raw.dtstart)
		if err != nil {
			continue
		}
		end := start
		if raw.dtend != "" {
			if parsed, _, err := parseICSTime(raw.dtend); err == nil {
				end = parsed
			}
		}
		events = append(events, domain.Event{
			ID:          raw.id,
			CalendarID:  calendarID,
			Title:       raw.title,
			Description: raw.desc,
			Location:    raw.location,
			Start:       start,
			End:         end,
			AllDay:      allDay,
		})
	}
	return events, nil
}

func parseICSTime(v string) (time.Time, bool, error) {
	v = strings.TrimSpace(v)
	if len(v) == len("20060102") {
		t, err := time.Parse("20060102", v)
		return t, true, err
	}
	formats := []string{"20060102T150405Z", "20060102T150405"}
	for _, f := range formats {
		if t, err := time.Parse(f, v); err == nil {
			return t, false, nil
		}
	}
	return time.Time{}, false, errors.New("invalid ics datetime")
}
