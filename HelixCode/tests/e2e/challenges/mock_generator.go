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
