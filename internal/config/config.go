package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Config represents the application configuration
type Config struct {
	ServerURL      string `mapstructure:"server_url"`
	Timeout        int    `mapstructure:"timeout"`
	MaxConcurrent  int    `mapstructure:"max_concurrent"`
	IncludeMedia   bool   `mapstructure:"include_media"`
	OverwriteFiles bool   `mapstructure:"overwrite_files"`
	URL            string `mapstructure:"url"`
	Library        string `mapstructure:"library"`
	Output         string `mapstructure:"output"`

	// Logging configuration
	LogLevel       string `mapstructure:"log_level"`
	LogOutput      string `mapstructure:"log_output"`
	LogFilePath    string `mapstructure:"log_file_path"`
	LogIncludeTime bool   `mapstructure:"log_include_time"`
	LogStructured  bool   `mapstructure:"log_structured"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		ServerURL:      "http://192.168.1.27:8888/",
		Timeout:        30,
		MaxConcurrent:  5,
		IncludeMedia:   true,
		OverwriteFiles: false,
		LogLevel:       "INFO",
		LogOutput:      "console",
		LogFilePath:    "crawlr.log",
		LogIncludeTime: true,
		LogStructured:  true,
	}
}

// LoadConfig loads configuration from multiple sources (file, environment variables, flags)
func LoadConfig() (*Config, error) {
	v := viper.New()

	// Set default values
	config := DefaultConfig()
	v.SetDefault("server_url", config.ServerURL)
	v.SetDefault("timeout", config.Timeout)
	v.SetDefault("max_concurrent", config.MaxConcurrent)
	v.SetDefault("include_media", config.IncludeMedia)
	v.SetDefault("overwrite_files", config.OverwriteFiles)
	v.SetDefault("log_level", config.LogLevel)
	v.SetDefault("log_output", config.LogOutput)
	v.SetDefault("log_file_path", config.LogFilePath)
	v.SetDefault("log_include_time", config.LogIncludeTime)
	v.SetDefault("log_structured", config.LogStructured)

	// Configure viper to read from environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("CRAWLR") // Will look for CRAWLR_SERVER_URL, etc.

	// Configure viper to read from config file
	configDir := "config"
	configName := "config"

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set config file path
	v.SetConfigName(configName)
	v.AddConfigPath(configDir)
	v.AddConfigPath(".") // Also look in the current directory

	// Try to read the config file, but don't fail if it doesn't exist
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, we'll create a default one
		if err := createDefaultConfigFile(configDir, configName); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
	}

	// Unmarshal the configuration
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// LoadConfigWithViper loads configuration using the provided viper instance
func LoadConfigWithViper(v *viper.Viper) (*Config, error) {
	// Set default values if not already set
	config := DefaultConfig()
	v.SetDefault("server_url", config.ServerURL)
	v.SetDefault("timeout", config.Timeout)
	v.SetDefault("max_concurrent", config.MaxConcurrent)
	v.SetDefault("include_media", config.IncludeMedia)
	v.SetDefault("overwrite_files", config.OverwriteFiles)
	v.SetDefault("log_level", config.LogLevel)
	v.SetDefault("log_output", config.LogOutput)
	v.SetDefault("log_file_path", config.LogFilePath)
	v.SetDefault("log_include_time", config.LogIncludeTime)
	v.SetDefault("log_structured", config.LogStructured)

	// Configure viper to read from environment variables
	v.AutomaticEnv()
	v.SetEnvPrefix("CRAWLR") // Will look for CRAWLR_SERVER_URL, etc.

	// Configure viper to read from config file
	configDir := "config"
	configName := "config"

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Set config file path
	v.SetConfigName(configName)
	v.AddConfigPath(configDir)
	v.AddConfigPath(".") // Also look in the current directory

	// Try to read the config file, but don't fail if it doesn't exist
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, we'll create a default one
		if err := createDefaultConfigFile(configDir, configName); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
	}

	// Unmarshal the configuration
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

// createDefaultConfigFile creates a default configuration file
func createDefaultConfigFile(configDir, configName string) error {
	configPath := filepath.Join(configDir, configName+".yaml")

	// Check if config file already exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File already exists, no need to create
	}

	// Create default config file
	defaultConfig := DefaultConfig()

	v := viper.New()
	v.Set("server_url", defaultConfig.ServerURL)
	v.Set("timeout", defaultConfig.Timeout)
	v.Set("max_concurrent", defaultConfig.MaxConcurrent)
	v.Set("include_media", defaultConfig.IncludeMedia)
	v.Set("overwrite_files", defaultConfig.OverwriteFiles)
	v.Set("log_level", defaultConfig.LogLevel)
	v.Set("log_output", defaultConfig.LogOutput)
	v.Set("log_file_path", defaultConfig.LogFilePath)
	v.Set("log_include_time", defaultConfig.LogIncludeTime)
	v.Set("log_structured", defaultConfig.LogStructured)

	// Write the config file
	if err := v.WriteConfigAs(configPath); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// BindFlags binds Cobra flags to Viper configuration
func BindFlags(v *viper.Viper, cmd *cobra.Command, flagMappings map[string]string) error {
	for flagName, configKey := range flagMappings {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			return fmt.Errorf("flag %s not found", flagName)
		}
		if err := v.BindPFlag(configKey, flag); err != nil {
			return fmt.Errorf("failed to bind flag %s to config key %s: %w", flagName, configKey, err)
		}
	}
	return nil
}
