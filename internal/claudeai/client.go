// Package claudeai provides a client for fetching subscription and usage data from claude.ai.
package claudeai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	baseURL        = "https://claude.ai/api"
	requestTimeout = 10 * time.Second
	maxBodySize    = 1 << 20 // 1 MB
	keyPrefix      = "sk-ant-sid"
)

var (
	// ErrUnauthorized indicates the session key is expired or invalid.
	ErrUnauthorized = errors.New("claudeai: unauthorized (session key expired or invalid)")
	// ErrRateLimited indicates the API rate limit was hit.
	ErrRateLimited = errors.New("claudeai: rate limited")
)

// Client fetches subscription data from the claude.ai web API.
type Client struct {
	sessionKey string
	http       *http.Client
}

// NewClient creates a client for the given session key.
// Returns nil if the key is empty or has the wrong prefix.
func NewClient(sessionKey string) *Client {
	sessionKey = strings.TrimSpace(sessionKey)
	if sessionKey == "" {
		return nil
	}
	if !strings.HasPrefix(sessionKey, keyPrefix) {
		return nil
	}
	return &Client{
		sessionKey: sessionKey,
		http:       &http.Client{},
	}
}

// FetchAll fetches orgs, usage, and overage for the first organization.
// Partial data is returned even if some requests fail.
func (c *Client) FetchAll(ctx context.Context) *SubscriptionData {
	result := &SubscriptionData{FetchedAt: time.Now()}

	orgs, err := c.FetchOrganizations(ctx)
	if err != nil {
		result.Error = err
		return result
	}
	if len(orgs) == 0 {
		result.Error = errors.New("claudeai: no organizations found")
		return result
	}

	result.Org = orgs[0]
	orgID := orgs[0].UUID

	// Fetch usage and overage independently — partial results are fine
	usage, usageErr := c.FetchUsage(ctx, orgID)
	if usageErr == nil {
		result.Usage = usage
	}

	overage, overageErr := c.FetchOverageLimit(ctx, orgID)
	if overageErr == nil {
		result.Overage = overage
	}

	// Surface first non-nil error for status display
	if usageErr != nil {
		result.Error = usageErr
	} else if overageErr != nil {
		result.Error = overageErr
	}

	return result
}

// FetchOrganizations returns the list of organizations for this session.
func (c *Client) FetchOrganizations(ctx context.Context) ([]Organization, error) {
	body, err := c.get(ctx, "/organizations")
	if err != nil {
		return nil, err
	}

	var orgs []Organization
	if err := json.Unmarshal(body, &orgs); err != nil {
		return nil, fmt.Errorf("claudeai: parsing organizations: %w", err)
	}
	return orgs, nil
}

// FetchUsage returns parsed usage windows for the given organization.
func (c *Client) FetchUsage(ctx context.Context, orgID string) (*ParsedUsage, error) {
	body, err := c.get(ctx, fmt.Sprintf("/organizations/%s/usage", orgID))
	if err != nil {
		return nil, err
	}

	var raw UsageResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("claudeai: parsing usage: %w", err)
	}

	return &ParsedUsage{
		FiveHour:       parseWindow(raw.FiveHour),
		SevenDay:       parseWindow(raw.SevenDay),
		SevenDayOpus:   parseWindow(raw.SevenDayOpus),
		SevenDaySonnet: parseWindow(raw.SevenDaySonnet),
	}, nil
}

// FetchOverageLimit returns overage spend limit data for the given organization.
func (c *Client) FetchOverageLimit(ctx context.Context, orgID string) (*OverageLimit, error) {
	body, err := c.get(ctx, fmt.Sprintf("/organizations/%s/overage_spend_limit", orgID))
	if err != nil {
		return nil, err
	}

	var ol OverageLimit
	if err := json.Unmarshal(body, &ol); err != nil {
		return nil, fmt.Errorf("claudeai: parsing overage limit: %w", err)
	}
	return &ol, nil
}

// get performs an authenticated GET request and returns the response body.
func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("claudeai: creating request: %w", err)
	}

	req.Header.Set("Cookie", "sessionKey="+c.sessionKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "github.com/theirongolddev/cburn/1.0")

	//nolint:gosec // URL is constructed from const baseURL
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("claudeai: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	case http.StatusTooManyRequests:
		return nil, ErrRateLimited
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("claudeai: unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("claudeai: reading response: %w", err)
	}
	return body, nil
}

// parseWindow converts a raw UsageWindow into a normalized ParsedWindow.
// Returns nil if the input is nil or unparseable.
func parseWindow(w *UsageWindow) *ParsedWindow {
	if w == nil {
		return nil
	}

	pct, ok := parseUtilization(w.Utilization)
	if !ok {
		return nil
	}

	pw := &ParsedWindow{Pct: pct}

	if w.ResetsAt != nil {
		if t, err := time.Parse(time.RFC3339, *w.ResetsAt); err == nil {
			pw.ResetsAt = t
		}
	}

	return pw
}

// parseUtilization defensively parses the polymorphic utilization field.
// Handles int (75), float (0.75 or 75.0), and string ("75%" or "0.75").
// Returns value normalized to 0.0-1.0 range.
func parseUtilization(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}

	// Try number first (covers both int and float JSON)
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return normalizeUtilization(f), true
	}

	// Try string
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSuffix(strings.TrimSpace(s), "%")
		if v, err := strconv.ParseFloat(s, 64); err == nil {
			return normalizeUtilization(v), true
		}
	}

	return 0, false
}

// normalizeUtilization converts a value to 0.0-1.0 range.
// Values > 1.0 are assumed to be percentages (0-100 scale).
func normalizeUtilization(v float64) float64 {
	if v > 1.0 {
		return v / 100.0
	}
	return v
}
