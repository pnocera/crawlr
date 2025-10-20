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
```

## Python Test Components

The project includes Python test scripts in the `crawl4aidocs/` directory for testing crawl4ai functionality:

```bash
# Run all Python tests
python crawl4aidocs/tests/run_all_tests.py

# Run individual tests
python crawl4aidocs/tests/test_basic_crawling.py
python crawl4aidocs/tests/test_markdown_generation.py
python crawl4aidocs/tests/test_data_extraction.py
python crawl4aidocs/tests/test_advanced_patterns.py
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

1. Start crawl job via POST to `/api/v1/crawl`
2. Poll job status via GET to `/task/{jobID}`
3. Retrieve results when complete
4. Save markdown content to `{library}/markdown/` directory
5. Download and save media files to `{library}/media/` directory if enabled

### Required Parameters

The CLI requires three main parameters:
- `--url`: Root URL to crawl
- `--library`: Name for organizing the crawled content
- `--output`: Destination folder for storing assets

## Dependencies

- **Cobra**: CLI framework for command structure and flags
- **Viper**: Configuration management (files, env vars, flags)
- Go 1.24+ required