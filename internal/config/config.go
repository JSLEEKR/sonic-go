// Package config provides configuration types for the sonic-go server.
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level server configuration.
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Store   StoreConfig   `yaml:"store"`
	Channel ChannelConfig `yaml:"channel"`
}

// ServerConfig controls server behavior.
type ServerConfig struct {
	LogLevel string `yaml:"log_level"`
}

// StoreConfig controls the storage backend.
type StoreConfig struct {
	DataDir            string        `yaml:"data_dir"`
	FlushInterval      time.Duration `yaml:"flush_interval"`
	RetainWordObjects  int           `yaml:"retain_word_objects"`
	ConsolidateInterval time.Duration `yaml:"consolidate_interval"`
}

// ChannelConfig controls the TCP channel server.
type ChannelConfig struct {
	ListenAddr     string `yaml:"listen_addr"`
	AuthPassword   string `yaml:"auth_password"`
	MaxBufferSize  int    `yaml:"max_buffer_size"`
	SearchPoolSize int    `yaml:"search_pool_size"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			LogLevel: "info",
		},
		Store: StoreConfig{
			DataDir:            "./sonic-data",
			FlushInterval:      5 * time.Minute,
			RetainWordObjects:  1000,
			ConsolidateInterval: 30 * time.Second,
		},
		Channel: ChannelConfig{
			ListenAddr:     ":1491",
			AuthPassword:   "SecretPassword",
			MaxBufferSize:  20000,
			SearchPoolSize: 4,
		},
	}
}

// LoadFromFile reads configuration from a YAML file, merging with defaults.
func LoadFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config YAML: %w", err)
	}

	return cfg, nil
}

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if c.Store.DataDir == "" {
		return fmt.Errorf("store.data_dir must not be empty")
	}
	if c.Channel.ListenAddr == "" {
		return fmt.Errorf("channel.listen_addr must not be empty")
	}
	if c.Store.RetainWordObjects < 1 {
		return fmt.Errorf("store.retain_word_objects must be >= 1")
	}
	if c.Channel.MaxBufferSize < 100 {
		return fmt.Errorf("channel.max_buffer_size must be >= 100")
	}
	return nil
}
