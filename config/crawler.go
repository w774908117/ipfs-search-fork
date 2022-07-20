package config

import (
	"github.com/ipfs-search/ipfs-search/components/crawler"
	"time"
)

// Crawler contains configuration for a Crawler.
type Crawler struct {
	DirEntryBufferSize uint          `yaml:"direntry_buffer_size"`        // Size of buffer for processing directory entry channels.
	MinUpdateAge       time.Duration `yaml:"min_update_age"`              // The minimum age for items to be updated.
	StatTimeout        time.Duration `yaml:"stat_timeout"`                // Timeout for Stat() calls.
	DirEntryTimeout    time.Duration `yaml:"direntry_timeout"`            // Timeout *between* directory entries.
	MaxDirSize         uint          `yaml:"max_dirsize"`                 // Maximum number of directory entries
	ServerURL          string        `yaml:"server_url" env:"SERVER_URL"` //server URL for contacting interested file type
}

// CrawlerConfig returns component-specific configuration from the canonical central configuration.
func (c *Config) CrawlerConfig() *crawler.Config {
	cfg := crawler.Config(c.Crawler)
	return &cfg
}

// CrawlerDefaults wraps the defaults from the component-specific configuration.
func CrawlerDefaults() Crawler {
	return Crawler(*crawler.DefaultConfig())
}
