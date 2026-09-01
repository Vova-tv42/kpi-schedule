// Package mykpi fetches and parses the authenticated student calendar from
// my.kpi.ua. The parser (parser.go) is fixture-driven: its selectors are
// verified against a real HTML dump rather than guessed from CSS files.
// See docs/schedules/main/data-extraction.md.
//
// The calendar page (/room/student/calendar) does NOT render a static HTML
// schedule table — it embeds a FullCalendar.js widget that fetches lesson
// data from a separate JSON endpoint, /calendar/studevents?id=<studentId>.
// The studentId is only known after parsing it out of the calendar page's
// inline script, so fetching a student's schedule is a two-step process:
// FetchCalendarPage (to discover the events URL) then FetchEventsJSONRange.
package mykpi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/model"
)

const baseURL = "https://my.kpi.ua"
const calendarURL = baseURL + "/room/student/calendar"

// ErrSessionExpired signals that the given cookies were rejected by my.kpi.ua.
var ErrSessionExpired = errors.New("my.kpi.ua session expired or invalid")

// ErrEventsURLNotFound signals that the calendar page's inline FullCalendar
// config didn't contain a recognizable "events" URL — the page layout likely
// changed and the parser needs re-verifying against a fresh fixture.
var ErrEventsURLNotFound = errors.New("could not find FullCalendar events URL in calendar page")

var eventsURLPattern = regexp.MustCompile(`"events"\s*:\s*"([^"]+)"`)

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

// FetchCalendarPage performs the authenticated GET of the calendar shell page
// and returns the raw HTML body. It returns ErrSessionExpired if the cookies
// were rejected.
func (c *Client) FetchCalendarPage(ctx context.Context, cookies model.Cookies, userAgent string) ([]byte, error) {
	return c.authenticatedGet(ctx, calendarURL, "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8", cookies, userAgent, true)
}

// ExtractEventsURL pulls the FullCalendar "events" URL out of the calendar
// page's inline script, e.g. "/calendar/studevents?id=33101".
func ExtractEventsURL(calendarHTML []byte) (string, error) {
	m := eventsURLPattern.FindSubmatch(calendarHTML)
	if m == nil {
		return "", ErrEventsURLNotFound
	}
	return string(m[1]), nil
}

// FetchEventsJSONRange fetches the raw JSON lesson-events payload from the
// URL discovered via ExtractEventsURL, for the [start, end] date range.
// eventsPath is relative (e.g. "/calendar/studevents?id=33101"). The range
// params are required — this is standard FullCalendar string-source
// behavior, and the endpoint silently returns "[]" without them (confirmed
// against the live endpoint).
func (c *Client) FetchEventsJSONRange(ctx context.Context, eventsPath string, cookies model.Cookies, userAgent string, start, end time.Time) ([]byte, error) {
	url := eventsPath
	if strings.HasPrefix(eventsPath, "/") {
		url = baseURL + eventsPath
	}
	sep := "?"
	if strings.Contains(url, "?") {
		sep = "&"
	}
	url = fmt.Sprintf("%s%sstart=%s&end=%s", url, sep, start.Format("2006-01-02"), end.Format("2006-01-02"))
	return c.authenticatedGet(ctx, url, "application/json", cookies, userAgent, false)
}

func (c *Client) authenticatedGet(ctx context.Context, url, accept string, cookies model.Cookies, userAgent string, checkLoginPage bool) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	req.Header.Set("Cookie", buildCookieHeader(cookies))
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound {
		loc := resp.Header.Get("Location")
		if strings.Contains(loc, "/user/login") {
			return nil, ErrSessionExpired
		}
		return nil, fmt.Errorf("unexpected redirect to %q", loc)
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrSessionExpired
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if checkLoginPage && looksLikeLoginPage(body) {
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

// FetchStudentEventsRange is the full two-step fetch: load the calendar
// page, discover the events URL, then fetch the JSON payload for the given
// date range.
func (c *Client) FetchStudentEventsRange(ctx context.Context, cookies model.Cookies, userAgent string, start, end time.Time) ([]byte, error) {
	page, err := c.FetchCalendarPage(ctx, cookies, userAgent)
	if err != nil {
		return nil, err
	}
	eventsURL, err := ExtractEventsURL(page)
	if err != nil {
		return nil, err
	}
	return c.FetchEventsJSONRange(ctx, eventsURL, cookies, userAgent, start, end)
}
