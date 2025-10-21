# Repository Guidelines

## Project Structure & Module Organization

```
crawlr/
├─ cmd/crawlr/          # CLI entry point (main.go)
├─ internal/            # Private application code
│   ├─ config/         # Configuration management
│   ├─ crawler/        # Core crawling logic
│   ├─ errors/         # Error handling utilities
│   ├─ logger/         # Logging functionality
│   ├─ progress/       # Progress tracking
│   └─ storage/        # File storage management
├─ pkg/                 # Reusable public packages
├─ config/             # Configuration files (config.yaml)
├─ assets/             # Generated output directory (gitignored)
└─ libraries/          # Crawled content storage (gitignored)
```

## Build, Test, and Development Commands

```bash
# Build the CLI binary
go build ./cmd/crawlr

# Run directly without building
go run ./cmd/crawlr -u <URL> -l <library> -o <output>

# Test the application
go test ./...

# Format code
go fmt ./...
```

## Coding Style & Naming Conventions

- **Language**: Go 1.24+
- **Indentation**: Tabs (Go standard)
- **Package naming**: lowercase, single words where possible
- **File naming**: snake_case for configuration files, PascalCase for Go files
- **Import grouping**: Standard Go format with blank lines between groups
- **No linter configured**: Follow Go standard conventions and `go fmt`

## Testing Guidelines

- **Framework**: Go's built-in testing package
- **Test files**: `*_test.go` alongside source files
- **Run tests**: `go test ./...`
- **Coverage**: Use `go test -cover ./...` for coverage reports
- **Naming**: Test functions should start with `Test` and describe the functionality

## Commit & Pull Request Guidelines

- **Format**: Conventional commits with concise messages
- **Current pattern**: Short descriptive messages (e.g., "otw", "first commit")
- **PR requirements**: Include clear description of changes and testing performed
- **Branch naming**: Use descriptive names for feature branches

## Configuration

- **Primary config**: `config/config.yaml`
- **Environment variables**: Prefix with `CRAWLR_`
- **CLI flags**: Override config file and environment variables
- **Default server**: `http://192.168.1.27:8888/`

## Development Notes

- The project uses Cobra for CLI framework and Viper for configuration
- Output directories (`assets/`, `libraries/`) are gitignored
- Log files and test output are excluded from version control
- The CLI connects to a crawl4ai server for web crawling operations
