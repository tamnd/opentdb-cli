package opentdb_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/opentdb-cli/opentdb"
)

// fakeQuestionsJSON simulates the encode=url3986 response: special characters
// are percent-encoded, not HTML-entity-encoded.
const fakeQuestionsJSON = `{
  "response_code": 0,
  "results": [
    {
      "type": "multiple",
      "difficulty": "medium",
      "category": "Science%3A%20Computers",
      "question": "What%20does%20%22HTTP%22%20stand%20for%3F",
      "correct_answer": "HyperText%20Transfer%20Protocol",
      "incorrect_answers": [
        "High%20Transfer%20Text%20Protocol",
        "HyperText%20Transmission%20Protocol",
        "HyperType%20Transfer%20Protocol"
      ]
    },
    {
      "type": "boolean",
      "difficulty": "easy",
      "category": "Science%3A%20Computers",
      "question": "Linux%20was%20first%20created%20as%20an%20alternative%20to%20Windows%20XP.",
      "correct_answer": "False",
      "incorrect_answers": ["True"]
    }
  ]
}`

const fakeCategoriesJSON = `{
  "trivia_categories": [
    {"id": 9, "name": "General Knowledge"},
    {"id": 10, "name": "Entertainment: Books"},
    {"id": 11, "name": "Entertainment: Film"}
  ]
}`

const fakeCountJSON = `{
  "category_id": 18,
  "category_question_count": {
    "total_question_count": 192,
    "total_easy_question_count": 66,
    "total_medium_question_count": 83,
    "total_hard_question_count": 43
  }
}`

const fakeErrorJSON = `{"response_code": 1, "results": []}`
const fakeRateLimitJSON = `{"response_code": 5, "results": []}`

func newTestClient(ts *httptest.Server) *opentdb.Client {
	cfg := opentdb.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return opentdb.NewClient(cfg)
}

func TestQuestionsSendsUserAgent(t *testing.T) {
	var gotUA string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Questions(context.Background(), 2, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotUA == "" {
		t.Error("User-Agent not sent")
	}
}

func TestQuestionsParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Questions(context.Background(), 2, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].Rank != 1 {
		t.Errorf("items[0].Rank = %d, want 1", items[0].Rank)
	}
	if items[0].Category != "Science: Computers" {
		t.Errorf("items[0].Category = %q, want %q", items[0].Category, "Science: Computers")
	}
	if items[0].Difficulty != "medium" {
		t.Errorf("items[0].Difficulty = %q, want %q", items[0].Difficulty, "medium")
	}
	if items[0].CorrectAnswer != "HyperText Transfer Protocol" {
		t.Errorf("items[0].CorrectAnswer = %q", items[0].CorrectAnswer)
	}
	if len(items[0].IncorrectAnswers) != 3 {
		t.Errorf("len(items[0].IncorrectAnswers) = %d, want 3", len(items[0].IncorrectAnswers))
	}
}

// TestQuestionsURLDecoded checks that percent-encoded strings (encode=url3986)
// are decoded correctly back to plain text.
func TestQuestionsURLDecoded(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	items, err := c.Questions(context.Background(), 2, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := `What does "HTTP" stand for?`
	if items[0].Question != want {
		t.Errorf("items[0].Question = %q, want %q", items[0].Question, want)
	}
	if items[0].Category != "Science: Computers" {
		t.Errorf("items[0].Category = %q, want %q", items[0].Category, "Science: Computers")
	}
}

func TestQuestionsURLEncode(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Questions(context.Background(), 5, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(gotURL, "encode=url3986") {
		t.Errorf("URL %q should contain encode=url3986", gotURL)
	}
}

func TestQuestionsRetriesOn503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	cfg := opentdb.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := opentdb.NewClient(cfg)

	_, err := c.Questions(context.Background(), 2, 0, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
}

func TestCategoriesParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeCategoriesJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	cats, err := c.Categories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cats) != 3 {
		t.Fatalf("len(cats) = %d, want 3", len(cats))
	}
	if cats[0].Rank != 1 {
		t.Errorf("cats[0].Rank = %d, want 1", cats[0].Rank)
	}
	if cats[0].ID != 9 {
		t.Errorf("cats[0].ID = %d, want 9", cats[0].ID)
	}
	if cats[0].Name != "General Knowledge" {
		t.Errorf("cats[0].Name = %q, want %q", cats[0].Name, "General Knowledge")
	}
}

func TestCountParsesItems(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeCountJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	cc, err := c.Count(context.Background(), 18)
	if err != nil {
		t.Fatal(err)
	}
	if cc.CategoryID != 18 {
		t.Errorf("CategoryID = %d, want 18", cc.CategoryID)
	}
	if cc.Total != 192 {
		t.Errorf("Total = %d, want 192", cc.Total)
	}
	if cc.Easy != 66 {
		t.Errorf("Easy = %d, want 66", cc.Easy)
	}
	if cc.Medium != 83 {
		t.Errorf("Medium = %d, want 83", cc.Medium)
	}
	if cc.Hard != 43 {
		t.Errorf("Hard = %d, want 43", cc.Hard)
	}
}

func TestQuestionsErrorCode1(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeErrorJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Questions(context.Background(), 1, 0, "", "")
	if err == nil {
		t.Fatal("expected error for response_code=1, got nil")
	}
}

func TestQuestionsErrorCode5RateLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, fakeRateLimitJSON)
	}))
	defer ts.Close()

	cfg := opentdb.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 0 // no retries so the test is instant
	c := opentdb.NewClient(cfg)

	_, err := c.Questions(context.Background(), 1, 0, "", "")
	if err == nil {
		t.Fatal("expected error for response_code=5, got nil")
	}
}

func TestQuestionsURLHasCategory(t *testing.T) {
	var gotURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL = r.URL.String()
		_, _ = fmt.Fprint(w, fakeQuestionsJSON)
	}))
	defer ts.Close()

	c := newTestClient(ts)
	_, err := c.Questions(context.Background(), 2, 18, "medium", "multiple")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"category=18", "difficulty=medium", "type=multiple"} {
		if !contains(gotURL, want) {
			t.Errorf("URL %q missing %q", gotURL, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
