package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SearchResult represents a single search result
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Source  string `json:"source"` // "tavily" or "serper"
}

// WebSearchService handles web search operations using Tavily or Serper APIs
type WebSearchService struct {
	tavilyAPIKey string
	serperAPIKey string
	client       *http.Client
}

// NewWebSearchService creates a new web search service
func NewWebSearchService(tavilyKey, serperKey string) *WebSearchService {
	return &WebSearchService{
		tavilyAPIKey: tavilyKey,
		serperAPIKey: serperKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search performs a web search with the given query
// It tries Tavily first, then falls back to Serper if Tavily fails
func (s *WebSearchService) Search(query string, maxResults int) ([]SearchResult, error) {
	// Try Tavily first (more comprehensive)
	if s.tavilyAPIKey != "" {
		results, err := s.searchTavily(query, maxResults)
		if err == nil && len(results) > 0 {
			for i := range results {
				results[i].Source = "tavily"
			}
			return results, nil
		}
		slog.Debug("Tavily search failed, trying Serper", "error", err)
	}

	// Fall back to Serper
	if s.serperAPIKey != "" {
		results, err := s.searchSerper(query, maxResults)
		if err == nil && len(results) > 0 {
			for i := range results {
				results[i].Source = "serper"
			}
			return results, nil
		}
		slog.Debug("Serper search failed", "error", err)
	}

	return nil, fmt.Errorf("both Tavily and Serper search failed")
}

// searchTavily performs a search using Tavily API
func (s *WebSearchService) searchTavily(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 || maxResults > 100 {
		maxResults = 10
	}

	reqBody := map[string]interface{}{
		"api_key": s.tavilyAPIKey,
		"query":   query,
		"max_results": maxResults,
		"search_depth": "basic",
		"include_answer": false,
		"include_raw_content": false,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Tavily API returned status %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)

	var tavilyResp struct {
		Answer string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Score   float64 `json:"score"`
		} `json:"results"`
	}

	if err := json.Unmarshal(respBody, &tavilyResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(tavilyResp.Results))
	for _, r := range tavilyResp.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: truncateContent(r.Content, 300),
		})
	}

	return results, nil
}

// searchSerper performs a search using Serper.dev API
func (s *WebSearchService) searchSerper(query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 || maxResults > 100 {
		maxResults = 10
	}

	reqBody := map[string]interface{}{
		"q": query,
		"num": maxResults,
	}

	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequest("POST", "https://google.serper.dev/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-KEY", s.serperAPIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Serper API returned status %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)

	var serperResp struct {
		Organic []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"organic"`
	}

	if err := json.Unmarshal(respBody, &serperResp); err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(serperResp.Organic))
	for _, r := range serperResp.Organic {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.Link,
			Snippet: r.Snippet,
		})
	}

	return results, nil
}

// FormatResults returns a formatted string of search results for AI context
func (s *WebSearchService) FormatResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var formatted string
	for i, r := range results {
		formatted += fmt.Sprintf("%d. **%s**\n   URL: %s\n   %s\n\n", i+1, r.Title, r.URL, r.Snippet)
	}
	return formatted
}

// truncateContent truncates content to max length
func truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}
