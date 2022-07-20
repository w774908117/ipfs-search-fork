package crawler

import (
	"time"
)

// Config contains configuration for a Crawler.
type Config struct {
	DirEntryBufferSize uint          // Size of buffer for processing directory entry channels.
	MinUpdateAge       time.Duration // The minimum age for items to be updated.
	StatTimeout        time.Duration // Timeout for Stat() calls.
	DirEntryTimeout    time.Duration // Timeout *between* directory entries.
	MaxDirSize         uint          // Maximum number of directory entries
	ServerURL          string        // server URL for contacting interested file type
}

// DefaultConfig generates a default configuration for a Crawler.
func DefaultConfig() *Config {
	return &Config{
		DirEntryBufferSize: 8192,
		MinUpdateAge:       time.Hour,
		StatTimeout:        60 * time.Second,
		DirEntryTimeout:    60 * time.Second,
		MaxDirSize:         32768,
		ServerURL:          "127.0.0.1:9999",
	}
}
