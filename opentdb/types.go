package opentdb

// Question is one trivia question from the Open Trivia Database.
type Question struct {
	Rank             int      `json:"rank"`
	Category         string   `kit:"id" json:"category"`
	Type             string   `json:"type"`              // "multiple" or "boolean"
	Difficulty       string   `json:"difficulty"`        // "easy", "medium", "hard"
	Question         string   `json:"question"`          // HTML-decoded
	CorrectAnswer    string   `json:"correct_answer"`    // HTML-decoded
	IncorrectAnswers []string `json:"incorrect_answers"` // HTML-decoded
}

// Category is one entry from the categories endpoint.
type Category struct {
	Rank int    `json:"rank"`
	ID   int    `kit:"id" json:"id"`
	Name string `json:"name"`
}

// CategoryCount holds the question count breakdown for one category.
type CategoryCount struct {
	CategoryID int `kit:"id" json:"category_id"`
	Total      int `json:"total"`
	Easy       int `json:"easy"`
	Medium     int `json:"medium"`
	Hard       int `json:"hard"`
}

// unexported: used inside opentdb.go for JSON decode only.

type apiResponse struct {
	ResponseCode int           `json:"response_code"`
	Results      []apiQuestion `json:"results"`
}

type apiQuestion struct {
	Type             string   `json:"type"`
	Difficulty       string   `json:"difficulty"`
	Category         string   `json:"category"`
	Question         string   `json:"question"`
	CorrectAnswer    string   `json:"correct_answer"`
	IncorrectAnswers []string `json:"incorrect_answers"`
}

type categoryResponse struct {
	TriviaCategories []apiCategory `json:"trivia_categories"`
}

type apiCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type countResponse struct {
	CategoryID            int `json:"category_id"`
	CategoryQuestionCount struct {
		Total  int `json:"total_question_count"`
		Easy   int `json:"total_easy_question_count"`
		Medium int `json:"total_medium_question_count"`
		Hard   int `json:"total_hard_question_count"`
	} `json:"category_question_count"`
}
