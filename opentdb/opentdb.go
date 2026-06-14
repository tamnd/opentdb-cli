// Package opentdb is the library behind the opentdb command line:
// the HTTP client, request shaping, and the typed data models for the Open
// Trivia Database (opentdb.com).
//
// The Client sets a real User-Agent, paces requests so a busy session stays
// polite, and retries transient failures (429 and 5xx). Questions and Categories
// are the two endpoints this client covers; no API key required.
package opentdb

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "opentdb.com"

// Config holds all tunable parameters for the Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://opentdb.com",
		UserAgent: "opentdb-cli/0.1 (tamnd87@gmail.com)",
		Rate:      500 * time.Millisecond,
		Timeout:   15 * time.Second,
		Retries:   3,
	}
}

// Client talks to opentdb over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured with cfg.
func NewClient(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// Questions fetches trivia questions from opentdb.
// amount must be 1–50. Pass 0 for category, "" for difficulty and qtype to use
// the API defaults (all categories, all difficulties, all types).
// The request uses encode=url3986 so that special characters arrive as
// percent-encoded bytes; each string field is decoded with url.QueryUnescape.
func (c *Client) Questions(ctx context.Context, amount int, category int, difficulty string, qtype string) ([]Question, error) {
	u := fmt.Sprintf("%s/api.php?amount=%d&encode=url3986", c.cfg.BaseURL, amount)
	if category > 0 {
		u += fmt.Sprintf("&category=%d", category)
	}
	if difficulty != "" {
		u += "&difficulty=" + difficulty
	}
	if qtype != "" {
		u += "&type=" + qtype
	}

	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp apiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode questions: %w", err)
	}

	switch resp.ResponseCode {
	case 0:
		// success
	case 1:
		return nil, fmt.Errorf("no results for the given parameters")
	case 2:
		return nil, fmt.Errorf("invalid parameter")
	case 5:
		return nil, fmt.Errorf("rate limited by API")
	default:
		return nil, fmt.Errorf("API error code %d", resp.ResponseCode)
	}

	items := make([]Question, 0, len(resp.Results))
	for i, r := range resp.Results {
		ia := make([]string, len(r.IncorrectAnswers))
		for j, a := range r.IncorrectAnswers {
			ia[j] = unescape(a)
		}
		items = append(items, Question{
			Rank:             i + 1,
			Category:         unescape(r.Category),
			Type:             unescape(r.Type),
			Difficulty:       unescape(r.Difficulty),
			Question:         unescape(r.Question),
			CorrectAnswer:    unescape(r.CorrectAnswer),
			IncorrectAnswers: ia,
		})
	}
	return items, nil
}

// unescape decodes a percent-encoded string as returned by encode=url3986.
// If decoding fails the original string is returned unchanged.
func unescape(s string) string {
	if dec, err := url.QueryUnescape(s); err == nil {
		return dec
	}
	return s
}

// Categories fetches the complete list of trivia categories.
func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	u := c.cfg.BaseURL + "/api_category.php"
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}

	var resp categoryResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode categories: %w", err)
	}

	cats := make([]Category, 0, len(resp.TriviaCategories))
	for i, a := range resp.TriviaCategories {
		cats = append(cats, Category{
			Rank: i + 1,
			ID:   a.ID,
			Name: a.Name,
		})
	}
	return cats, nil
}

// Count fetches the question count breakdown for a given category ID.
func (c *Client) Count(ctx context.Context, categoryID int) (CategoryCount, error) {
	u := fmt.Sprintf("%s/api_count.php?category=%d", c.cfg.BaseURL, categoryID)
	body, err := c.get(ctx, u)
	if err != nil {
		return CategoryCount{}, err
	}

	var resp countResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return CategoryCount{}, fmt.Errorf("decode count: %w", err)
	}

	return CategoryCount{
		CategoryID: resp.CategoryID,
		Total:      resp.CategoryQuestionCount.Total,
		Easy:       resp.CategoryQuestionCount.Easy,
		Medium:     resp.CategoryQuestionCount.Medium,
		Hard:       resp.CategoryQuestionCount.Hard,
	}, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	return min(time.Duration(attempt)*500*time.Millisecond, 5*time.Second)
}
