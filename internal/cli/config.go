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

type APIConfig struct {
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

func (ac *ACMEConfig) DNSProvider() (certmagic.DNSProvider, error) {
	if ac.ACMEDNS != nil {
		return ac.ACMEDNS, nil
	} else if ac.Cloudflare != nil {
		return ac.Cloudflare, nil
	}

	return nil, fmt.Errorf("no solver configured")
}

type Config struct {
	Domain string `json:"domain"`
	// API is the configuration for the TrueNAS JSONRPC 2.0 WebSocket API.
	API *APIConfig `json:"api"`
	// Scale is the configuration for the TrueNAS SCALE REST API.
	// Deprecated: Use [Config.API] instead.
	Scale *APIConfig `json:"scale"`
	ACME  ACMEConfig `json:"acme"`
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
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var cf Config
	if err := json.Unmarshal(data, &cf); err != nil {
		return err
	}

	if cf.Domain != "" {
		c.Domain = cf.Domain
	}

	if cf.API != nil {
		if c.API == nil {
			c.API = &APIConfig{}
		}
		if cf.API.APIKey != "" {
			c.API.APIKey = cf.API.APIKey
		}
		if cf.API.URL != "" {
			c.API.URL = cf.API.URL
		}
		c.API.SkipVerify = cf.API.SkipVerify
	}

	if cf.Scale != nil {
		c.Scale = cf.Scale
	}

	if cf.ACME.Email != "" {
		c.ACME.Email = cf.ACME.Email
	}
	if cf.ACME.TOSAgreed {
		c.ACME.TOSAgreed = cf.ACME.TOSAgreed
	}
	if len(cf.ACME.Resolvers) > 0 {
		c.ACME.Resolvers = cf.ACME.Resolvers
	}
	if cf.ACME.Storage != "" {
		c.ACME.Storage = cf.ACME.Storage
	}
	if cf.ACME.ACMEDNS != nil {
		c.ACME.ACMEDNS = cf.ACME.ACMEDNS
	}
	if cf.ACME.Cloudflare != nil {
		c.ACME.Cloudflare = cf.ACME.Cloudflare
	}

	return nil
}

func (c *Config) Write(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(c)
}

func (c *Config) Valid() error {
	errs := []error{}

	if c.Domain == "" {
		errs = append(errs, fmt.Errorf("no domain specified"))
	}

	if c.API == nil {
		errs = append(errs, fmt.Errorf("no api config specified"))
	} else {
		if c.API.APIKey == "" {
			errs = append(errs, fmt.Errorf("no api.api_key specified"))
		}
		if _, err := url.Parse(c.API.URL); err != nil {
			errs = append(errs, fmt.Errorf("invalid api.url: %w", err))
		}
	}

	for _, resolver := range c.ACME.Resolvers {
		if ip := net.ParseIP(resolver); ip == nil {
			errs = append(errs, fmt.Errorf("invalid acme.resolvers: '%s'", resolver))
		}
	}

	if c.ACME.Email == "" {
		errs = append(errs, fmt.Errorf("no acme.email specified"))
	}

	if _, err := c.ACME.DNSProvider(); err != nil {
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

	config = c.handleDeprecatedConfig(config)

	return &config, config.Valid()
}

func (c cmd) handleDeprecatedConfig(conf Config) Config {
	if conf.API == nil && conf.Scale != nil {
		c.CLILogger.Warn("config is using deprecated scale fields", zap.String("domain", conf.Domain))
		apiURL := defaultURL
		if conf.Scale.URL != "" {
			old, err := url.Parse(conf.Scale.URL)
			if err != nil {
				c.CLILogger.Error("invalid scale.url", zap.String("url", conf.Scale.URL), zap.Error(err))
			} else {
				newURL, err := url.Parse(defaultURL)
				if err != nil {
					c.CLILogger.Error("parsing default url", zap.String("url", defaultURL), zap.Error(err))
				} else {
					newURL.Host = old.Host
					apiURL = newURL.String()
				}
			}
		}
		conf.API = &APIConfig{
			APIKey:     conf.Scale.APIKey,
			URL:        apiURL,
			SkipVerify: conf.Scale.SkipVerify,
		}
		conf.Scale = nil
	}

	return conf
}
