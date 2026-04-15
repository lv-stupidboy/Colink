// internal/reporter/config.go
package reporter

import "time"

// Config holds reporter configuration.
type Config struct {
	Enabled       bool          // Whether reporter is enabled
	Endpoint      string        // Reporting endpoint URL
	Interval      time.Duration // Reporting interval
	RetryTimes    int           // Number of retries on failure
	RetryInterval time.Duration // Interval between retries
}

// DefaultConfig returns Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:       false,
		Endpoint:      "",
		Interval:      30 * time.Minute,
		RetryTimes:    3,
		RetryInterval: 1 * time.Minute,
	}
}

// IsRunnable returns true if reporter should be started.
func (c Config) IsRunnable() bool {
	return c.Enabled && c.Endpoint != ""
}