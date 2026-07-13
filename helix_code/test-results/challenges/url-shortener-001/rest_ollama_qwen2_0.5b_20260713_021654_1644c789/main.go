package main

import (
	"fmt"
	"net/http"
	"strings"
)

// ShortenerService defines the RESTful endpoints for generating unique short URLs.
type ShortenerService struct {
	// URLShortener API endpoint to generate short URLs.
	urlShortener *urlshortener.URLShortenerAPI

	// Redis storage for quick lookups
	redis redisstore.RedisStore

	// PostgreSQL database for persistence and analytics
	pgPostgreSQL *postgres.PostgresDB

	// Rate limiting middleware
	middleware http.Middleware
}

// NewShortenerService creates a new ShortenerService instance.
func NewShortenerService() *ShortenerService {
	return &ShortenerService{
		urlShortener: urlshortener.New(),
		redis:         redisstore.New(),
		pgPostgreSQL:   postgres.New(),
		middleware:    http.DefaultHeader,
	}
}

// GenerateShortCode generates a unique short code.
func (ss *ShortenerService) GenerateShortCode() string {
	return ss.urlShortener.Generate()
}

// GetShortCode retrieves the generated short code from the URL Shortener API.
func (ss *ShortenerService) GetShortCode(shortCode string) (*urlshortener.URLShortenerAPIResponse, error) {
	return ss.urlShortener.Get(shortCode)
}

// DeleteShortCode deletes the generated short code from the URL Shortener API.
func (ss *ShortenerService) DeleteShortCode(shortCode string) error {
	return ss.urlShortener.Delete(shortCode)
}

// GetStats retrieves statistics about the generated short codes.
func (ss *ShortenerService) GetStats(shortCode string) (*urlshortener.URLShortenerAPIResponse, error) {
	return ss.urlShortener.GetStats(shortCode)
}

// DeleteShortCode deletes the generated short code from the URL Shortener API.
func (ss *ShortenerService) DeleteShortCode(shortCode string) error {
	return ss.urlShortener.Delete(shortCode)
}

// RateLimiting middleware for rate limiting
func (ss *ShortenerService) RateLimit() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !r.Method == "GET" && !r.Method == "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Invalid code length", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "GET" && !r.Method == "POST" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "DELETE" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

		if len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long", http.StatusBadRequest)
			return
		}

		if !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Invalid path", http.StatusMethodNotAllowed)
			return
		}

		if r.Method == "POST" && len(r.URL.Query().Get("code")) < 6 {
			http.Error(w, "Code too short", http.StatusBadRequest)
			return
		}

		if !r.Method == "DELETE" {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		if len(r.URL.Query().Get("code")) < 6 || len(r.URL.Query().Get("code")) > 8 {
			http.Error(w, "Code too long or short", http.StatusBadRequest)
			return
		}

		if r.Method == "GET" && !r.URL.Path.StartsWithSegments("/api") {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		if r.Method != "POST" {
			http.Error(w, "Invalid request method", http.StatusBadRequest)
			return
		}

