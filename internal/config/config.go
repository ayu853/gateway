package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the full gateway configuration.
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Backends []Backend      `yaml:"backends"`
	Balancer BalancerConfig `yaml:"balancer"`
	Health   HealthConfig   `yaml:"health"`
	Rate     RateConfig     `yaml:"rate_limit"`
	Auth     AuthConfig     `yaml:"auth"`
	Cache    CacheConfig    `yaml:"cache"`
	Logging  LogConfig      `yaml:"logging"`
	Metrics  MetricsConfig  `yaml:"metrics"`
}

type ServerConfig struct {
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

type Backend struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

type BalancerConfig struct {
	Algorithm string `yaml:"algorithm"` // round-robin, least-conn, weighted-rr, ip-hash
}

type HealthConfig struct {
	Enabled           bool          `yaml:"enabled"`
	Interval          time.Duration `yaml:"interval"`
	Timeout           time.Duration `yaml:"timeout"`
	Path              string        `yaml:"path"`
	UnhealthyThreshold int          `yaml:"unhealthy_threshold"`
	HealthyThreshold   int          `yaml:"healthy_threshold"`
}

type RateConfig struct {
	Enabled    bool `yaml:"enabled"`
	Requests   int  `yaml:"requests_per_minute"`
	Burst      int  `yaml:"burst"`
}

type AuthConfig struct {
	Enabled   bool     `yaml:"enabled"`
	JWTSecret string   `yaml:"jwt_secret"`
	APIKeys   []string `yaml:"api_keys"`
	ExcludePaths []string `yaml:"exclude_paths"`
}

type CacheConfig struct {
	Enabled  bool          `yaml:"enabled"`
	TTL      time.Duration `yaml:"ttl"`
	MaxSize  int           `yaml:"max_size"`
	RedisURL string        `yaml:"redis_url"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // json, console
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
	Path    string `yaml:"path"`
}

// Load reads and parses a YAML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	cfg.setDefaults()
	return cfg, nil
}

// setDefaults applies default values for missing configuration.
func (c *Config) setDefaults() {
	if c.Server.Port == 0 {
		c.Server.Port = 8080
	}
	if c.Server.ReadTimeout == 0 {
		c.Server.ReadTimeout = 15 * time.Second
	}
	if c.Server.WriteTimeout == 0 {
		c.Server.WriteTimeout = 15 * time.Second
	}
	if c.Server.IdleTimeout == 0 {
		c.Server.IdleTimeout = 60 * time.Second
	}
	if c.Balancer.Algorithm == "" {
		c.Balancer.Algorithm = "round-robin"
	}
	if c.Health.Interval == 0 {
		c.Health.Interval = 10 * time.Second
	}
	if c.Health.Timeout == 0 {
		c.Health.Timeout = 5 * time.Second
	}
	if c.Health.Path == "" {
		c.Health.Path = "/health"
	}
	if c.Health.UnhealthyThreshold == 0 {
		c.Health.UnhealthyThreshold = 3
	}
	if c.Health.HealthyThreshold == 0 {
		c.Health.HealthyThreshold = 2
	}
	if c.Rate.Requests == 0 {
		c.Rate.Requests = 100
	}
	if c.Rate.Burst == 0 {
		c.Rate.Burst = 20
	}
	if c.Cache.TTL == 0 {
		c.Cache.TTL = 5 * time.Minute
	}
	if c.Cache.MaxSize == 0 {
		c.Cache.MaxSize = 1000
	}
	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "console"
	}
	if c.Metrics.Path == "" {
		c.Metrics.Path = "/metrics"
	}
	for i := range c.Backends {
		if c.Backends[i].Weight == 0 {
			c.Backends[i].Weight = 1
		}
	}
}
