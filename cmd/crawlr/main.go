package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"crawlr/internal/config"
	"crawlr/internal/crawler"
	"crawlr/internal/errors"
	"crawlr/internal/logger"
	"crawlr/internal/progress"
	"crawlr/internal/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfg       *config.Config
	url       string
	library   string
	output    string
	appLogger *logger.Logger
)

var rootCmd = &cobra.Command{
	Use:   "crawlr",
	Short: "Crawlr is a web crawling tool for extracting and storing content",
	Long: `Crawlr is a powerful web crawling tool that connects to a crawl4ai server
to extract content from websites and store markdown and media files locally.`,
	Example: `crawlr --url https://example.com --library my-library --output ./assets
  crawlr -u https://example.com -l my-library -o ./assets`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create a new viper instance
		v := viper.New()

		// Bind flags to viper
		flagMappings := map[string]string{
			"url":              "url",
			"library":          "library",
			"output":           "output",
			"server-url":       "server_url",
			"timeout":          "timeout",
			"max-concurrent":   "max_concurrent",
			"include-media":    "include_media",
			"overwrite-files":  "overwrite_files",
			"max-depth":        "max_depth",
			"discovery-method": "discovery_method",
			"batch-size":       "batch_size",
			"exclude-patterns": "exclude_patterns",
			"max-urls":         "max_urls",
			"log-level":        "log_level",
			"log-output":       "log_output",
			"log-file-path":    "log_file_path",
			"log-include-time": "log_include_time",
			"log-structured":   "log_structured",
		}
		if err := config.BindFlags(v, cmd, flagMappings); err != nil {
			return errors.Wrap(err, errors.ConfigurationError, "failed to bind flags")
		}

		// Load configuration with the viper instance that has flags bound
		var err error
		cfg, err = config.LoadConfigWithViper(v)
		if err != nil {
			return errors.Wrap(err, errors.ConfigurationError, "failed to load configuration")
		}

		// Override config with flag values if provided
		if cmd.Flags().Changed("url") {
			cfg.URL = url
		}
		if cmd.Flags().Changed("library") {
			cfg.Library = library
		}
		if cmd.Flags().Changed("output") {
			cfg.Output = output
		}

		// Initialize logger
		logLevel := logger.INFO
		switch cfg.LogLevel {
		case "DEBUG":
			logLevel = logger.DEBUG
		case "INFO":
			logLevel = logger.INFO
		case "WARN":
			logLevel = logger.WARN
		case "ERROR":
			logLevel = logger.ERROR
		default:
			return errors.New(errors.ConfigurationError, "invalid log level: "+cfg.LogLevel)
		}

		logOutput := logger.Console
		switch cfg.LogOutput {
		case "console":
			logOutput = logger.Console
		case "file":
			logOutput = logger.File
		case "both":
			logOutput = logger.Both
		default:
			return errors.New(errors.ConfigurationError, "invalid log output: "+cfg.LogOutput)
		}

		loggerConfig := logger.LoggerConfig{
			Level:       logLevel,
			Output:      logOutput,
			FilePath:    cfg.LogFilePath,
			IncludeTime: cfg.LogIncludeTime,
			Structured:  cfg.LogStructured,
		}

		var loggerErr error
		appLogger, loggerErr = logger.NewLogger(loggerConfig)
		if loggerErr != nil {
			return errors.Wrap(loggerErr, errors.ConfigurationError, "failed to initialize logger")
		}
		defer appLogger.Close()

		// Validate required parameters
		if cfg.URL == "" {
			return errors.New(errors.ValidationError, "url is required")
		}
		if cfg.Library == "" {
			return errors.New(errors.ValidationError, "library name is required")
		}
		if cfg.Output == "" {
			return errors.New(errors.ValidationError, "output folder is required")
		}

		appLogger.Info("Starting crawlr application", map[string]interface{}{
			"url":      cfg.URL,
			"library":  cfg.Library,
			"output":   cfg.Output,
			"logLevel": cfg.LogLevel,
		})

		// Initialize the crawler with the configuration
		c := crawler.NewCrawler(cfg, appLogger)

		// Set authentication token if needed (for now, we'll leave it empty)
		// c.SetAuthToken("your-auth-token")

		// Initialize storage system
		storage, err := storage.NewStorage(cfg, appLogger)
		if err != nil {
			return errors.Wrap(err, errors.StorageError, "failed to initialize storage")
		}

		// Set storage for the crawler
		c.SetStorage(storage)

		// Create progress manager
		progressManager := progress.NewProgressManager(appLogger)

		// Start the crawling job
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
		defer cancel()

		appLogger.Info("Starting crawl", map[string]interface{}{
			"url": cfg.URL,
			"maxDepth": cfg.MaxDepth,
			"discoveryMethod": cfg.DiscoveryMethod,
		})

		// Create overall progress reporter with estimated total
		crawlProgress := progressManager.CreateReporter("crawl", "Crawling URLs", cfg.MaxURLs)
		defer crawlProgress.Complete()

		// Use the recursive crawling method for true multi-level crawling with configured batch size
		startResp, err := c.StartBatchRecursiveCrawling(ctx, cfg.URL, nil, cfg.MaxDepth, cfg.MaxURLs, cfg.BatchSize)
		if err != nil {
			return errors.Wrap(err, errors.CrawlerError, "failed to start crawl")
		}

		// Check if the crawl was successful
		if !startResp.Success {
			return errors.New(errors.CrawlerError, "crawl failed")
		}

		if len(startResp.Results) == 0 {
			return errors.New(errors.CrawlerError, "no results returned from crawl")
		}

		// Update progress to show discovered URLs
		crawlProgress.SetTotal(len(startResp.Results))

		// Process all results
		for i, result := range startResp.Results {
			// Update progress
			crawlProgress.SetCurrent(i + 1)
			
			if !result.Success {
				appLogger.Warn("Skipping unsuccessful result", map[string]interface{}{"url": result.URL})
				continue
			}

			appLogger.Info("Processing result", map[string]interface{}{"url": result.URL})

			// Save markdown if available
			if result.Markdown.RawMarkdown != "" {
				markdownPath, err := storage.SaveMarkdown(result.Markdown.RawMarkdown, result.URL)
				if err != nil {
					appLogger.Error("Failed to save markdown", map[string]interface{}{"error": err, "url": result.URL})
				} else {
					appLogger.Info("Saved markdown", map[string]interface{}{"path": markdownPath.Path, "url": result.URL})
				}
			}

			// Save media files if available
			if len(result.Media.Images) > 0 {
				// Create a response wrapper for this specific result
				mediaStartResp := c.CreateSingleResultResponse(result)
				
				mediaProgress := progressManager.CreateReporter("media", fmt.Sprintf("Downloading media for %s", result.URL), len(result.Media.Images))
				defer mediaProgress.Complete()
				
				mediaFiles, err := c.DownloadAndSaveMediaFromStartResponse(ctx, mediaStartResp, mediaProgress)
				if err != nil {
					appLogger.Error("Failed to save media files", map[string]interface{}{"error": err, "url": result.URL})
				} else {
					appLogger.Info("Saved media files", map[string]interface{}{"count": len(mediaFiles), "url": result.URL})
				}
			}
		}

		appLogger.Info("Crawlr application completed successfully")
		return nil
	},
}

func init() {
	// Add flags to the root command
	rootCmd.Flags().StringVarP(&url, "url", "u", "", "The root URL to crawl (required)")
	rootCmd.Flags().StringVarP(&library, "library", "l", "", "The name of the library (required)")
	rootCmd.Flags().StringVarP(&output, "output", "o", "", "The destination folder to store assets (required)")

	// Add configuration flags
	rootCmd.Flags().String("server-url", "http://192.168.1.27:8888/", "Crawl4ai server URL")
	rootCmd.Flags().Int("timeout", 30, "Timeout for HTTP requests in seconds")
	rootCmd.Flags().Int("max-concurrent", 5, "Maximum number of concurrent requests")
	rootCmd.Flags().Bool("include-media", true, "Whether to include media files")
	rootCmd.Flags().Bool("overwrite-files", false, "Whether to overwrite existing files")

	// Add crawling configuration flags
	rootCmd.Flags().Int("max-depth", 2, "Maximum crawling depth")
	rootCmd.Flags().String("discovery-method", "auto", "URL discovery method (auto, sitemap, links)")
	rootCmd.Flags().Int("batch-size", 5, "Number of URLs to process in each batch")
	rootCmd.Flags().String("exclude-patterns", "", "Regex patterns to exclude from crawling")
	rootCmd.Flags().Int("max-urls", 50, "Maximum number of URLs to crawl")

	// Add logging configuration flags
	rootCmd.Flags().String("log-level", "INFO", "Log level (DEBUG, INFO, WARN, ERROR)")
	rootCmd.Flags().String("log-output", "console", "Log output (console, file, both)")
	rootCmd.Flags().String("log-file-path", "crawlr.log", "Path to log file")
	rootCmd.Flags().Bool("log-include-time", true, "Include timestamp in logs")
	rootCmd.Flags().Bool("log-structured", true, "Use structured logging format")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Whoops. There was an error while executing your CLI '%s'", err)
		os.Exit(1)
	}
}
