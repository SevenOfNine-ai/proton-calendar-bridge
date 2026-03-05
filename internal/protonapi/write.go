package protonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// writeClient wraps an *http.Client and base URL to perform authenticated
// write requests against the Proton Calendar API.
//
// Proton uses these auth headers on every request:
//   - Authorization: Bearer {access_token}
//   - x-pm-uid:      {uid}
//   - x-pm-appversion: {app_version}
type writeClient struct {
	http    *http.Client
	baseURL string
	appVer  string
}

func newWriteClient(baseURL, appVer string) *writeClient {
	return &writeClient{
		http:    &http.Client{},
		baseURL: baseURL,
		appVer:  appVer,
	}
}

// do performs an authenticated JSON request. auth provides the session credentials.
func (wc *writeClient) do(ctx context.Context, method, path string, body interface{}, auth Auth) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	url := wc.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	req.Header.Set("x-pm-uid", auth.UID)
	req.Header.Set("x-pm-appversion", wc.appVer)

	resp, err := wc.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

// checkProtonCode inspects the Proton API envelope "Code" field.
// Proton returns HTTP 200 with Code=1000 on success; non-1000 codes are errors.
func checkProtonCode(data []byte) error {
	var envelope struct {
		Code  int    `json:"Code"`
		Error string `json:"Error"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil // not a Proton JSON envelope; let caller decide
	}
	if envelope.Code != 1000 && envelope.Code != 0 {
		if envelope.Error != "" {
			return fmt.Errorf("proton API error %d: %s", envelope.Code, envelope.Error)
		}
		return fmt.Errorf("proton API error code %d", envelope.Code)
	}
	return nil
}

// CreateCalendarEvent creates a new event via POST /calendar/v1/{calendarID}/events.
// Returns the created CalendarEvent as returned by the API.
func (c *Client) CreateCalendarEvent(ctx context.Context, calendarID string, req CreateCalendarEventReq) (CalendarEvent, error) {
	_, auth := c.session()
	if auth.UID == "" {
		return CalendarEvent{}, fmt.Errorf("proton session is not initialized")
	}

	wc := newWriteClient(c.baseURL(), c.appVer)
	path := "/calendar/v1/" + calendarID + "/events"
	data, status, err := wc.do(ctx, http.MethodPost, path, req, auth)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return CalendarEvent{}, err
	}
	if status != http.StatusOK {
		c.setStatus(StatusDisconnected)
		return CalendarEvent{}, fmt.Errorf("create event: HTTP %d: %s", status, truncate(data, 200))
	}
	if err := checkProtonCode(data); err != nil {
		return CalendarEvent{}, err
	}

	var res struct {
		Event CalendarEvent `json:"Event"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return CalendarEvent{}, fmt.Errorf("decode create event response: %w", err)
	}
	c.setStatus(StatusConnected)
	return res.Event, nil
}

// UpdateCalendarEvent updates an existing event via PUT /calendar/v1/{calendarID}/events/{eventID}.
func (c *Client) UpdateCalendarEvent(ctx context.Context, calendarID, eventID string, req CreateCalendarEventReq) (CalendarEvent, error) {
	_, auth := c.session()
	if auth.UID == "" {
		return CalendarEvent{}, fmt.Errorf("proton session is not initialized")
	}

	wc := newWriteClient(c.baseURL(), c.appVer)
	path := "/calendar/v1/" + calendarID + "/events/" + eventID
	data, status, err := wc.do(ctx, http.MethodPut, path, req, auth)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return CalendarEvent{}, err
	}
	if status != http.StatusOK {
		c.setStatus(StatusDisconnected)
		return CalendarEvent{}, fmt.Errorf("update event: HTTP %d: %s", status, truncate(data, 200))
	}
	if err := checkProtonCode(data); err != nil {
		return CalendarEvent{}, err
	}

	var res struct {
		Event CalendarEvent `json:"Event"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return CalendarEvent{}, fmt.Errorf("decode update event response: %w", err)
	}
	c.setStatus(StatusConnected)
	return res.Event, nil
}

// DeleteCalendarEvent deletes an event via DELETE /calendar/v1/{calendarID}/events/{eventID}.
func (c *Client) DeleteCalendarEvent(ctx context.Context, calendarID, eventID string) error {
	_, auth := c.session()
	if auth.UID == "" {
		return fmt.Errorf("proton session is not initialized")
	}

	wc := newWriteClient(c.baseURL(), c.appVer)
	path := "/calendar/v1/" + calendarID + "/events/" + eventID
	data, status, err := wc.do(ctx, http.MethodDelete, path, nil, auth)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return err
	}
	if status != http.StatusOK {
		c.setStatus(StatusDisconnected)
		return fmt.Errorf("delete event: HTTP %d: %s", status, truncate(data, 200))
	}
	if err := checkProtonCode(data); err != nil {
		return err
	}
	c.setStatus(StatusConnected)
	return nil
}

// baseURL returns the base URL used for write requests.
// Falls back to DefaultBaseURL if the manager URL is not available.
func (c *Client) baseURL() string {
	// The proton.Manager stores the host URL internally. We mirror DefaultBaseURL
	// since go-proton-api does not expose a getter for the configured base URL.
	return DefaultBaseURL
}

// truncate returns at most n bytes of data as a string, for error messages.
func truncate(data []byte, n int) string {
	if len(data) <= n {
		return string(data)
	}
	return string(data[:n]) + "..."
}
