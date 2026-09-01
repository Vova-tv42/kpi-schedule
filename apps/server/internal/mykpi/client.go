// Package mykpi fetches and parses the authenticated student calendar from
// my.kpi.ua. The parser (parser.go) is fixture-driven: its selectors are
// verified against a real HTML dump rather than guessed from CSS files.
// See docs/schedules/main/data-extraction.md.
package mykpi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/model"
)

const calendarURL = "https://my.kpi.ua/room/student/calendar"

// ErrSessionExpired signals that the given cookies were rejected by my.kpi.ua.
var ErrSessionExpired = errors.New("my.kpi.ua session expired or invalid")

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 15 * time.Second,
			// Observe a redirect to /user/login rather than following it,
			// so an expired session is detected instead of silently
			// returning the login page's HTML.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// FetchCalendarHTML performs the authenticated GET and returns the raw HTML
// body. It returns ErrSessionExpired if the cookies were rejected.
func (c *Client) FetchCalendarHTML(ctx context.Context, cookies model.Cookies, userAgent string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, calendarURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Cookie", buildCookieHeader(cookies))
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting calendar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound {
		loc := resp.Header.Get("Location")
		if strings.Contains(loc, "/user/login") {
			return nil, ErrSessionExpired
		}
		return nil, fmt.Errorf("unexpected redirect to %q", loc)
	}
	if resp.StatusCode == http.StatusForbidden {
		return nil, ErrSessionExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from my.kpi.ua", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if looksLikeLoginPage(body) {
		return nil, ErrSessionExpired
	}

	return body, nil
}

func buildCookieHeader(c model.Cookies) string {
	var parts []string
	if c.PHPSESSID != "" {
		parts = append(parts, "PHPSESSID="+c.PHPSESSID)
	}
	if c.Identity != "" {
		parts = append(parts, "_identity="+c.Identity)
	}
	if c.Language != "" {
		parts = append(parts, "language="+c.Language)
	}
	return strings.Join(parts, "; ")
}

// looksLikeLoginPage is a fallback for the case where my.kpi.ua returns 200
// but silently renders the login form instead of redirecting (observed
// behavior varies across Yii2 apps depending on session middleware config).
func looksLikeLoginPage(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "login-form") || strings.Contains(s, "id=\"login-form\"")
}
