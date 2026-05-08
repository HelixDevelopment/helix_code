package clarification

import "time"

type QuestionType string

const (
	YesNo          QuestionType = "yes_no"
	MultipleChoice QuestionType = "multiple_choice"
	FreeText       QuestionType = "free_text"
)

type Question struct {
	ID      string       `json:"id"`
	Type    QuestionType `json:"type"`
	Text    string       `json:"text"`
	Options []string     `json:"options,omitempty"`
	Default string       `json:"default,omitempty"`
}

type Answer struct {
	QuestionID string `json:"question_id"`
	Value      string `json:"value"`
}

type Session struct {
	ID        string     `json:"id"`
	Questions []Question `json:"questions"`
	Answers   []Answer   `json:"answers"`
	Context   string     `json:"context"`
	Timeout   time.Duration
}