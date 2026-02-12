// Package config provides unified TOML configuration for smtptunnel.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the top-level configuration.
type Config struct {
	Server  ServerConfig  `toml:"server"`
	Client  ClientConfig  `toml:"client"`
	Stealth StealthConfig `toml:"stealth"`
}

// ServerConfig holds server-side settings.
type ServerConfig struct {
	Listen   string      `toml:"listen"`
	Hostname string      `toml:"hostname"`
	CertFile string      `toml:"cert_file"`
	KeyFile  string      `toml:"key_file"`
	LogLevel string      `toml:"log_level"`
	TLS      TLSConfig   `toml:"tls"`
	Users    []UserEntry `toml:"users"`
}

// TLSConfig holds TLS-specific settings.
type TLSConfig struct {
	MinVersion string `toml:"min_version"`
}

// UserEntry defines a single user in the config.
type UserEntry struct {
	Username  string   `toml:"username"`
	Secret    string   `toml:"secret"`
	Whitelist []string `toml:"whitelist"`
	Logging   bool     `toml:"logging"`
}

// ClientConfig holds client-side settings.
type ClientConfig struct {
	Server             string        `toml:"server"`
	Username           string        `toml:"username"`
	Secret             string        `toml:"secret"`
	CACert             string        `toml:"ca_cert"`
	InsecureSkipVerify bool          `toml:"insecure_skip_verify"`
	ReconnectDelay     Duration      `toml:"reconnect_delay"`
	MaxReconnectDelay  Duration      `toml:"max_reconnect_delay"`
	Socks              []SocksEntry  `toml:"socks"`
}

// SocksEntry defines a single SOCKS5 listener.
type SocksEntry struct {
	Listen   string `toml:"listen"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// StealthConfig controls DPI evasion features.
type StealthConfig struct {
	Enabled          bool    `toml:"enabled"`
	MinDelayMs       int     `toml:"min_delay_ms"`
	MaxDelayMs       int     `toml:"max_delay_ms"`
	PaddingSizes     []int   `toml:"padding_sizes"`
	DummyProbability float64 `toml:"dummy_probability"`
}

// Duration wraps time.Duration for TOML string parsing.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Duration.String()), nil
}

// Load reads and parses a TOML configuration file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	// Set defaults
	cfg.Server.Listen = "0.0.0.0:587"
	cfg.Server.Hostname = "mail.example.com"
	cfg.Server.LogLevel = "info"
	cfg.Server.TLS.MinVersion = "1.2"

	cfg.Client.ReconnectDelay = Duration{2 * time.Second}
	cfg.Client.MaxReconnectDelay = Duration{30 * time.Second}

	cfg.Stealth.MinDelayMs = 50
	cfg.Stealth.MaxDelayMs = 500
	cfg.Stealth.PaddingSizes = []int{4096, 8192, 16384, 32768}
	cfg.Stealth.DummyProbability = 0.1

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	// Set default Logging=true for users if not explicitly set
	for i := range cfg.Server.Users {
		if cfg.Server.Users[i].Secret == "" {
			return nil, fmt.Errorf("user %q has no secret", cfg.Server.Users[i].Username)
		}
	}

	return cfg, nil
}

// Validate checks the config for obvious errors.
func (c *Config) Validate(mode string) error {
	switch mode {
	case "server":
		if c.Server.Listen == "" {
			return fmt.Errorf("server.listen is required")
		}
		if c.Server.CertFile == "" {
			return fmt.Errorf("server.cert_file is required")
		}
		if c.Server.KeyFile == "" {
			return fmt.Errorf("server.key_file is required")
		}
		if len(c.Server.Users) == 0 {
			return fmt.Errorf("at least one [[server.users]] entry is required")
		}
	case "client":
		if c.Client.Server == "" {
			return fmt.Errorf("client.server is required")
		}
		if c.Client.Username == "" {
			return fmt.Errorf("client.username is required")
		}
		if c.Client.Secret == "" {
			return fmt.Errorf("client.secret is required")
		}
		if len(c.Client.Socks) == 0 {
			return fmt.Errorf("at least one [[client.socks]] entry is required")
		}
	}
	return nil
}

// FindUser looks up a user by username.
func (c *Config) FindUser(username string) *UserEntry {
	for i := range c.Server.Users {
		if c.Server.Users[i].Username == username {
			return &c.Server.Users[i]
		}
	}
	return nil
}

// WriteDefault writes a default config file to the given path.
func WriteDefault(path string) error {
	content := `# SMTP Tunnel Configuration (unified)
# All settings for server, client, stealth, and users in one file.

[server]
listen = "0.0.0.0:587"
hostname = "mail.example.com"
cert_file = "server.crt"
key_file = "server.key"
log_level = "info"

[server.tls]
min_version = "1.2"

# Add users with: smtptunnel-server adduser <name> -c config.toml
# [[server.users]]
# username = "alice"
# secret = "auto-generated-secret"
# whitelist = ["0.0.0.0/0"]
# logging = true

[client]
server = "mail.example.com:587"
username = ""
secret = ""
ca_cert = "ca.crt"
insecure_skip_verify = false
reconnect_delay = "2s"
max_reconnect_delay = "30s"

[[client.socks]]
listen = "127.0.0.1:1080"
username = ""
password = ""

[stealth]
enabled = true
min_delay_ms = 50
max_delay_ms = 500
padding_sizes = [4096, 8192, 16384, 32768]
dummy_probability = 0.1
`
	return os.WriteFile(path, []byte(content), 0644)
}
