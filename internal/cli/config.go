package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/acmedns"
	"github.com/libdns/cloudflare"
	"go.uber.org/zap"
)

type ScaleConfig struct {
	APIKey     string `json:"api_key"`
	URL        string `json:"url"`
	SkipVerify bool   `json:"skip_verify"`
}

type ACMEConfig struct {
	Email      string               `json:"email"`
	TOSAgreed  bool                 `json:"tos_agreed"`
	Resolvers  []string             `json:"resolvers,omitempty"`
	Storage    string               `json:"storage,omitempty"`
	ACMEDNS    *acmedns.Provider    `json:"acme-dns,omitempty"`
	Cloudflare *cloudflare.Provider `json:"cloudflare,omitempty"`
}

func (ac *ACMEConfig) DNSProvider() (certmagic.ACMEDNSProvider, error) {
	if ac.ACMEDNS != nil {
		return ac.ACMEDNS, nil
	} else if ac.Cloudflare != nil {
		return ac.Cloudflare, nil
	}

	return nil, fmt.Errorf("no solver configured")
}

type Config struct {
	Domain string      `json:"domain"`
	Scale  ScaleConfig `json:"scale"`
	ACME   ACMEConfig  `json:"acme"`
}

func defaultConfigPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}

	return filepath.Join(base, "truenas-scale-acme", "config.json")
}

func defaultDataDir() string {
	baseDir := os.Getenv("XDG_DATA_HOME")

	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}

		baseDir = filepath.Join(home, ".local")
	}

	return filepath.Join(baseDir, "truenas-scale-acme")
}

func (c *Config) Merge(r io.Reader) error {
	return json.NewDecoder(r).Decode(c)
}

func (c *Config) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func (c *Config) Valid() error {
	errs := []error{}

	if c.Domain == "" {
		errs = append(errs, fmt.Errorf("no domain specified: '%s'", c.Scale.APIKey))
	}

	if c.Scale.APIKey == "" {
		errs = append(errs, fmt.Errorf("no scale.api_key specified: '%s'", c.Scale.APIKey))
	}

	_, err := url.Parse(c.Scale.URL)
	if err != nil {
		errs = append(errs, fmt.Errorf("invalid scale.url: %w", err))
	}

	for _, resolver := range c.ACME.Resolvers {
		if ip := net.ParseIP(resolver); ip == nil {
			errs = append(errs, fmt.Errorf("invalid acme.resolvers: '%s'", resolver))
		}
	}

	if c.ACME.Email == "" {
		errs = append(errs, fmt.Errorf("no acme.email specified: '%s'", c.Scale.APIKey))
	}

	if _, err = c.ACME.DNSProvider(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

func (c cmd) loadConfig(path string) (*Config, error) {
	flags := os.O_RDONLY

	// if the default config is used,
	// an example config should be written.
	if path == defaultConfigPath() {
		err := os.MkdirAll(filepath.Dir(path), os.ModePerm)
		if err != nil {
			return nil, err
		}

		flags = os.O_RDWR | os.O_CREATE
	}

	c.CLILogger.Info("reading config", zap.String("path", path))
	configFile, err := os.OpenFile(path, flags, 0o700)
	if err != nil {
		return nil, err
	}
	defer configFile.Close()

	s, err := configFile.Stat()
	if err != nil {
		return nil, err
	}

	if s.Size() == 0 && flags != os.O_RDONLY {
		c.CLILogger.Info("config does not exist, writing example config", zap.String("path", path))
		return nil, exampleConfig.Write(configFile)
	}

	config := defaultConfig
	err = config.Merge(configFile)
	if err != nil {
		return nil, err
	}

	return &config, config.Valid()
}
