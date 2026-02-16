package protonapi

import (
	"context"
	"net/url"
)

func (c *Client) GetCalendars(ctx context.Context) ([]Calendar, error) {
	client, err := c.requireClient()
	if err != nil {
		return nil, err
	}
	items, err := client.GetCalendars(ctx)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return nil, err
	}
	c.setStatus(StatusConnected)
	return items, nil
}

func (c *Client) GetCalendar(ctx context.Context, id string) (Calendar, error) {
	client, err := c.requireClient()
	if err != nil {
		return Calendar{}, err
	}
	item, err := client.GetCalendar(ctx, id)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return Calendar{}, err
	}
	c.setStatus(StatusConnected)
	return item, nil
}

func (c *Client) GetCalendarKeys(ctx context.Context, id string) (CalendarKeys, error) {
	client, err := c.requireClient()
	if err != nil {
		return nil, err
	}
	items, err := client.GetCalendarKeys(ctx, id)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return nil, err
	}
	c.setStatus(StatusConnected)
	return items, nil
}

func (c *Client) GetCalendarMembers(ctx context.Context, id string) ([]CalendarMember, error) {
	client, err := c.requireClient()
	if err != nil {
		return nil, err
	}
	items, err := client.GetCalendarMembers(ctx, id)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return nil, err
	}
	c.setStatus(StatusConnected)
	return items, nil
}

func (c *Client) GetCalendarPassphrase(ctx context.Context, id string) (CalendarPassphrase, error) {
	client, err := c.requireClient()
	if err != nil {
		return CalendarPassphrase{}, err
	}
	item, err := client.GetCalendarPassphrase(ctx, id)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return CalendarPassphrase{}, err
	}
	c.setStatus(StatusConnected)
	return item, nil
}

func (c *Client) GetCalendarEvents(ctx context.Context, id string, page, pageSize int) ([]CalendarEvent, error) {
	client, err := c.requireClient()
	if err != nil {
		return nil, err
	}
	items, err := client.GetCalendarEvents(ctx, id, page, pageSize, url.Values{})
	if err != nil {
		c.setStatus(StatusDisconnected)
		return nil, err
	}
	c.setStatus(StatusConnected)
	return items, nil
}

func (c *Client) GetCalendarEvent(ctx context.Context, calID, eventID string) (CalendarEvent, error) {
	client, err := c.requireClient()
	if err != nil {
		return CalendarEvent{}, err
	}
	item, err := client.GetCalendarEvent(ctx, calID, eventID)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return CalendarEvent{}, err
	}
	c.setStatus(StatusConnected)
	return item, nil
}

func (c *Client) GetAddresses(ctx context.Context) ([]Address, error) {
	client, err := c.requireClient()
	if err != nil {
		return nil, err
	}
	items, err := client.GetAddresses(ctx)
	if err != nil {
		c.setStatus(StatusDisconnected)
		return nil, err
	}
	c.setStatus(StatusConnected)
	return items, nil
}
