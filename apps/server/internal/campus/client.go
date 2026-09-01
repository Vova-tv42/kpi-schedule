package campus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"kpi-schedule-bot/server/internal/storage"
)

const baseURL = "https://api.campus.kpi.ua"

// Cache TTLs (maxAge for storage.DB.CacheGet), unchanged from the previous
// in-memory cache — only the backing store moved to SQLite, so a value
// fetched before the host VM sleeps is still fresh on wake instead of
// forcing a cold-start burst of re-fetches. See docs/architecture/data-storage.md §5.
const (
	timeCacheTTL     = 1 * time.Minute
	slotsCacheTTL    = 24 * time.Hour
	groupsCacheTTL   = 24 * time.Hour
	scheduleCacheTTL = 6 * time.Hour
)

type Client struct {
	http *http.Client
	db   *storage.DB
}

func NewClient(db *storage.DB) *Client {
	return &Client{
		http: &http.Client{Timeout: 10 * time.Second},
		db:   db,
	}
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0")
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("requesting %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, path)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decoding response from %s: %w", path, err)
	}
	return nil
}

// cachedJSON serves out from the SQLite-backed cache (key, maxAge) when
// fresh, otherwise fetches path, decodes into out, and persists it under key
// for next time. A cache read/write failure is logged-worthy but not fatal
// to the caller — falling through to a live fetch keeps the server working
// even if the cache table is briefly unavailable.
func (c *Client) cachedJSON(ctx context.Context, key string, maxAge time.Duration, path string, out any) error {
	if hit, err := c.db.CacheGet(ctx, key, maxAge, out); err == nil && hit {
		return nil
	}
	if err := c.getJSON(ctx, path, out); err != nil {
		return err
	}
	return c.db.CacheSet(ctx, key, out)
}

// CurrentTime returns the current academic week/day/slot, cached for 1 minute.
func (c *Client) CurrentTime(ctx context.Context) (CurrentAcademicTime, error) {
	var out CurrentAcademicTime
	if err := c.cachedJSON(ctx, "time:current", timeCacheTTL, "/time/current", &out); err != nil {
		return CurrentAcademicTime{}, err
	}
	return out, nil
}

// LessonSlots returns the slot-number -> start-time map, cached for 24 hours.
func (c *Client) LessonSlots(ctx context.Context) (map[string]string, error) {
	var out map[string]string
	if err := c.cachedJSON(ctx, "slots:all", slotsCacheTTL, "/schedule/lessons/slots", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Groups returns the full group catalog, cached for 24 hours.
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	var out []Group
	if err := c.cachedJSON(ctx, "groups:all", groupsCacheTTL, "/group/all", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SearchGroups filters the cached catalog by a case-insensitive substring match on name or faculty.
func (c *Client) SearchGroups(ctx context.Context, query string) ([]Group, error) {
	groups, err := c.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if query == "" {
		return groups, nil
	}
	q := strings.ToLower(query)
	var matches []Group
	for _, g := range groups {
		if strings.Contains(strings.ToLower(g.Name), q) || strings.Contains(strings.ToLower(g.Faculty), q) {
			matches = append(matches, g)
		}
	}
	return matches, nil
}

// ResolveGroupID finds a group's numeric ID by exact (case-insensitive) name match.
func (c *Client) ResolveGroupID(ctx context.Context, groupName string) (int, error) {
	groups, err := c.Groups(ctx)
	if err != nil {
		return 0, err
	}
	target := strings.ToLower(strings.TrimSpace(groupName))
	for _, g := range groups {
		if strings.ToLower(g.Name) == target {
			return g.ID, nil
		}
	}
	return 0, fmt.Errorf("group %q not found in catalog", groupName)
}

// GroupSchedule returns the full 2-week schedule for a group, cached for 6 hours.
func (c *Client) GroupSchedule(ctx context.Context, groupID int) (GroupScheduleResponse, error) {
	key := "schedule:" + strconv.Itoa(groupID)
	var out GroupScheduleResponse
	path := "/schedule/lessons?" + url.Values{"groupId": {strconv.Itoa(groupID)}}.Encode()
	if err := c.cachedJSON(ctx, key, scheduleCacheTTL, path, &out); err != nil {
		return GroupScheduleResponse{}, err
	}
	return out, nil
}
