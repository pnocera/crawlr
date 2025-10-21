package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"
	"time"

	"crawlr/internal/config"
	"crawlr/internal/errors"
	"crawlr/internal/logger"
	"crawlr/internal/progress"
	"crawlr/internal/storage"
)

// Crawler represents the HTTP client for communicating with the crawl4ai API
type Crawler struct {
	client        *http.Client
	serverURL     string
	timeout       time.Duration
	maxConcurrent int
	includeMedia  bool
	authToken     string
	logger        *logger.Logger
	storage       *storage.Storage
}

// NewCrawler creates a new Crawler instance with the provided configuration
func NewCrawler(cfg *config.Config, logger *logger.Logger) *Crawler {
	return &Crawler{
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		serverURL:     cfg.ServerURL,
		timeout:       time.Duration(cfg.Timeout) * time.Second,
		maxConcurrent: cfg.MaxConcurrent,
		includeMedia:  cfg.IncludeMedia,
		logger:        logger,
	}
}

// SetStorage sets the storage instance for saving crawled content
func (c *Crawler) SetStorage(storage *storage.Storage) {
	c.storage = storage
}

// SetAuthToken sets the authentication token for API requests
func (c *Crawler) SetAuthToken(token string) {
	c.authToken = token
}

// StartCrawlRequest represents the request to start a crawling job
type StartCrawlRequest struct {
	Urls                 []string               `json:"urls"`                     // URLs array as expected by crawl4ai API
	IncludeRawHTML       bool                   `json:"include_raw_html,omitempty"`
	WordCountThreshold   int                    `json:"word_count_threshold,omitempty"`
	Priority             int                    `json:"priority,omitempty"`
	TTL                  int                    `json:"ttl,omitempty"`
	// Crawl4ai crawler configuration for multi-URL crawling
	CrawlerConfig        CrawlerConfig          `json:"crawler_config,omitempty"`
	// Extraction and processing options
	ProcessURLs          bool                   `json:"process_urls,omitempty"`
	// Browser configuration for crawling
	BrowserConfig        map[string]interface{} `json:"browser_config,omitempty"`
}

// CrawlerConfig contains configuration for automatic URL discovery and crawling
type CrawlerConfig struct {
	MaxDepth        int    `json:"max_depth,omitempty"`
	MaxURLs         int    `json:"max_urls,omitempty"`
	Strategy        string `json:"strategy,omitempty"`        // bfs, dfs, bestfirst
	ExternalLinks   bool   `json:"external_links,omitempty"` // false = stay in domain
	OnlyText        bool   `json:"only_text,omitempty"`
	WordCountThreshold int `json:"word_count_threshold,omitempty"`
}

// StartCrawlResponse represents the response from starting a crawling job
type StartCrawlResponse struct {
	Success                bool `json:"success"`
	Results                []struct {
		URL             string `json:"url"`
		HTML            string `json:"html"`
		Success         bool   `json:"success"`
		CleanedHTML     string `json:"cleaned_html"`
		Markdown        struct {
			RawMarkdown         string `json:"raw_markdown"`
			MarkdownWithCitations string `json:"markdown_with_citations"`
		} `json:"markdown"`
		Media           struct {
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"media"`
		Metadata        map[string]interface{} `json:"metadata"`
	} `json:"results"`
	ServerProcessingTimeS float64 `json:"server_processing_time_s"`
	ServerMemoryDeltaMB  float64 `json:"server_memory_delta_mb"`
	ServerPeakMemoryMB   float64 `json:"server_peak_memory_mb"`
}

// CrawlResult represents a crawl result for media processing compatibility
type CrawlResult struct {
	Success bool `json:"success"`
	Results []struct {
		URL      string `json:"url"`
		Success  bool   `json:"success"`
		HTML     string `json:"html"`
		Markdown struct {
			RawMarkdown string `json:"raw_markdown"`
		} `json:"markdown"`
		Media struct {
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"media"`
		Metadata map[string]interface{} `json:"metadata,omitempty"`
	} `json:"results"`
}

// MediaFile represents a media file in the crawl result
type MediaFile struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Type     string `json:"type"` // "image", "video", "audio", etc.
	Size     int64  `json:"size,omitempty"`
}

// APIError represents an error response from the API
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Details    string `json:"details,omitempty"`
}

// Error implements the error interface
func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %d - %s", e.StatusCode, e.Message)
}

// StartCrawl starts a new crawling job with the provided URL and options
func (c *Crawler) StartCrawl(ctx context.Context, url string, includeMedia *bool) (*StartCrawlResponse, error) {
	return c.StartCrawlWithConfig(ctx, []string{url}, includeMedia, 2, true, 50)
}

// StartCrawlWithConfig starts a crawling job with custom configuration
func (c *Crawler) StartCrawlWithConfig(ctx context.Context, urls []string, includeMedia *bool, maxDepth int, excludeExternalLinks bool, maxURLs int) (*StartCrawlResponse, error) {
	// Optimize for batch processing: disable internal URL discovery when doing our own discovery
	discoveryEnabled := len(urls) == 1 // Only enable discovery for single URL calls
	
	// Use the format that matches crawl4ai's expected structure
	req := StartCrawlRequest{
		Urls:           urls,   // Use URLs array format as expected by crawl4ai API
		IncludeRawHTML: true,   // Include raw HTML in response
		ProcessURLs:    discoveryEnabled,   // Enable URL processing only for single URLs
		CrawlerConfig: CrawlerConfig{
			MaxDepth:         maxDepth,        // Limit crawling depth
			MaxURLs:          maxURLs,         // Limit total URLs to crawl
			Strategy:         "bfs",           // Use breadth-first search for comprehensive crawling
			ExternalLinks:    false,           // Stay within the same domain
			OnlyText:         true,            // Focus on text content
			WordCountThreshold: 10,           // Skip low-content pages
		},
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Remove trailing slash from server URL if present
	serverURL := strings.TrimSuffix(c.serverURL, "/")
	apiURL := fmt.Sprintf("%s/crawl", serverURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	c.logger.Info("Starting crawl for URLs", map[string]interface{}{
		"urlCount": len(urls),
		"maxDepth": maxDepth,
		"maxURLs": maxURLs,
		"excludeExternal": excludeExternalLinks,
		"discoveryEnabled": discoveryEnabled,
		"isBatch": len(urls) > 1,
		"crawlerConfig": map[string]interface{}{
			"process_urls": discoveryEnabled,
			"strategy": "bfs",
			"external_links": false,
			"only_text": true,
			"word_count_threshold": 10,
		},
	})

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("Request sent", map[string]interface{}{
		"requestBody": string(reqBody),
	})
	
	c.logger.Debug("Start crawl response", map[string]interface{}{
		"statusCode": resp.StatusCode,
		"body":       string(body),
	})

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error response: %w, status code: %d", err, resp.StatusCode)
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, &apiErr
	}

	var result StartCrawlResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.logger.Info("Crawl completed", map[string]interface{}{
		"success": result.Success,
		"resultCount": len(result.Results),
		"processingTime": result.ServerProcessingTimeS,
	})
	
	// If we only got one result but expected more, log a warning
	if len(urls) == 1 && maxURLs > 1 && len(result.Results) == 1 {
		c.logger.Warn("Expected multiple URLs but got only one result. The crawl4ai server may not support multi-URL crawling or different parameters are needed.", map[string]interface{}{
			"requestedURLs": maxURLs,
			"actualResults": len(result.Results),
			"startingURL": urls[0],
		})
	}

	return &result, nil
}

// ExtractURLsFromHTML extracts URLs from HTML content using regex
func (c *Crawler) ExtractURLsFromHTML(html string, baseURL string) ([]string, error) {
	// Simple regex to find href attributes
	hrefRegex := regexp.MustCompile(`<a[^>]+href\s*=\s*["']([^"']+)["'][^>]*>`)
	matches := hrefRegex.FindAllStringSubmatch(html, -1)
	
	var urls []string
	seen := make(map[string]bool)
	
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		
		url := strings.TrimSpace(match[1])
		
		// Skip anchors, javascript, mailto, etc.
		if strings.HasPrefix(url, "#") || strings.HasPrefix(url, "javascript:") || strings.HasPrefix(url, "mailto:") {
			continue
		}
		
		// Make URL absolute
		absoluteURL, err := c.makeAbsoluteURL(url, baseURL)
		if err != nil {
			c.logger.Debug("Failed to make URL absolute", map[string]interface{}{
				"url": url,
				"baseURL": baseURL,
				"error": err,
			})
			continue
		}
		
		// Skip if already seen
		if seen[absoluteURL] {
			continue
		}
		
		seen[absoluteURL] = true
		urls = append(urls, absoluteURL)
	}
	
	c.logger.Info("Extracted URLs from HTML", map[string]interface{}{
		"totalURLs": len(urls),
		"baseURL": baseURL,
	})
	
	return urls, nil
}

// makeAbsoluteURL converts a relative URL to absolute URL
func (c *Crawler) makeAbsoluteURL(url, baseURL string) (string, error) {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url, nil
	}
	
	base, err := neturl.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}
	
	rel, err := neturl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("failed to parse relative URL: %w", err)
	}
	
	return base.ResolveReference(rel).String(), nil
}

// URLWithDepth represents a URL with its crawl depth
type URLWithDepth struct {
	URL   string
	Depth int
}

// StartRecursiveCrawling performs true recursive crawling with depth-based discovery
func (c *Crawler) StartRecursiveCrawling(ctx context.Context, startURL string, includeMedia *bool, maxDepth int, maxURLs int) (*StartCrawlResponse, error) {
	return c.StartBatchRecursiveCrawling(ctx, startURL, includeMedia, maxDepth, maxURLs, 5)
}

// StartBatchRecursiveCrawling performs recursive crawling with batch processing for efficiency
func (c *Crawler) StartBatchRecursiveCrawling(ctx context.Context, startURL string, includeMedia *bool, maxDepth int, maxURLs int, batchSize int) (*StartCrawlResponse, error) {
	c.logger.Info("Starting batch recursive crawling", map[string]interface{}{
		"startURL": startURL,
		"maxDepth": maxDepth,
		"maxURLs": maxURLs,
		"batchSize": batchSize,
	})
	
	// Initialize crawling state
	frontier := []URLWithDepth{{URL: startURL, Depth: 0}}
	visited := make(map[string]bool)
	
	c.logger.Info("Batch recursive crawling initialized", map[string]interface{}{
		"startURL": startURL,
		"maxDepth": maxDepth,
		"maxURLs": maxURLs,
		"batchSize": batchSize,
		"initialFrontierSize": len(frontier),
	})
	var allResults []struct {
		URL             string `json:"url"`
		HTML            string `json:"html"`
		Success         bool   `json:"success"`
		CleanedHTML     string `json:"cleaned_html"`
		Markdown        struct {
			RawMarkdown         string `json:"raw_markdown"`
			MarkdownWithCitations string `json:"markdown_with_citations"`
		} `json:"markdown"`
		Media           struct {
			Images []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"media"`
		Metadata        map[string]interface{} `json:"metadata"`
	}
	
	// Progress reporter will be managed by the caller
	
	for len(frontier) > 0 && len(allResults) < maxURLs {
		// Check context for cancellation
		select {
		case <-ctx.Done():
			c.logger.Warn("Batch crawling cancelled by context", map[string]interface{}{
				"processedURLs": len(allResults),
				"remainingFrontier": len(frontier),
			})
			break
		default:
		}
		
		// Process URLs in batches for efficiency
		batchSizeToProcess := min(batchSize, min(len(frontier), maxURLs-len(allResults)))
		if batchSizeToProcess <= 0 {
			break
		}
		
		// Extract current batch
		var currentBatch []URLWithDepth
		for i := 0; i < batchSizeToProcess; i++ {
			if i >= len(frontier) {
				break
			}
			current := frontier[i]
			
			// Skip if already visited or too deep
			if !visited[current.URL] && current.Depth <= maxDepth {
				currentBatch = append(currentBatch, current)
			}
		}
		
		// Remove processed URLs from frontier
		frontier = frontier[batchSizeToProcess:]
		
		if len(currentBatch) == 0 {
			continue
		}
		
		c.logger.Info("Processing batch", map[string]interface{}{
			"batchSize": len(currentBatch),
			"batchDepth": currentBatch[0].Depth,
			"processedCount": len(allResults),
			"remainingFrontier": len(frontier),
		})
		
		// Extract URLs for batch processing
		var batchURLs []string
		for _, item := range currentBatch {
			batchURLs = append(batchURLs, item.URL)
			visited[item.URL] = true
		}
		
		// Crawl the batch with optimized parameters for batch processing
		result, err := c.StartCrawlWithRetry(ctx, batchURLs, includeMedia, 1, true, len(batchURLs), 1)
		if err != nil {
			c.logger.Warn("Failed to crawl batch", map[string]interface{}{
				"batchSize": len(batchURLs),
				"error": err,
			})
			continue
		}
		
		if len(result.Results) == 0 {
			continue
		}
		
		// Add results and extract new URLs
		var newFrontierItems []URLWithDepth
		for i, crawlResult := range result.Results {
			if i >= len(currentBatch) {
				break // Safety check
			}
			
			// Add to results
			allResults = append(allResults, crawlResult)
			
			// Extract URLs from this page if we haven't reached max depth
			if currentBatch[i].Depth < maxDepth {
				html := crawlResult.HTML
				extractedURLs, err := c.ExtractURLsFromHTML(html, crawlResult.URL)
				if err != nil {
					c.logger.Warn("Failed to extract URLs from page", map[string]interface{}{
						"url": crawlResult.URL,
						"error": err,
					})
					continue
				}
				
				// Filter and add new URLs to frontier
				filteredURLs := c.filterURLsForRecursive(extractedURLs, startURL, visited)
				for _, url := range filteredURLs {
					if len(visited) < maxURLs {
						newFrontierItems = append(newFrontierItems, URLWithDepth{
							URL:   url,
							Depth: currentBatch[i].Depth + 1,
						})
					}
				}
			}
		}
		
		// Add new URLs to frontier
		frontier = append(newFrontierItems, frontier...)
		
		c.logger.Info("Batch completed", map[string]interface{}{
			"batchSize": len(batchURLs),
			"resultsCount": len(result.Results),
			"newURLs": len(newFrontierItems),
			"frontierSize": len(frontier),
			"visitedCount": len(visited),
			"processedCount": len(allResults),
			"maxURLs": maxURLs,
		})
	}
	
	// Log frontier exhaustion
	if len(frontier) == 0 {
		c.logger.Info("Frontier exhausted - batch crawling completed", map[string]interface{}{
			"finalProcessedCount": len(allResults),
			"totalVisited": len(visited),
			"maxURLsReached": len(visited) >= maxURLs,
		})
	}
	
	// Create combined response
	combinedResponse := &StartCrawlResponse{
		Success: len(allResults) > 0,
		Results: allResults,
	}
	
	c.logger.Info("Batch recursive crawling completed", map[string]interface{}{
		"totalResults": len(allResults),
		"visitedURLs": len(visited),
		"startURL": startURL,
		"maxDepth": maxDepth,
		"maxURLs": maxURLs,
		"batchSize": batchSize,
	})
	
	return combinedResponse, nil
}

// filterURLs filters URLs to stay within domain and limits the count
func (c *Crawler) filterURLs(urls []string, baseURL string, maxCount int) []string {
	var filtered []string
	base, err := neturl.Parse(baseURL)
	if err != nil {
		c.logger.Error("Failed to parse base URL for filtering", map[string]interface{}{
			"baseURL": baseURL,
			"error": err,
		})
		return urls[:min(maxCount, len(urls))]
	}
	
	baseDomain := base.Hostname()
	
	for _, url := range urls {
		if len(filtered) >= maxCount {
			break
		}
		
		parsed, err := neturl.Parse(url)
		if err != nil {
			continue
		}
		
		// Stay within the same domain
		if parsed.Hostname() == baseDomain {
			filtered = append(filtered, url)
		}
	}
	
	c.logger.Info("Filtered URLs", map[string]interface{}{
		"originalCount": len(urls),
		"filteredCount": len(filtered),
		"baseDomain": baseDomain,
		"maxCount": maxCount,
	})
	
	return filtered
}

// filterURLsForRecursive filters URLs for recursive crawling, avoiding already visited URLs
func (c *Crawler) filterURLsForRecursive(urls []string, baseURL string, visited map[string]bool) []string {
	var filtered []string
	base, err := neturl.Parse(baseURL)
	if err != nil {
		c.logger.Error("Failed to parse base URL for filtering", map[string]interface{}{
			"baseURL": baseURL,
			"error": err,
		})
		return urls
	}
	
	baseDomain := base.Hostname()
	
	for _, url := range urls {
		// Skip if already visited
		if visited[url] {
			continue
		}
		
		parsed, err := neturl.Parse(url)
		if err != nil {
			continue
		}
		
		// Stay within the same domain
		if parsed.Hostname() == baseDomain {
			filtered = append(filtered, url)
		}
	}
	
	// Sort URLs by priority (high-value discovery pages first)
	filtered = c.prioritizeURLs(filtered)
	
	c.logger.Info("Filtered URLs for recursive crawling", map[string]interface{}{
		"originalCount": len(urls),
		"filteredCount": len(filtered),
		"baseDomain": baseDomain,
		"visitedCount": len(visited),
	})
	
	return filtered
}

// prioritizeURLs sorts URLs based on their likelihood to contain many links
// High-value discovery pages (overviews, indexes, docs) are prioritized
func (c *Crawler) prioritizeURLs(urls []string) []string {
	if len(urls) <= 1 {
		return urls
	}
	
	// Define high-value discovery patterns
	discoveryPatterns := []string{
		"/overview",
		"/docs", 
		"/documentation",
		"/api",
		"/components",
		"/reference",
		"/guides",
		"/examples",
		"/tutorials",
		"/index",
		"/introduction",
		"/getting-started",
	}
	
	// Calculate priority scores
	type URLScore struct {
		URL   string
		Score int
	}
	
	var scoredURLs []URLScore
	for _, url := range urls {
		score := 0
		lowerURL := strings.ToLower(url)
		
		// High priority for discovery patterns
		for _, pattern := range discoveryPatterns {
			if strings.Contains(lowerURL, pattern) {
				score += 10
				break
			}
		}
		
		// Additional scoring based on URL characteristics
		if strings.Contains(lowerURL, "/list") {
			score += 8
		}
		if strings.HasSuffix(lowerURL, "/") {
			score += 3 // Index pages
		}
		if !strings.Contains(lowerURL, "#") {
			score += 2 // Prefer pages without anchors
		}
		
		// Penalize certain patterns
		if strings.Contains(lowerURL, "/demo") ||
		   strings.Contains(lowerURL, "/example") ||
		   strings.Contains(lowerURL, "/playground") {
			score -= 5
		}
		
		scoredURLs = append(scoredURLs, URLScore{URL: url, Score: score})
	}
	
	// Sort by score (descending)
	for i := 0; i < len(scoredURLs)-1; i++ {
		for j := i + 1; j < len(scoredURLs); j++ {
			if scoredURLs[j].Score > scoredURLs[i].Score {
				scoredURLs[i], scoredURLs[j] = scoredURLs[j], scoredURLs[i]
			}
		}
	}
	
	// Extract sorted URLs
	var result []string
	for _, scored := range scoredURLs {
		result = append(result, scored.URL)
	}
	
	c.logger.Debug("URL prioritization completed", map[string]interface{}{
		"urlCount": len(urls),
		"topScore": func() int { if len(scoredURLs) > 0 { return scoredURLs[0].Score } else { return 0 } }(),
		"samplePrioritized": func() []string { 
			if len(result) > 3 { 
				return result[:3] 
			} else { 
				return result 
			} 
		}(),
	})
	
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// StartCrawlWithRetry starts a crawling job with retry logic
func (c *Crawler) StartCrawlWithRetry(ctx context.Context, urls []string, includeMedia *bool, maxDepth int, excludeExternalLinks bool, maxURLs int, maxRetries int) (*StartCrawlResponse, error) {
	var lastErr error
	
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Info("Retrying crawl", map[string]interface{}{
				"attempt": attempt + 1,
				"maxRetries": maxRetries + 1,
				"urlCount": len(urls),
			})
			
			// Add exponential backoff
			backoffDuration := time.Duration(attempt*attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoffDuration):
				// Continue with retry
			}
		}
		
		result, err := c.StartCrawlWithConfig(ctx, urls, includeMedia, maxDepth, excludeExternalLinks, maxURLs)
		if err == nil {
			return result, nil
		}
		
		lastErr = err
		c.logger.Warn("Crawl attempt failed", map[string]interface{}{
			"attempt": attempt + 1,
			"error": err,
			"urlCount": len(urls),
		})
	}
	
	return nil, fmt.Errorf("crawl failed after %d attempts: %w", maxRetries+1, lastErr)
}

// CreateSingleResultResponse creates a StartCrawlResponse for a single result
func (c *Crawler) CreateSingleResultResponse(result interface{}) *StartCrawlResponse {
	return &StartCrawlResponse{
		Success: true,
		Results: []struct {
			URL             string `json:"url"`
			HTML            string `json:"html"`
			Success         bool   `json:"success"`
			CleanedHTML     string `json:"cleaned_html"`
			Markdown        struct {
				RawMarkdown         string `json:"raw_markdown"`
				MarkdownWithCitations string `json:"markdown_with_citations"`
			} `json:"markdown"`
			Media           struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
			Metadata        map[string]interface{} `json:"metadata"`
		}{result.(struct {
			URL             string `json:"url"`
			HTML            string `json:"html"`
			Success         bool   `json:"success"`
			CleanedHTML     string `json:"cleaned_html"`
			Markdown        struct {
				RawMarkdown         string `json:"raw_markdown"`
				MarkdownWithCitations string `json:"markdown_with_citations"`
			} `json:"markdown"`
			Media           struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
			Metadata        map[string]interface{} `json:"metadata"`
		})},
	}
}

// ConvertToCrawlResult converts StartCrawlResponse to CrawlResult for media processing
func (r *StartCrawlResponse) ConvertToCrawlResult() *CrawlResult {
	if len(r.Results) == 0 {
		return &CrawlResult{Success: r.Success, Results: []struct {
			URL      string `json:"url"`
			Success  bool   `json:"success"`
			HTML     string `json:"html"`
			Markdown struct {
				RawMarkdown string `json:"raw_markdown"`
			} `json:"markdown"`
			Media struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}{}}
	}
	
	result := &CrawlResult{
		Success: r.Success,
		Results: make([]struct {
			URL      string `json:"url"`
			Success  bool   `json:"success"`
			HTML     string `json:"html"`
			Markdown struct {
				RawMarkdown string `json:"raw_markdown"`
			} `json:"markdown"`
			Media struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}, len(r.Results)),
	}
	
	for i, res := range r.Results {
		result.Results[i] = struct {
			URL      string `json:"url"`
			Success  bool   `json:"success"`
			HTML     string `json:"html"`
			Markdown struct {
				RawMarkdown string `json:"raw_markdown"`
			} `json:"markdown"`
			Media struct {
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		}{
			URL:     res.URL,
			Success: res.Success,
			HTML:    res.HTML,
			Markdown: struct {
				RawMarkdown string `json:"raw_markdown"`
			}{
				RawMarkdown: res.Markdown.RawMarkdown,
			},
			Media:    res.Media,
			Metadata: res.Metadata,
		}
	}
	
	return result
}

// DownloadAndSaveMediaFromStartResponse downloads and saves media files directly from StartCrawlResponse
func (c *Crawler) DownloadAndSaveMediaFromStartResponse(ctx context.Context, startResp *StartCrawlResponse, progressReporter *progress.ProgressReporter) ([]*storage.FileInfo, error) {
	if !c.includeMedia || len(startResp.Results) == 0 || len(startResp.Results[0].Media.Images) == 0 {
		return nil, nil
	}

	if c.storage == nil {
		return nil, errors.New(errors.StorageError, "storage not initialized")
	}

	var savedFiles []*storage.FileInfo

	for i, mediaFile := range startResp.Results[0].Media.Images {
		select {
		case <-ctx.Done():
			return savedFiles, ctx.Err()
		default:
		}

		// Update progress
		progressReporter.SetCurrent(i)

		// Resolve the media URL
		mediaURL, err := neturl.Parse(mediaFile.URL)
		if err != nil {
			c.logger.Error("Failed to resolve media URL", map[string]interface{}{
				"url":   mediaFile.URL,
				"error": err,
			})
			continue
		}

		// Make the media URL absolute if it's relative
		if !mediaURL.IsAbs() {
			baseURL, err := neturl.Parse(startResp.Results[0].URL)
			if err != nil {
				c.logger.Error("Failed to parse base URL", map[string]interface{}{
					"url":   startResp.Results[0].URL,
					"error": err,
				})
				continue
			}
			mediaURL = baseURL.ResolveReference(mediaURL)
		}

		// Download the media file
		resp, err := c.client.Get(mediaURL.String())
		if err != nil {
			c.logger.Error("Failed to download media file", map[string]interface{}{
				"url":   mediaURL.String(),
				"error": err,
			})
			continue
		}
		defer resp.Body.Close()

		// Check if the response is successful
		if resp.StatusCode != http.StatusOK {
			c.logger.Error("Failed to download media file", map[string]interface{}{
				"url":        mediaURL.String(),
				"statusCode": resp.StatusCode,
			})
			continue
		}

		// Save the media file
		fileInfo, err := c.storage.SaveMediaFile(resp.Body, mediaURL.String(), "")
		if err != nil {
			c.logger.Error("Failed to save media file", map[string]interface{}{
				"url":   mediaURL.String(),
				"error": err,
			})
			continue
		}

		c.logger.Info("Saved media file", map[string]interface{}{
			"path": fileInfo.Path,
			"size": fileInfo.Size,
		})

		savedFiles = append(savedFiles, fileInfo)
	}

	// Mark progress as complete
	progressReporter.SetCurrent(len(startResp.Results[0].Media.Images))

	return savedFiles, nil
}

// DownloadAndSaveMedia downloads and saves media files from the crawl result
func (c *Crawler) DownloadAndSaveMedia(ctx context.Context, result *CrawlResult) ([]*storage.FileInfo, error) {
	if !c.includeMedia || c.storage == nil || len(result.Results) == 0 || len(result.Results[0].Media.Images) == 0 {
		return nil, nil
	}

	var savedFiles []*storage.FileInfo

	for _, mediaFile := range result.Results[0].Media.Images {
		// Parse the media URL to make it absolute if it's relative
		mediaURL, err := c.resolveURL(result.Results[0].Metadata, mediaFile.URL)
		if err != nil {
			c.logger.Error("Failed to resolve media URL", map[string]interface{}{
				"url":   mediaFile.URL,
				"error": err,
			})
			continue
		}

		// Download the media file
		fileData, err := c.downloadFile(ctx, mediaURL)
		if err != nil {
			c.logger.Error("Failed to download media file", map[string]interface{}{
				"url":   mediaURL,
				"error": err,
			})
			continue
		}

		// Save the media file using the storage system
		fileInfo, err := c.storage.SaveMedia(fileData, mediaURL, "")
		if err != nil {
			c.logger.Error("Failed to save media file", map[string]interface{}{
				"url":   mediaURL,
				"error": err,
			})
			continue
		}

		if fileInfo != nil {
			savedFiles = append(savedFiles, fileInfo)
			c.logger.Info("Saved media file", map[string]interface{}{
				"path": fileInfo.Path,
				"size": fileInfo.Size,
			})
		}
	}

	return savedFiles, nil
}

// DownloadAndSaveMediaWithProgress downloads and saves media files with progress reporting
func (c *Crawler) DownloadAndSaveMediaWithProgress(ctx context.Context, result *CrawlResult, progressReporter *progress.ProgressReporter) ([]*storage.FileInfo, error) {
	if !c.includeMedia || len(result.Results) == 0 || len(result.Results[0].Media.Images) == 0 {
		return nil, nil
	}

	if c.storage == nil {
		return nil, errors.New(errors.StorageError, "storage not initialized")
	}

	var savedFiles []*storage.FileInfo

	for i, mediaFile := range result.Results[0].Media.Images {
		select {
		case <-ctx.Done():
			return savedFiles, ctx.Err()
		default:
		}

		// Update progress
		progressReporter.SetCurrent(i)

		// Resolve the media URL
		mediaURL, err := neturl.Parse(mediaFile.URL)
		if err != nil {
			c.logger.Error("Failed to resolve media URL", map[string]interface{}{
				"url":   mediaFile.URL,
				"error": err,
			})
			continue
		}

		// Make the media URL absolute if it's relative
		if !mediaURL.IsAbs() {
			baseURL, err := neturl.Parse(result.Results[0].URL)
			if err != nil {
				c.logger.Error("Failed to parse base URL", map[string]interface{}{
					"url":   result.Results[0].URL,
					"error": err,
				})
				continue
			}
			mediaURL = baseURL.ResolveReference(mediaURL)
		}

		// Download the media file
		resp, err := c.client.Get(mediaURL.String())
		if err != nil {
			c.logger.Error("Failed to download media file", map[string]interface{}{
				"url":   mediaURL.String(),
				"error": err,
			})
			continue
		}
		defer resp.Body.Close()

		// Check if the response is successful
		if resp.StatusCode != http.StatusOK {
			c.logger.Error("Failed to download media file", map[string]interface{}{
				"url":        mediaURL.String(),
				"statusCode": resp.StatusCode,
			})
			continue
		}

		// Save the media file
		fileInfo, err := c.storage.SaveMediaFile(resp.Body, mediaURL.String(), "")
		if err != nil {
			c.logger.Error("Failed to save media file", map[string]interface{}{
				"url":   mediaURL.String(),
				"error": err,
			})
			continue
		}

		c.logger.Info("Saved media file", map[string]interface{}{
			"path": fileInfo.Path,
			"size": fileInfo.Size,
		})

		savedFiles = append(savedFiles, fileInfo)
	}

	// Mark progress as complete
	progressReporter.SetCurrent(len(result.Results[0].Media.Images))

	return savedFiles, nil
}

// resolveURL resolves a potentially relative URL based on the context
func (c *Crawler) resolveURL(metadata map[string]interface{}, mediaURL string) (string, error) {
	// If the URL is already absolute, return it as is
	if strings.HasPrefix(mediaURL, "http://") || strings.HasPrefix(mediaURL, "https://") {
		return mediaURL, nil
	}

	// Try to get the base URL from metadata
	baseURLStr, ok := metadata["base_url"].(string)
	if !ok {
		return "", fmt.Errorf("base URL not found in metadata")
	}

	// Parse the base URL
	baseURL, err := neturl.Parse(baseURLStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Parse the media URL
	mediaURLParsed, err := neturl.Parse(mediaURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse media URL: %w", err)
	}

	// Resolve the media URL against the base URL
	resolvedURL := baseURL.ResolveReference(mediaURLParsed)

	return resolvedURL.String(), nil
}

// downloadFile downloads a file from the given URL
func (c *Crawler) downloadFile(ctx context.Context, fileURL string) (io.Reader, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fileURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers to mimic a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}

	// Check if the response is successful
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
