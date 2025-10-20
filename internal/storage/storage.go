package storage

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"crawlr/internal/config"
	"crawlr/internal/errors"
	"crawlr/internal/logger"
)

// Storage handles file operations for crawled content
type Storage struct {
	config         *config.Config
	logger         *logger.Logger
	basePath       string
	libraryPath    string
	markdownPath   string
	mediaPath      string
	sanitizeRegexp *regexp.Regexp
}

// FileInfo represents information about a stored file
type FileInfo struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Type     string `json:"type"` // "markdown", "image", "video", etc.
	URL      string `json:"url,omitempty"`
}

// NewStorage creates a new Storage instance with the provided configuration
func NewStorage(cfg *config.Config, logger *logger.Logger) (*Storage, error) {
	// Create a regular expression for sanitizing filenames
	sanitizeRegexp, err := regexp.Compile(`[<>:"/\\|?*\x00-\x1F]`)
	if err != nil {
		return nil, fmt.Errorf("failed to compile sanitize regexp: %w", err)
	}

	storage := &Storage{
		config:         cfg,
		logger:         logger,
		sanitizeRegexp: sanitizeRegexp,
	}

	// Initialize directory structure
	if err := storage.initializePaths(); err != nil {
		return nil, fmt.Errorf("failed to initialize paths: %w", err)
	}

	return storage, nil
}

// initializePaths sets up the directory structure for storing crawled content
func (s *Storage) initializePaths() error {
	// Set base path from configuration
	s.basePath = s.config.Output

	// Create library path
	s.libraryPath = filepath.Join(s.basePath, s.sanitizeFilename(s.config.Library))

	// Create content type paths
	s.markdownPath = filepath.Join(s.libraryPath, "markdown")
	s.mediaPath = filepath.Join(s.libraryPath, "media")

	// Create all directories
	if err := s.ensureDir(s.basePath); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	if err := s.ensureDir(s.libraryPath); err != nil {
		return fmt.Errorf("failed to create library directory: %w", err)
	}

	if err := s.ensureDir(s.markdownPath); err != nil {
		return fmt.Errorf("failed to create markdown directory: %w", err)
	}

	if s.config.IncludeMedia {
		if err := s.ensureDir(s.mediaPath); err != nil {
			return fmt.Errorf("failed to create media directory: %w", err)
		}
	}

	return nil
}

// ensureDir creates a directory if it doesn't exist
func (s *Storage) ensureDir(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		s.logger.Info("Creating directory", map[string]interface{}{"path": path})
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}
	return nil
}

// sanitizeFilename replaces special characters in filenames with underscores
func (s *Storage) sanitizeFilename(filename string) string {
	return s.sanitizeRegexp.ReplaceAllString(filename, "_")
}

// GetMarkdownPath returns the path for storing markdown content for a given URL
func (s *Storage) GetMarkdownPath(pageURL string) string {
	// Parse URL to extract path
	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		s.logger.Error("Failed to parse URL", map[string]interface{}{
			"url":   pageURL,
			"error": err,
		})
		return filepath.Join(s.markdownPath, "index.md")
	}

	// Get path without leading slash
	path := strings.TrimPrefix(parsedURL.Path, "/")

	// If path is empty, use index.md
	if path == "" {
		return filepath.Join(s.markdownPath, "index.md")
	}

	// Sanitize path components
	pathComponents := strings.Split(path, "/")
	for i, component := range pathComponents {
		pathComponents[i] = s.sanitizeFilename(component)
	}

	// Join path components and add .md extension
	sanitizedPath := filepath.Join(pathComponents...)
	if !strings.HasSuffix(sanitizedPath, ".md") {
		sanitizedPath += ".md"
	}

	return filepath.Join(s.markdownPath, sanitizedPath)
}

// GetMediaPath returns the path for storing a media file
func (s *Storage) GetMediaPath(mediaURL string, filename string) string {
	// Parse URL to extract path
	parsedURL, err := url.Parse(mediaURL)
	if err != nil {
		s.logger.Error("Failed to parse media URL", map[string]interface{}{
			"url":   mediaURL,
			"error": err,
		})
		return filepath.Join(s.mediaPath, s.sanitizeFilename(filename))
	}

	// Get path without leading slash
	path := strings.TrimPrefix(parsedURL.Path, "/")

	// If path is empty, use the filename
	if path == "" {
		return filepath.Join(s.mediaPath, s.sanitizeFilename(filename))
	}

	// Sanitize path components
	pathComponents := strings.Split(path, "/")
	for i, component := range pathComponents {
		pathComponents[i] = s.sanitizeFilename(component)
	}

	// Join path components
	sanitizedPath := filepath.Join(pathComponents...)

	return filepath.Join(s.mediaPath, sanitizedPath)
}

// SaveMarkdown saves markdown content to a file
func (s *Storage) SaveMarkdown(content string, pageURL string) (*FileInfo, error) {
	path := s.GetMarkdownPath(pageURL)

	// Check if file exists and handle overwrite logic
	if !s.config.OverwriteFiles {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("file already exists and overwrite is disabled: %s", path)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := s.ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create directory for markdown file: %w", err)
	}

	// Write content to file
	s.logger.Info("Saving markdown content", map[string]interface{}{"path": path})
	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write markdown file: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &FileInfo{
		Path:     path,
		Filename: filepath.Base(path),
		Size:     fileInfo.Size(),
		Type:     "markdown",
		URL:      pageURL,
	}, nil
}

// SaveMedia saves a media file from a reader
func (s *Storage) SaveMedia(reader io.Reader, mediaURL string, filename string) (*FileInfo, error) {
	if !s.config.IncludeMedia {
		return nil, nil // Skip media files if not configured to include them
	}

	path := s.GetMediaPath(mediaURL, filename)

	// Check if file exists and handle overwrite logic
	if !s.config.OverwriteFiles {
		if _, err := os.Stat(path); err == nil {
			return nil, fmt.Errorf("file already exists and overwrite is disabled: %s", path)
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := s.ensureDir(dir); err != nil {
		return nil, fmt.Errorf("failed to create directory for media file: %w", err)
	}

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("failed to create media file: %w", err)
	}
	defer file.Close()

	// Copy content from reader to file
	s.logger.Info("Saving media file", map[string]interface{}{"path": path})
	size, err := io.Copy(file, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to write media file: %w", err)
	}

	// Determine file type based on extension
	ext := strings.ToLower(filepath.Ext(filename))
	fileType := "other"
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		fileType = "image"
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm":
		fileType = "video"
	case ".mp3", ".wav", ".ogg", ".flac", ".aac":
		fileType = "audio"
	}

	return &FileInfo{
		Path:     path,
		Filename: filepath.Base(path),
		Size:     size,
		Type:     fileType,
		URL:      mediaURL,
	}, nil
}

// SaveMediaFile saves a media file from a reader with a specific filename
func (s *Storage) SaveMediaFile(reader io.Reader, mediaURL string, filename string) (*FileInfo, error) {
	if !s.config.IncludeMedia {
		return nil, nil // Skip media files if not configured to include them
	}

	path := s.GetMediaPath(mediaURL, filename)

	// Check if file exists and handle overwrite logic
	if !s.config.OverwriteFiles {
		if _, err := os.Stat(path); err == nil {
			return nil, errors.New(errors.StorageError, fmt.Sprintf("file already exists and overwrite is disabled: %s", path))
		}
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := s.ensureDir(dir); err != nil {
		return nil, errors.Wrap(err, errors.StorageError, "failed to create directory for media file")
	}

	// Create file
	file, err := os.Create(path)
	if err != nil {
		return nil, errors.Wrap(err, errors.StorageError, "failed to create media file")
	}
	defer file.Close()

	// Copy content from reader to file
	s.logger.Info("Saving media file", map[string]interface{}{"path": path})
	size, err := io.Copy(file, reader)
	if err != nil {
		return nil, errors.Wrap(err, errors.StorageError, "failed to write media file")
	}

	// Determine file type based on extension
	ext := strings.ToLower(filepath.Ext(filename))
	fileType := "other"
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp":
		fileType = "image"
	case ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm":
		fileType = "video"
	case ".mp3", ".wav", ".ogg", ".flac", ".aac":
		fileType = "audio"
	}

	return &FileInfo{
		Path:     path,
		Filename: filepath.Base(path),
		Size:     size,
		Type:     fileType,
		URL:      mediaURL,
	}, nil
}
