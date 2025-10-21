# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Crawlr is a CLI tool for interacting with a crawl4ai server to crawl websites and store content locally. It's built in Go and uses Cobra for CLI management and Viper for configuration.

## Build and Run Commands

```bash
# Build the project
go build ./cmd/crawlr

# Run the project directly
go run ./cmd/crawlr

# Run with flags
go run ./cmd/crawlr --url https://example.com --library my-library --output ./assets

# Short form
go run ./cmd/crawlr -u https://example.com -l my-library -o ./assets

# Using built binary
./crawlr --url https://example.com --library my-library --output ./assets

# With crawling configuration
go run ./cmd/crawlr -u https://docs.example.com -l my-docs -o ./assets --max-depth 3 --discovery-method auto

# Advanced example for documentation crawling
go run ./cmd/crawlr -u https://ng.ant.design -l ng-zorro-docs -o ./assets --max-depth 2 --exclude-patterns "blog|news|changelog"
```

## Project Structure

```
crawlr/
├── cmd/crawlr/           # Main CLI command entry point
├── internal/             # Private application code
│   ├── config/          # Configuration management
│   ├── crawler/         # HTTP client for crawl4ai API
│   ├── storage/         # File system storage for markdown/media
│   ├── logger/          # Structured logging
│   ├── progress/        # Progress reporting
│   └── errors/          # Custom error types
├── config/              # Configuration files (config.yaml)
├── libraries/           # Example crawled content storage
├── pkg/                 # Reusable code packages (currently empty)
├── go.mod               # Go module file
├── go.sum               # Go module checksums
├── CLAUDE.md            # This file
└── README.md            # Project documentation
```

## Architecture

### Core Components

- **cmd/crawlr/main.go**: Entry point with Cobra CLI setup and command execution
- **internal/config/**: Configuration management using Viper with support for YAML files, environment variables (CRAWLR_ prefix), and CLI flags
- **internal/crawler/**: HTTP client for communicating with crawl4ai API
- **internal/storage/**: File system storage for markdown and media files
- **internal/logger/**: Structured logging with configurable output (console/file/both)
- **internal/progress/**: Progress reporting for long-running operations
- **internal/errors/**: Custom error types with wrapping

### Configuration

Configuration is handled in layers (default → config file → environment variables → CLI flags):
- Default config file location: `config/config.yaml`
- Environment variables use `CRAWLR_` prefix (e.g., `CRAWLR_SERVER_URL`)
- Server URL defaults to `http://192.168.1.27:8888/`

### Crawler Workflow

1. Start crawl job via POST to `/crawl`
2. Get immediate response with results
3. Save markdown content to `{library}/markdown/` directory
4. Download and save media files to `{library}/media/` directory if enabled

### Required Parameters

The CLI requires three main parameters:
- `--url, -u`: Root URL to crawl (required)
- `--library, -l`: Name for organizing the crawled content (required)
- `--output, -o`: Destination folder for storing assets (required)

### Optional Configuration Parameters

- `--server-url`: Crawl4ai server URL (default: http://192.168.1.27:8888/)
- `--timeout`: HTTP request timeout in seconds (default: 30)
- `--max-concurrent`: Maximum concurrent requests (default: 5)
- `--include-media`: Whether to download media files (default: true)
- `--overwrite-files`: Whether to overwrite existing files (default: false)

### Crawling Configuration Parameters

- `--max-depth`: Maximum crawling depth (default: 2)
- `--discovery-method`: URL discovery method - auto, sitemap, or links (default: auto)
- `--batch-size`: Number of URLs to process in each batch (default: 5)
- `--exclude-patterns`: Regex patterns to exclude from crawling (default: empty)

### Logging Configuration

- `--log-level`: Log level (DEBUG, INFO, WARN, ERROR) (default: INFO)
- `--log-output`: Log output (console, file, both) (default: console)
- `--log-file-path`: Path to log file (default: crawlr.log)
- `--log-include-time`: Include timestamp in logs (default: true)
- `--log-structured`: Use structured logging format (default: true)

## Output Structure

Crawled content is organized as follows:

```
output/
└── library-name/
    ├── markdown/
    │   └── [markdown files]
    └── media/
        └── [images and other media]
```

## Dependencies

- **Cobra**: CLI framework for command structure and flags
- **Viper**: Configuration management (files, env vars, flags)
- Go 1.24+ required

## Environment Variables

Configuration can be set via environment variables with `CRAWLR_` prefix:
- `CRAWLR_SERVER_URL`: Crawl4ai server URL
- `CRAWLR_TIMEOUT`: Request timeout
- `CRAWLR_LOG_LEVEL`: Logging level
- `CRAWLR_LOG_OUTPUT`: Logging output destination
- `CRAWLR_MAX_DEPTH`: Maximum crawling depth
- `CRAWLR_DISCOVERY_METHOD`: URL discovery method
- `CRAWLR_BATCH_SIZE`: Number of URLs per batch
- `CRAWLR_EXCLUDE_PATTERNS`: Regex patterns to exclude

## Key Features

### Multi-URL Crawling
- Automatically discovers and crawls multiple URLs within the same domain
- Leverages crawl4ai's built-in URL extraction and domain filtering
- Processes URLs in configurable batches for efficiency

### Smart Content Organization
- Maintains URL path hierarchy in file storage
- Example: `/docs/introduce/en` → `docs/introduce/en.md`
- Handles filename conflicts intelligently

### Error Handling & Retry Logic
- Automatic retry with exponential backoff (max 3 attempts)
- Continues processing when individual URLs fail
- Comprehensive error logging and reporting

### Progress Reporting
- Real-time progress tracking for crawling operations
- Shows discovered URLs and processing status
- Separate progress for media downloads

## Usage Examples

### Basic Documentation Crawling
```bash
go run ./cmd/crawlr \
  -u https://docs.example.com \
  -l my-docs \
  -o ./assets \
  --max-depth 2
```

### Advanced Crawling with Filtering
```bash
go run ./cmd/crawlr \
  -u https://docs.example.com \
  -l api-docs \
  -o ./assets \
  --max-depth 3 \
  --discovery-method auto \
  --exclude-patterns "blog|news|changelog|examples" \
  --include-media true
```

The tool automatically uses crawl4ai's powerful features for:
- URL extraction and domain filtering (`exclude_external_links: true`)
- Content filtering (`word_count_threshold: 10`)
- Efficient caching and processing