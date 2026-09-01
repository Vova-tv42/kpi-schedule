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

	"kpi-schedule-bot/server/internal/cache"
)

const baseURL = "https://api.campus.kpi.ua"

type Client struct {
	http *http.Client

	timeCache     *cache.TTL[CurrentAcademicTime]
	slotsCache    *cache.TTL[map[string]string]
	groupsCache   *cache.TTL[[]Group]
	scheduleCache *cache.TTL[GroupScheduleResponse]
}

func NewClient() *Client {
	return &Client{
		http:          &http.Client{Timeout: 10 * time.Second},
		timeCache:     cache.New[CurrentAcademicTime](1 * time.Minute),
		slotsCache:    cache.New[map[string]string](24 * time.Hour),
		groupsCache:   cache.New[[]Group](24 * time.Hour),
		scheduleCache: cache.New[GroupScheduleResponse](6 * time.Hour),
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

// CurrentTime returns the current academic week/day/slot, cached for 1 minute.
func (c *Client) CurrentTime(ctx context.Context) (CurrentAcademicTime, error) {
	if v, ok := c.timeCache.Get("current"); ok {
		return v, nil
	}
	var out CurrentAcademicTime
	if err := c.getJSON(ctx, "/time/current", &out); err != nil {
		return CurrentAcademicTime{}, err
	}
	c.timeCache.Set("current", out)
	return out, nil
}

// LessonSlots returns the slot-number -> start-time map, cached for 24 hours.
func (c *Client) LessonSlots(ctx context.Context) (map[string]string, error) {
	if v, ok := c.slotsCache.Get("slots"); ok {
		return v, nil
	}
	var out map[string]string
	if err := c.getJSON(ctx, "/schedule/lessons/slots", &out); err != nil {
		return nil, err
	}
	c.slotsCache.Set("slots", out)
	return out, nil
}

// Groups returns the full group catalog, cached for 24 hours.
func (c *Client) Groups(ctx context.Context) ([]Group, error) {
	if v, ok := c.groupsCache.Get("all"); ok {
		return v, nil
	}
	var out []Group
	if err := c.getJSON(ctx, "/group/all", &out); err != nil {
		return nil, err
	}
	c.groupsCache.Set("all", out)
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
	key := strconv.Itoa(groupID)
	if v, ok := c.scheduleCache.Get(key); ok {
		return v, nil
	}
	var out GroupScheduleResponse
	path := "/schedule/lessons?" + url.Values{"groupId": {key}}.Encode()
	if err := c.getJSON(ctx, path, &out); err != nil {
		return GroupScheduleResponse{}, err
	}
	c.scheduleCache.Set(key, out)
	return out, nil
}
