# Crawlr

A CLI tool for interacting with a crawl4ai server to crawl websites and store content locally.

## Project Structure

```
crawlr/
├── cmd/           # Main CLI command
│   └── crawlr/    # Entry point for the CLI application
├── internal/      # Private application code
├── pkg/           # Code that might be reusable
├── config/        # Configuration files
├── go.mod         # Go module file
├── go.sum         # Go module checksums (generated)
├── .gitignore     # Git ignore rules
└── README.md      # This file
```

## Features

- Connect to a crawl4ai server
- Crawl URLs and store markdown and media files locally
- Organize crawled content into library structures
- Configurable logging with multiple output options
- Progress tracking during crawl operations
- Automatic media file downloads
- Configurable settings via files, environment variables, or CLI flags

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - A powerful CLI application framework
- [Viper](https://github.com/spf13/viper) - A configuration solution for Go applications

## Getting Started

## Getting Started

### Prerequisites

- Go 1.24 or higher
- Running crawl4ai server (default: http://192.168.1.27:8888/)

### Installation

To build the project:

```bash
go build ./cmd/crawlr
```

To run the project directly:

```bash
go run ./cmd/crawlr -u <URL> -l <library> -o <output>
```

## Usage

### Basic Usage

```bash
# Crawl a single URL
go run ./cmd/crawlr --url https://example.com --library my-library --output ./assets

# Using short flags
go run ./cmd/crawlr -u https://example.com -l my-library -o ./assets

# Using built binary
./crawlr --url https://example.com --library my-library --output ./assets
```

### Required Parameters

- `--url, -u`: The root URL to crawl
- `--library, -l`: The name of the library for organizing content
- `--output, -o`: The destination folder to store assets

### Optional Configuration

```bash
# Specify custom server URL
--server-url http://localhost:8888/

# Adjust timeout (default: 30 seconds)
--timeout 60

# Control concurrent requests
--max-concurrent 3

# Disable media downloads
--include-media false

# Overwrite existing files
--overwrite-files true

# Logging configuration
--log-level DEBUG
--log-output file
--log-file-path crawler.log
```

### Environment Variables

You can use environment variables with `CRAWLR_` prefix:

```bash
export CRAWLR_SERVER_URL=http://192.168.1.27:8888/
export CRAWLR_TIMEOUT=60
export CRAWLR_LOG_LEVEL=DEBUG
go run ./cmd/crawlr -u https://example.com -l my-library -o ./assets
```

### Configuration File

Create `config/config.yaml`:

```yaml
server_url: http://192.168.1.27:8888/
timeout: 30
max_concurrent: 5
include_media: true
overwrite_files: false
log_level: INFO
log_output: console
```

## Output Structure

Crawled content is organized as follows:

```
output/
└── library-name/
    ├── markdown/
    │   ├── index.md
    │   └── html.md
    └── media/
        ├── images/
        └── videos/
```
