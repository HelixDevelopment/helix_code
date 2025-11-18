package challenges

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// MockGenerator generates mock project code for testing the validation framework
// This is temporary until we integrate with the real HelixCode
type MockGenerator struct{}

// NewMockGenerator creates a new mock generator
func NewMockGenerator() *MockGenerator {
	return &MockGenerator{}
}

// GenerateNotesProject generates a mock Notes project for testing
func (g *MockGenerator) GenerateNotesProject(ctx context.Context, outputDir string) error {
	// Create directory structure
	dirs := []string{
		filepath.Join(outputDir, "cmd", "server"),
		filepath.Join(outputDir, "internal", "api"),
		filepath.Join(outputDir, "internal", "models"),
		filepath.Join(outputDir, "internal", "db"),
		filepath.Join(outputDir, "migrations"),
		filepath.Join(outputDir, "tests"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create go.mod
	goMod := `module notes-app

go 1.24

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/google/uuid v1.6.0
	github.com/lib/pq v1.10.9
)
`
	if err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Create main.go
	mainGo := `package main

import (
	"log"
	"notes-app/internal/api"
	"notes-app/internal/db"

	"github.com/gin-gonic/gin"
)

func main() {
	// Initialize database
	database, err := db.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Setup router
	router := gin.Default()
	api.SetupRoutes(router, database)

	// Start server
	log.Println("Server starting on :8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "cmd", "server", "main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	// Create models
	modelsGo := `package models

import (
	"time"

	"github.com/google/uuid"
)

type Note struct {
	ID        uuid.UUID ` + "`json:\"id\"`" + `
	Title     string    ` + "`json:\"title\"`" + `
	Content   string    ` + "`json:\"content\"`" + `
	Tags      []string  ` + "`json:\"tags\"`" + `
	CreatedAt time.Time ` + "`json:\"created_at\"`" + `
	UpdatedAt time.Time ` + "`json:\"updated_at\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "internal", "models", "note.go"), []byte(modelsGo), 0644); err != nil {
		return err
	}

	// Create API handlers
	apiGo := `package api

import (
	"net/http"
	"notes-app/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type NotesAPI struct {
	db interface{}
}

func SetupRoutes(router *gin.Engine, db interface{}) {
	api := &NotesAPI{db: db}

	router.GET("/notes", api.ListNotes)
	router.GET("/notes/:id", api.GetNote)
	router.POST("/notes", api.CreateNote)
	router.PUT("/notes/:id", api.UpdateNote)
	router.DELETE("/notes/:id", api.DeleteNote)
	router.GET("/notes/search", api.SearchNotes)
}

func (api *NotesAPI) ListNotes(c *gin.Context) {
	notes := []models.Note{}
	c.JSON(http.StatusOK, gin.H{"notes": notes})
}

func (api *NotesAPI) GetNote(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"note": models.Note{ID: uuid.New()}})
}

func (api *NotesAPI) CreateNote(c *gin.Context) {
	var note models.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	note.ID = uuid.New()
	c.JSON(http.StatusCreated, gin.H{"note": note})
}

func (api *NotesAPI) UpdateNote(c *gin.Context) {
	id := c.Param("id")
	var note models.Note
	if err := c.ShouldBindJSON(&note); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"note": note, "id": id})
}

func (api *NotesAPI) DeleteNote(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"message": "deleted", "id": id})
}

func (api *NotesAPI) SearchNotes(c *gin.Context) {
	query := c.Query("q")
	notes := []models.Note{}
	c.JSON(http.StatusOK, gin.H{"notes": notes, "query": query})
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "internal", "api", "notes.go"), []byte(apiGo), 0644); err != nil {
		return err
	}

	// Create database package
	dbGo := `package db

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func Connect() (*sql.DB, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "5432"
	}

	user := os.Getenv("DB_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		dbname = "notes"
	}

	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "internal", "db", "db.go"), []byte(dbGo), 0644); err != nil {
		return err
	}

	// Create tests
	testGo := `package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestListNotes(t *testing.T) {
	router := gin.Default()
	SetupRoutes(router, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/notes", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestGetNote(t *testing.T) {
	router := gin.Default()
	SetupRoutes(router, nil)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/notes/123", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "internal", "api", "notes_test.go"), []byte(testGo), 0644); err != nil {
		return err
	}

	// Create README
	readme := "# Notes Application\n\n" +
		"A simple RESTful API for managing notes.\n\n" +
		"## Features\n\n" +
		"- Create, read, update, and delete notes\n" +
		"- Search notes by content\n" +
		"- Tag support\n" +
		"- PostgreSQL persistence\n\n" +
		"## Setup\n\n" +
		"1. Install dependencies:\n" +
		"```bash\n" +
		"go mod download\n" +
		"```\n\n" +
		"2. Set environment variables:\n" +
		"```bash\n" +
		"export DB_HOST=localhost\n" +
		"export DB_PORT=5432\n" +
		"export DB_USER=postgres\n" +
		"export DB_PASSWORD=postgres\n" +
		"export DB_NAME=notes\n" +
		"```\n\n" +
		"3. Run the server:\n" +
		"```bash\n" +
		"go run cmd/server/main.go\n" +
		"```\n\n" +
		"## API Endpoints\n\n" +
		"- `GET /notes` - List all notes\n" +
		"- `GET /notes/:id` - Get a specific note\n" +
		"- `POST /notes` - Create a new note\n" +
		"- `PUT /notes/:id` - Update a note\n" +
		"- `DELETE /notes/:id` - Delete a note\n" +
		"- `GET /notes/search?q=query` - Search notes\n\n" +
		"## Testing\n\n" +
		"```bash\n" +
		"go test ./...\n" +
		"```\n"
	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}

	// Create Dockerfile
	dockerfile := `FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o server cmd/server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
`
	if err := os.WriteFile(filepath.Join(outputDir, "Dockerfile"), []byte(dockerfile), 0644); err != nil {
		return err
	}

	// Create docker-compose.yml
	dockerCompose := `version: '3.8'

services:
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
      POSTGRES_DB: notes
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      DB_HOST: postgres
      DB_PORT: 5432
      DB_USER: postgres
      DB_PASSWORD: postgres
      DB_NAME: notes
    depends_on:
      - postgres

volumes:
  postgres_data:
`
	if err := os.WriteFile(filepath.Join(outputDir, "docker-compose.yml"), []byte(dockerCompose), 0644); err != nil {
		return err
	}

	// Create .gitignore
	gitignore := `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
server

# Test binary
*.test

# Output of the go coverage tool
*.out

# Go workspace file
go.work

# IDE
.vscode/
.idea/
*.swp
*.swo
*~
`
	if err := os.WriteFile(filepath.Join(outputDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return err
	}

	// Create go.sum (empty for now)
	if err := os.WriteFile(filepath.Join(outputDir, "go.sum"), []byte(""), 0644); err != nil {
		return err
	}

	return nil
}

// GenerateTicTacToeGame generates a cross-platform TUI tic-tac-toe game
func (g *MockGenerator) GenerateTicTacToeGame(ctx context.Context, outputDir string) error {
	// Create directory structure
	dirs := []string{
		filepath.Join(outputDir, "game"),
		filepath.Join(outputDir, "ui"),
		filepath.Join(outputDir, "tests"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Create go.mod
	goMod := `module tic-tac-toe

go 1.24

require (
	github.com/charmbracelet/bubbletea v0.25.0
	github.com/charmbracelet/lipgloss v0.9.1
)
`
	if err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return err
	}

	// Create main.go
	mainGo := `package main

import (
	"fmt"
	"os"
	"tic-tac-toe/game"
	"tic-tac-toe/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Create new game
	g := game.NewGame()

	// Create TUI model
	m := ui.NewModel(g)

	// Start the TUI
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running game: %v\n", err)
		os.Exit(1)
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "main.go"), []byte(mainGo), 0644); err != nil {
		return err
	}

	// Create game/board.go
	boardGo := `package game

import "fmt"

type Cell int

const (
	Empty Cell = iota
	X
	O
)

type Board struct {
	cells [3][3]Cell
}

func NewBoard() *Board {
	return &Board{}
}

func (b *Board) Get(row, col int) Cell {
	if row < 0 || row > 2 || col < 0 || col > 2 {
		return Empty
	}
	return b.cells[row][col]
}

func (b *Board) Set(row, col int, cell Cell) error {
	if row < 0 || row > 2 || col < 0 || col > 2 {
		return fmt.Errorf("invalid position: (%d, %d)", row, col)
	}
	if b.cells[row][col] != Empty {
		return fmt.Errorf("cell already occupied")
	}
	b.cells[row][col] = cell
	return nil
}

func (b *Board) IsFull() bool {
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			if b.cells[row][col] == Empty {
				return false
			}
		}
	}
	return true
}

func (b *Board) CheckWin(player Cell) bool {
	// Check rows
	for row := 0; row < 3; row++ {
		if b.cells[row][0] == player && b.cells[row][1] == player && b.cells[row][2] == player {
			return true
		}
	}

	// Check columns
	for col := 0; col < 3; col++ {
		if b.cells[0][col] == player && b.cells[1][col] == player && b.cells[2][col] == player {
			return true
		}
	}

	// Check diagonals
	if b.cells[0][0] == player && b.cells[1][1] == player && b.cells[2][2] == player {
		return true
	}
	if b.cells[0][2] == player && b.cells[1][1] == player && b.cells[2][0] == player {
		return true
	}

	return false
}

func (b *Board) Reset() {
	b.cells = [3][3]Cell{}
}

func (c Cell) String() string {
	switch c {
	case X:
		return "X"
	case O:
		return "O"
	default:
		return " "
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "game", "board.go"), []byte(boardGo), 0644); err != nil {
		return err
	}

	// Create game/player.go
	playerGo := `package game

type Player struct {
	Symbol Cell
	Name   string
}

func NewPlayer(symbol Cell, name string) *Player {
	return &Player{
		Symbol: symbol,
		Name:   name,
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "game", "player.go"), []byte(playerGo), 0644); err != nil {
		return err
	}

	// Create game/game.go
	gameGo := `package game

type GameState int

const (
	Playing GameState = iota
	Won
	Draw
)

type Game struct {
	Board         *Board
	CurrentPlayer *Player
	Player1       *Player
	Player2       *Player
	State         GameState
	Winner        *Player
}

func NewGame() *Game {
	return &Game{
		Board:         NewBoard(),
		Player1:       NewPlayer(X, "Player 1"),
		Player2:       NewPlayer(O, "Player 2"),
		CurrentPlayer: NewPlayer(X, "Player 1"),
		State:         Playing,
	}
}

func (g *Game) MakeMove(row, col int) error {
	if g.State != Playing {
		return nil
	}

	err := g.Board.Set(row, col, g.CurrentPlayer.Symbol)
	if err != nil {
		return err
	}

	// Check for win
	if g.Board.CheckWin(g.CurrentPlayer.Symbol) {
		g.State = Won
		g.Winner = g.CurrentPlayer
		return nil
	}

	// Check for draw
	if g.Board.IsFull() {
		g.State = Draw
		return nil
	}

	// Switch player
	g.SwitchPlayer()

	return nil
}

func (g *Game) SwitchPlayer() {
	if g.CurrentPlayer.Symbol == X {
		g.CurrentPlayer = g.Player2
	} else {
		g.CurrentPlayer = g.Player1
	}
}

func (g *Game) Reset() {
	g.Board.Reset()
	g.CurrentPlayer = g.Player1
	g.State = Playing
	g.Winner = nil
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "game", "game.go"), []byte(gameGo), 0644); err != nil {
		return err
	}

	// Create ui/tui.go
	tuiGo := `package ui

import (
	"fmt"
	"tic-tac-toe/game"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#00FF00")).
		MarginBottom(1)

	boardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#874BFD")).
		Padding(1, 2)

	cellStyle = lipgloss.NewStyle().
		Width(5).
		Height(3).
		Align(lipgloss.Center, lipgloss.Center)

	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFF00")).
		MarginTop(1)
)

type Model struct {
	game     *game.Game
	cursorX  int
	cursorY  int
	quitting bool
}

func NewModel(g *game.Game) Model {
	return Model{
		game:    g,
		cursorX: 1,
		cursorY: 1,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit

		case "r":
			m.game.Reset()
			m.cursorX = 1
			m.cursorY = 1

		case "up", "k":
			if m.cursorY > 0 {
				m.cursorY--
			}

		case "down", "j":
			if m.cursorY < 2 {
				m.cursorY++
			}

		case "left", "h":
			if m.cursorX > 0 {
				m.cursorX--
			}

		case "right", "l":
			if m.cursorX < 2 {
				m.cursorX++
			}

		case "enter", " ":
			if m.game.State == game.Playing {
				m.game.MakeMove(m.cursorY, m.cursorX)
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return "Thanks for playing!\n"
	}

	s := titleStyle.Render("🎮 Tic-Tac-Toe") + "\n\n"

	// Render board
	board := ""
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			cell := m.game.Board.Get(row, col)
			cellContent := cell.String()

			style := cellStyle
			if row == m.cursorY && col == m.cursorX && m.game.State == game.Playing {
				style = style.Border(lipgloss.DoubleBorder()).
					BorderForeground(lipgloss.Color("#FF00FF"))
			} else {
				style = style.Border(lipgloss.NormalBorder())
			}

			board += style.Render(cellContent)
		}
		board += "\n"
	}

	s += boardStyle.Render(board) + "\n"

	// Status message
	status := ""
	switch m.game.State {
	case game.Playing:
		status = fmt.Sprintf("Current Player: %s (%s)",
			m.game.CurrentPlayer.Name,
			m.game.CurrentPlayer.Symbol.String())
	case game.Won:
		status = fmt.Sprintf("🎉 %s wins!", m.game.Winner.Name)
	case game.Draw:
		status = "It's a draw!"
	}

	s += statusStyle.Render(status) + "\n\n"

	// Controls
	s += "Controls: ↑↓←→/hjkl to move, Enter/Space to place, R to restart, Q to quit\n"

	return s
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "ui", "tui.go"), []byte(tuiGo), 0644); err != nil {
		return err
	}

	// Create tests/board_test.go
	testGo := `package game

import (
	"testing"
	"tic-tac-toe/game"
)

func TestBoardSet(t *testing.T) {
	b := game.NewBoard()

	err := b.Set(0, 0, game.X)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if b.Get(0, 0) != game.X {
		t.Errorf("Expected X, got %v", b.Get(0, 0))
	}
}

func TestBoardSetOccupied(t *testing.T) {
	b := game.NewBoard()

	b.Set(0, 0, game.X)
	err := b.Set(0, 0, game.O)

	if err == nil {
		t.Error("Expected error for occupied cell")
	}
}

func TestBoardCheckWinRow(t *testing.T) {
	b := game.NewBoard()

	b.Set(0, 0, game.X)
	b.Set(0, 1, game.X)
	b.Set(0, 2, game.X)

	if !b.CheckWin(game.X) {
		t.Error("Expected X to win")
	}
}

func TestBoardCheckWinColumn(t *testing.T) {
	b := game.NewBoard()

	b.Set(0, 0, game.O)
	b.Set(1, 0, game.O)
	b.Set(2, 0, game.O)

	if !b.CheckWin(game.O) {
		t.Error("Expected O to win")
	}
}

func TestBoardCheckWinDiagonal(t *testing.T) {
	b := game.NewBoard()

	b.Set(0, 0, game.X)
	b.Set(1, 1, game.X)
	b.Set(2, 2, game.X)

	if !b.CheckWin(game.X) {
		t.Error("Expected X to win")
	}
}

func TestBoardIsFull(t *testing.T) {
	b := game.NewBoard()

	// Fill board
	for row := 0; row < 3; row++ {
		for col := 0; col < 3; col++ {
			b.Set(row, col, game.X)
		}
	}

	if !b.IsFull() {
		t.Error("Expected board to be full")
	}
}
`
	if err := os.WriteFile(filepath.Join(outputDir, "tests", "board_test.go"), []byte(testGo), 0644); err != nil {
		return err
	}

	// Create README.md
	readme := "# Tic-Tac-Toe TUI Game\n\n" +
		"A cross-platform terminal user interface tic-tac-toe game built with Go and Bubble Tea.\n\n" +
		"## Features\n\n" +
		"- Beautiful terminal UI with colors and borders\n" +
		"- Two-player local multiplayer\n" +
		"- Keyboard navigation with arrow keys or vim keys (hjkl)\n" +
		"- Win detection for rows, columns, and diagonals\n" +
		"- Draw detection\n" +
		"- Game restart functionality\n" +
		"- Cross-platform support (Linux, macOS, Windows, Harmony OS)\n\n" +
		"## Installation\n\n" +
		"1. Install dependencies:\n" +
		"```bash\n" +
		"go mod download\n" +
		"```\n\n" +
		"2. Build the game:\n" +
		"```bash\n" +
		"go build -o tic-tac-toe\n" +
		"```\n\n" +
		"## Usage\n\n" +
		"Run the game:\n" +
		"```bash\n" +
		"./tic-tac-toe\n" +
		"```\n\n" +
		"### Controls\n\n" +
		"- **Arrow Keys** or **hjkl**: Move cursor\n" +
		"- **Enter** or **Space**: Place your mark (X or O)\n" +
		"- **R**: Restart game\n" +
		"- **Q** or **Ctrl+C**: Quit game\n\n" +
		"## Game Rules\n\n" +
		"1. Two players take turns placing their marks (X and O) on a 3x3 grid\n" +
		"2. Player 1 is X, Player 2 is O\n" +
		"3. The first player to get 3 of their marks in a row (horizontally, vertically, or diagonally) wins\n" +
		"4. If all 9 squares are filled and no player has won, the game is a draw\n\n" +
		"## Cross-Platform Support\n\n" +
		"This game is built with Go and uses the Bubble Tea TUI framework, making it compatible with:\n\n" +
		"- **Linux**: All major distributions\n" +
		"- **macOS**: 10.12 and later\n" +
		"- **Windows**: Windows 10 and later\n" +
		"- **Harmony OS**: Via Go cross-compilation\n\n" +
		"## Project Structure\n\n" +
		"```\n" +
		"tic-tac-toe/\n" +
		"├── main.go           # Entry point\n" +
		"├── game/             # Game logic\n" +
		"│   ├── board.go      # Board state and operations\n" +
		"│   ├── player.go     # Player data\n" +
		"│   └── game.go       # Game state management\n" +
		"├── ui/               # Terminal UI\n" +
		"│   └── tui.go        # Bubble Tea TUI implementation\n" +
		"└── tests/            # Tests\n" +
		"    └── board_test.go # Board logic tests\n" +
		"```\n\n" +
		"## Testing\n\n" +
		"Run tests:\n" +
		"```bash\n" +
		"go test ./...\n" +
		"```\n\n" +
		"Run with coverage:\n" +
		"```bash\n" +
		"go test -cover ./...\n" +
		"```\n\n" +
		"## Building for Different Platforms\n\n" +
		"### Linux\n" +
		"```bash\n" +
		"GOOS=linux GOARCH=amd64 go build -o tic-tac-toe-linux\n" +
		"```\n\n" +
		"### macOS\n" +
		"```bash\n" +
		"GOOS=darwin GOARCH=amd64 go build -o tic-tac-toe-macos\n" +
		"```\n\n" +
		"### Windows\n" +
		"```bash\n" +
		"GOOS=windows GOARCH=amd64 go build -o tic-tac-toe.exe\n" +
		"```\n\n" +
		"### Harmony OS\n" +
		"```bash\n" +
		"GOOS=linux GOARCH=arm64 go build -o tic-tac-toe-harmonyos\n" +
		"```\n"

	if err := os.WriteFile(filepath.Join(outputDir, "README.md"), []byte(readme), 0644); err != nil {
		return err
	}

	// Create .gitignore
	gitignore := `# Binaries
*.exe
*.exe~
*.dll
*.so
*.dylib
tic-tac-toe
tic-tac-toe-*

# Test binary
*.test

# Output
*.out

# Go workspace
go.work

# IDE
.vscode/
.idea/
*.swp
*.swo
*~
`
	if err := os.WriteFile(filepath.Join(outputDir, ".gitignore"), []byte(gitignore), 0644); err != nil {
		return err
	}

	// Create go.sum (empty for now)
	if err := os.WriteFile(filepath.Join(outputDir, "go.sum"), []byte(""), 0644); err != nil {
		return err
	}

	return nil
}
