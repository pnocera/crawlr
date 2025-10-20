package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	Urls                 []string `json:"urls"`                     // URLs array as expected by crawl4ai API
	IncludeRawHTML       bool     `json:"include_raw_html,omitempty"`
	WordCountThreshold   int      `json:"word_count_threshold,omitempty"`
	Priority             int      `json:"priority,omitempty"`
	TTL                  int      `json:"ttl,omitempty"`
	// Additional options can be added here as needed
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

// JobStatus represents the status of a crawling job
type JobStatus struct {
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
	} `json:"results"`
}

// CrawlResult represents the result of a completed crawling job
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
	req := StartCrawlRequest{
		Urls:               []string{url}, // Use URLs array format as expected by crawl4ai API
		IncludeRawHTML:     true,           // Include raw HTML in response
		WordCountThreshold: 10,             // Set minimum word count threshold
		Priority:           10,             // Set a default priority as per the documentation
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := fmt.Sprintf("%s/crawl", c.serverURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	c.logger.Info("Starting crawl for URL", map[string]interface{}{"url": url})

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

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

	return &result, nil
}

// GetJobStatus retrieves the status of a crawling job
func (c *Crawler) GetJobStatus(ctx context.Context, taskID string) (*JobStatus, error) {
	apiURL := fmt.Sprintf("%s/crawl/job/%s", c.serverURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	c.logger.Info("Checking job status", map[string]interface{}{"taskID": taskID})

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("Job status response", map[string]interface{}{
		"statusCode": resp.StatusCode,
		"body":       string(body),
	})

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error response: %w, status code: %d", err, resp.StatusCode)
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, &apiErr
	}

	var status JobStatus
	if err := json.Unmarshal(body, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &status, nil
}

// GetCrawlResult retrieves the results of a completed crawling job
func (c *Crawler) GetCrawlResult(ctx context.Context, taskID string) (*CrawlResult, error) {
	apiURL := fmt.Sprintf("%s/crawl/job/%s", c.serverURL, taskID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.authToken)
	}

	c.logger.Info("Retrieving crawl result", map[string]interface{}{"taskID": taskID})

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	c.logger.Debug("Crawl result response", map[string]interface{}{
		"statusCode": resp.StatusCode,
		"bodySize":   len(body),
	})

	if resp.StatusCode != http.StatusOK {
		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("failed to unmarshal error response: %w, status code: %d", err, resp.StatusCode)
		}
		apiErr.StatusCode = resp.StatusCode
		return nil, &apiErr
	}

	var result CrawlResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &result, nil
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
		mediaURL, err := url.Parse(mediaFile.URL)
		if err != nil {
			c.logger.Error("Failed to resolve media URL", map[string]interface{}{
				"url":   mediaFile.URL,
				"error": err,
			})
			continue
		}

		// Make the media URL absolute if it's relative
		if !mediaURL.IsAbs() {
			baseURL, err := url.Parse(result.Results[0].URL)
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
	baseURL, err := url.Parse(baseURLStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse base URL: %w", err)
	}

	// Parse the media URL
	mediaURLParsed, err := url.Parse(mediaURL)
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
