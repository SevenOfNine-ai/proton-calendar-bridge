// Package caldav provides a minimal CalDAV (RFC 4791) HTTP handler that translates
// CalDAV requests into calls on a CalendarProvider. This allows GNOME Calendar and
// other RFC-compliant clients to read and write Proton Calendar events.
//
// Supported CalDAV URL layout (relative to the mount prefix, default "/caldav"):
//
//	OPTIONS  /*                       Announce CalDAV capabilities
//	PROPFIND /                        Principal + calendar-home-set discovery
//	PROPFIND /calendars/              List all calendar collections
//	PROPFIND /calendars/{calID}/      Calendar properties (ctag, display-name, …)
//	REPORT   /calendars/{calID}/      calendar-query / calendar-multiget
//	GET      /calendars/{calID}/{uid}.ics  Fetch individual event as iCalendar
//	PUT      /calendars/{calID}/{uid}.ics  Create or update an event
//	DELETE   /calendars/{calID}/{uid}.ics  Delete an event
package caldav

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/sevenofnine/proton-calendar-bridge/internal/domain"
	"github.com/sevenofnine/proton-calendar-bridge/internal/provider"
)

// CalendarProvider is the subset of provider.CalendarProvider used by the CalDAV
// handler.  Using a local interface allows easy mocking in tests.
type CalendarProvider interface {
	ListCalendars(ctx context.Context) ([]domain.Calendar, error)
	ListEvents(ctx context.Context, calendarID string, from, to time.Time) ([]domain.Event, error)
	CreateEvent(ctx context.Context, in domain.EventMutation) (domain.Event, error)
	UpdateEvent(ctx context.Context, eventID string, in domain.EventMutation) (domain.Event, error)
	DeleteEvent(ctx context.Context, eventID string) error
}

// Handler is a CalDAV HTTP handler. Mount it under a prefix (e.g. "/caldav/")
// using http.StripPrefix.
type Handler struct {
	provider CalendarProvider
	log      *slog.Logger
}

// New creates a CalDAV handler backed by the given CalendarProvider.
func New(p CalendarProvider, log *slog.Logger) *Handler {
	if log == nil {
		log = slog.Default()
	}
	return &Handler{provider: p, log: log}
}

// ServeHTTP dispatches CalDAV requests.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Normalise path: strip trailing slash except for root.
	path := r.URL.Path
	if path != "/" && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}

	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	// parts[0] = "" (root) or "calendars"
	// parts[1] = calendarID (if present)
	// parts[2] = uid.ics (if present)

	switch r.Method {
	case "OPTIONS":
		h.handleOptions(w, r)
	case "PROPFIND":
		h.handlePropfind(w, r, parts)
	case "REPORT":
		h.handleReport(w, r, parts)
	case http.MethodGet:
		h.handleGet(w, r, parts)
	case http.MethodPut:
		h.handlePut(w, r, parts)
	case http.MethodDelete:
		h.handleDelete(w, r, parts)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ---------------------------------------------------------------------------
// OPTIONS
// ---------------------------------------------------------------------------

func (h *Handler) handleOptions(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("DAV", "1, 3, calendar-access")
	w.Header().Set("Allow", "OPTIONS, GET, PUT, DELETE, PROPFIND, REPORT")
	w.WriteHeader(http.StatusOK)
}

// ---------------------------------------------------------------------------
// PROPFIND
// ---------------------------------------------------------------------------

func (h *Handler) handlePropfind(w http.ResponseWriter, r *http.Request, parts []string) {
	depth := r.Header.Get("Depth")
	if depth == "" {
		depth = "1"
	}

	switch {
	case len(parts) <= 1 || (len(parts) == 1 && parts[0] == ""):
		// Root: return principal + calendar-home-set.
		h.propfindPrincipal(w, r)
	case parts[0] == "calendars" && len(parts) == 1:
		// Collection of all calendars.
		h.propfindCalendars(w, r, depth)
	case parts[0] == "calendars" && len(parts) == 2:
		// Single calendar.
		h.propfindCalendar(w, r, parts[1])
	default:
		writeXMLError(w, http.StatusNotFound, "not found")
	}
}

func (h *Handler) propfindPrincipal(w http.ResponseWriter, r *http.Request) {
	base := requestBase(r)
	responses := []xmlResponse{
		{
			Href: base + "/",
			Props: []xmlProp{
				{Name: "displayname", Value: "Proton Calendar Bridge"},
				{Name: "calendar-home-set", Namespace: nsCaldav, HrefValue: base + "/calendars/"},
				{Name: "principal-URL", HrefValue: base + "/"},
				{Name: "resourcetype", ResourceType: "principal"},
			},
		},
	}
	writeMultiStatus(w, responses)
}

func (h *Handler) propfindCalendars(w http.ResponseWriter, r *http.Request, depth string) {
	cals, err := h.provider.ListCalendars(r.Context())
	if err != nil {
		h.log.Error("list calendars", "error", err)
		writeXMLError(w, http.StatusBadGateway, err.Error())
		return
	}

	base := requestBase(r)
	var responses []xmlResponse

	// Include the collection itself.
	responses = append(responses, xmlResponse{
		Href: base + "/calendars/",
		Props: []xmlProp{
			{Name: "displayname", Value: "Calendars"},
			{Name: "resourcetype", ResourceType: "collection"},
		},
	})

	if depth != "0" {
		for _, c := range cals {
			responses = append(responses, calendarToResponse(base, c))
		}
	}
	writeMultiStatus(w, responses)
}

func (h *Handler) propfindCalendar(w http.ResponseWriter, r *http.Request, calID string) {
	cals, err := h.provider.ListCalendars(r.Context())
	if err != nil {
		writeXMLError(w, http.StatusBadGateway, err.Error())
		return
	}
	for _, c := range cals {
		if c.ID == calID {
			base := requestBase(r)
			writeMultiStatus(w, []xmlResponse{calendarToResponse(base, c)})
			return
		}
	}
	writeXMLError(w, http.StatusNotFound, "calendar not found")
}

// ---------------------------------------------------------------------------
// REPORT (calendar-query / calendar-multiget)
// ---------------------------------------------------------------------------

func (h *Handler) handleReport(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) < 2 || parts[0] != "calendars" {
		writeXMLError(w, http.StatusNotFound, "not found")
		return
	}
	calID := parts[1]

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeXMLError(w, http.StatusBadRequest, "cannot read body")
		return
	}

	// Parse just the root element name to dispatch.
	var root struct {
		XMLName xml.Name
	}
	_ = xml.Unmarshal(body, &root)

	switch root.XMLName.Local {
	case "calendar-multiget":
		h.reportMultiget(w, r, calID, body)
	default:
		// calendar-query: return all events in range.
		h.reportQuery(w, r, calID)
	}
}

func (h *Handler) reportQuery(w http.ResponseWriter, r *http.Request, calID string) {
	events, err := h.provider.ListEvents(r.Context(), calID, time.Time{}, time.Time{})
	if err != nil {
		writeXMLError(w, http.StatusBadGateway, err.Error())
		return
	}
	base := requestBase(r)
	var responses []xmlResponse
	for _, e := range events {
		responses = append(responses, eventToResponse(base, calID, e))
	}
	writeMultiStatus(w, responses)
}

func (h *Handler) reportMultiget(w http.ResponseWriter, r *http.Request, calID string, body []byte) {
	var req struct {
		XMLName xml.Name `xml:"calendar-multiget"`
		Hrefs   []string `xml:"href"`
	}
	if err := xml.Unmarshal(body, &req); err != nil {
		writeXMLError(w, http.StatusBadRequest, "invalid xml")
		return
	}

	events, err := h.provider.ListEvents(r.Context(), calID, time.Time{}, time.Time{})
	if err != nil {
		writeXMLError(w, http.StatusBadGateway, err.Error())
		return
	}

	// Index events by UID slug.
	byUID := make(map[string]domain.Event, len(events))
	for _, e := range events {
		byUID[uidSlug(e.ID)] = e
	}

	base := requestBase(r)
	var responses []xmlResponse
	for _, href := range req.Hrefs {
		slug := icsSlugFromHref(href)
		if e, ok := byUID[slug]; ok {
			responses = append(responses, eventToResponse(base, calID, e))
		} else {
			responses = append(responses, xmlResponse{
				Href:   href,
				Status: "HTTP/1.1 404 Not Found",
			})
		}
	}
	writeMultiStatus(w, responses)
}

// ---------------------------------------------------------------------------
// GET  /calendars/{calID}/{uid}.ics
// ---------------------------------------------------------------------------

func (h *Handler) handleGet(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 3 || parts[0] != "calendars" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	calID := parts[1]
	slug := icsSlugFromPart(parts[2])

	events, err := h.provider.ListEvents(r.Context(), calID, time.Time{}, time.Time{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for _, e := range events {
		if uidSlug(e.ID) == slug {
			w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
			w.Header().Set("ETag", etag(e))
			fmt.Fprint(w, eventToICS(e))
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// PUT  /caldav/calendars/{calID}/{uid}.ics
// ---------------------------------------------------------------------------

func (h *Handler) handlePut(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 3 || parts[0] != "calendars" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	calID := parts[1]
	slug := icsSlugFromPart(parts[2])

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	mutation, err := parseICSToMutation(string(body), calID)
	if err != nil {
		http.Error(w, "invalid iCal: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Determine whether this is a create or update by checking existing events.
	events, err := h.provider.ListEvents(r.Context(), calID, time.Time{}, time.Time{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	for _, e := range events {
		if uidSlug(e.ID) == slug {
			// Update existing event.
			updated, err := h.provider.UpdateEvent(r.Context(), e.ID, mutation)
			if err != nil {
				h.log.Error("update event", "error", err)
				if isNotSupported(err) {
					http.Error(w, "write not supported by provider", http.StatusForbidden)
					return
				}
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.Header().Set("ETag", etag(updated))
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// Create new event.
	created, err := h.provider.CreateEvent(r.Context(), mutation)
	if err != nil {
		h.log.Error("create event", "error", err)
		if isNotSupported(err) {
			http.Error(w, "write not supported by provider", http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	base := requestBase(r)
	w.Header().Set("Location", base+"/calendars/"+calID+"/"+uidSlug(created.ID)+".ics")
	w.Header().Set("ETag", etag(created))
	w.WriteHeader(http.StatusCreated)
}

// ---------------------------------------------------------------------------
// DELETE  /caldav/calendars/{calID}/{uid}.ics
// ---------------------------------------------------------------------------

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request, parts []string) {
	if len(parts) != 3 || parts[0] != "calendars" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	calID := parts[1]
	slug := icsSlugFromPart(parts[2])

	events, err := h.provider.ListEvents(r.Context(), calID, time.Time{}, time.Time{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	for _, e := range events {
		if uidSlug(e.ID) == slug {
			if err := h.provider.DeleteEvent(r.Context(), e.ID); err != nil {
				if isNotSupported(err) {
					http.Error(w, "delete not supported by provider", http.StatusForbidden)
					return
				}
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}

// ---------------------------------------------------------------------------
// ICS helpers
// ---------------------------------------------------------------------------

// parseICSToMutation parses a VCALENDAR iCal string and converts it to an
// EventMutation suitable for CreateEvent / UpdateEvent.
func parseICSToMutation(ics, calendarID string) (domain.EventMutation, error) {
	lines := unfoldICSLines(ics)
	get := func(key string) string {
		for _, l := range lines {
			k, v, ok := strings.Cut(l, ":")
			if !ok {
				continue
			}
			if strings.EqualFold(strings.SplitN(k, ";", 2)[0], key) {
				return v
			}
		}
		return ""
	}

	startRaw := get("DTSTART")
	endRaw := get("DTEND")

	start, allDay, err := parseICSDateTime(startRaw)
	if err != nil {
		return domain.EventMutation{}, fmt.Errorf("DTSTART: %w", err)
	}
	end := start
	if endRaw != "" {
		if t, _, e := parseICSDateTime(endRaw); e == nil {
			end = t
		}
	}

	var attendees []string
	for _, l := range lines {
		k, v, ok := strings.Cut(l, ":")
		if ok && strings.EqualFold(strings.SplitN(k, ";", 2)[0], "ATTENDEE") {
			attendees = append(attendees, v)
		}
	}

	return domain.EventMutation{
		CalendarID:  calendarID,
		Title:       unescapeICSText(get("SUMMARY")),
		Description: unescapeICSText(get("DESCRIPTION")),
		Location:    unescapeICSText(get("LOCATION")),
		Start:       start,
		End:         end,
		AllDay:      allDay,
		Recurrence:  get("RRULE"),
		Attendees:   attendees,
	}, nil
}

func unfoldICSLines(data string) []string {
	raw := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		if len(out) > 0 && (strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")) {
			out[len(out)-1] += strings.TrimLeft(line, " \t")
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func parseICSDateTime(v string) (time.Time, bool, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return time.Time{}, false, fmt.Errorf("empty datetime")
	}
	// All-day: YYYYMMDD (8 chars)
	if len(v) == 8 {
		t, err := time.Parse("20060102", v)
		return t, true, err
	}
	for _, f := range []string{"20060102T150405Z", "20060102T150405", "20060102T1504Z", "20060102T1504"} {
		if t, err := time.Parse(f, v); err == nil {
			return t.UTC(), false, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("unrecognised datetime: %s", v)
}

func unescapeICSText(s string) string {
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\,`, ",")
	s = strings.ReplaceAll(s, `\;`, ";")
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

// eventToICS serialises a domain.Event to a VCALENDAR iCal string.
func eventToICS(e domain.Event) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//proton-calendar-bridge//EN\r\n")
	b.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&b, "UID:%s\r\n", e.ID)
	fmt.Fprintf(&b, "DTSTART%s\r\n", formatCalDAVDateTime(e.Start, e.AllDay))
	fmt.Fprintf(&b, "DTEND%s\r\n", formatCalDAVDateTime(e.End, e.AllDay))
	if e.Title != "" {
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", escapeICSText(e.Title))
	}
	if e.Description != "" {
		fmt.Fprintf(&b, "DESCRIPTION:%s\r\n", escapeICSText(e.Description))
	}
	if e.Location != "" {
		fmt.Fprintf(&b, "LOCATION:%s\r\n", escapeICSText(e.Location))
	}
	if e.Recurrence != "" {
		fmt.Fprintf(&b, "RRULE:%s\r\n", e.Recurrence)
	}
	for _, a := range e.Attendees {
		fmt.Fprintf(&b, "ATTENDEE:%s\r\n", a)
	}
	for _, trigger := range e.Reminders {
		b.WriteString("BEGIN:VALARM\r\n")
		b.WriteString("ACTION:DISPLAY\r\n")
		fmt.Fprintf(&b, "TRIGGER:%s\r\n", trigger)
		b.WriteString("END:VALARM\r\n")
	}
	if e.UpdatedAt != nil {
		fmt.Fprintf(&b, "LAST-MODIFIED:%s\r\n", e.UpdatedAt.UTC().Format("20060102T150405Z"))
	}
	b.WriteString("END:VEVENT\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

func formatCalDAVDateTime(t time.Time, allDay bool) string {
	if allDay {
		return fmt.Sprintf(";VALUE=DATE:%s", t.UTC().Format("20060102"))
	}
	return fmt.Sprintf(":%s", t.UTC().Format("20060102T150405Z"))
}

func escapeICSText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// etag returns a simple ETag string for an event based on its updated time.
func etag(e domain.Event) string {
	if e.UpdatedAt != nil {
		return fmt.Sprintf(`"%d"`, e.UpdatedAt.Unix())
	}
	return fmt.Sprintf(`"%s"`, e.ID)
}

// uidSlug converts an event ID to a filesystem-safe slug for use in .ics URLs.
func uidSlug(id string) string {
	// Replace characters that are not URL-safe.
	return strings.NewReplacer("/", "_", ":", "_", "@", "_", " ", "_").Replace(id)
}

// icsSlugFromHref extracts the base filename (without .ics) from a href like
// "/caldav/calendars/calID/abc.ics".
func icsSlugFromHref(href string) string {
	parts := strings.Split(href, "/")
	if len(parts) == 0 {
		return href
	}
	return icsSlugFromPart(parts[len(parts)-1])
}

func icsSlugFromPart(p string) string {
	return strings.TrimSuffix(p, ".ics")
}

// requestBase returns the scheme+host portion of the request URL.
func requestBase(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := r.Header.Get("X-Forwarded-Proto"); fwd != "" {
		scheme = fwd
	}
	return scheme + "://" + r.Host
}

// isNotSupported returns true if the error wraps provider.ErrNotSupported.
func isNotSupported(err error) bool {
	return strings.Contains(err.Error(), provider.ErrNotSupported.Error())
}

// ---------------------------------------------------------------------------
// XML helpers
// ---------------------------------------------------------------------------

const (
	nsDAV    = "DAV:"
	nsCaldav = "urn:ietf:params:xml:ns:caldav"
)

type xmlProp struct {
	Name         string
	Namespace    string
	Value        string
	HrefValue    string
	ResourceType string // "principal", "collection", "calendar", ""
}

type xmlResponse struct {
	Href   string
	Status string // if empty, defaults to 200 OK
	Props  []xmlProp
}

func writeMultiStatus(w http.ResponseWriter, responses []xmlResponse) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(207) // Multi-Status

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">`)

	for _, resp := range responses {
		b.WriteString(`<D:response>`)
		fmt.Fprintf(&b, `<D:href>%s</D:href>`, xmlEscape(resp.Href))

		if resp.Status != "" {
			fmt.Fprintf(&b, `<D:status>%s</D:status>`, xmlEscape(resp.Status))
		} else {
			b.WriteString(`<D:propstat>`)
			b.WriteString(`<D:prop>`)
			for _, p := range resp.Props {
				writePropXML(&b, p)
			}
			b.WriteString(`</D:prop>`)
			b.WriteString(`<D:status>HTTP/1.1 200 OK</D:status>`)
			b.WriteString(`</D:propstat>`)
		}
		b.WriteString(`</D:response>`)
	}
	b.WriteString(`</D:multistatus>`)
	fmt.Fprint(w, b.String())
}

func writePropXML(b *strings.Builder, p xmlProp) {
	prefix := "D"
	if p.Namespace == nsCaldav {
		prefix = "C"
	}
	tag := prefix + ":" + p.Name

	b.WriteString("<" + tag + ">")
	switch {
	case p.HrefValue != "":
		fmt.Fprintf(b, "<D:href>%s</D:href>", xmlEscape(p.HrefValue))
	case p.ResourceType != "":
		switch p.ResourceType {
		case "principal":
			b.WriteString("<D:principal/>")
		case "collection":
			b.WriteString("<D:collection/>")
		case "calendar":
			b.WriteString("<D:collection/><C:calendar/>")
		}
	default:
		b.WriteString(xmlEscape(p.Value))
	}
	b.WriteString("</" + tag + ">")
}

func writeXMLError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.WriteHeader(code)
	fmt.Fprintf(w, `<?xml version="1.0"?><error><description>%s</description></error>`, xmlEscape(msg))
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---------------------------------------------------------------------------
// CalDAV response builders
// ---------------------------------------------------------------------------

func calendarToResponse(base string, c domain.Calendar) xmlResponse {
	href := base + "/calendars/" + c.ID + "/"
	props := []xmlProp{
		{Name: "displayname", Value: c.Name},
		{Name: "resourcetype", ResourceType: "calendar"},
		{Name: "supported-calendar-component-set", Namespace: nsCaldav, Value: "<C:comp name=\"VEVENT\"/>"},
		{Name: "getctag", Namespace: nsCaldav, Value: c.ID},
	}
	if c.ReadOnly {
		props = append(props, xmlProp{Name: "current-user-privilege-set", Value: "<D:privilege><D:read/></D:privilege>"})
	} else {
		props = append(props, xmlProp{Name: "current-user-privilege-set", Value: "<D:privilege><D:read/></D:privilege><D:privilege><D:write/></D:privilege>"})
	}
	return xmlResponse{Href: href, Props: props}
}

func eventToResponse(base, calID string, e domain.Event) xmlResponse {
	slug := uidSlug(e.ID)
	href := base + "/calendars/" + calID + "/" + slug + ".ics"
	return xmlResponse{
		Href: href,
		Props: []xmlProp{
			{Name: "getetag", Value: etag(e)},
			{Name: "calendar-data", Namespace: nsCaldav, Value: eventToICS(e)},
		},
	}
}
