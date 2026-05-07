package continua

import "errors"

type CompletionResult struct {
	Suggestion string `json:"suggestion"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
}

type EditResult struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
	Lines    int    `json:"lines"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatSession struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	Messages []ChatMessage `json:"messages"`
	Model    string        `json:"model"`
}

type DiffResult struct {
	FilePath   string `json:"file_path"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Patch      string `json:"patch"`
}

var (
	ErrCompletionFailed = errors.New("completion generation failed")
	ErrEditorFailed     = errors.New("workspace editor operation failed")
	ErrChatFailed       = errors.New("chat operation failed")
)
