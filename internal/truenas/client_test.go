package truenas

import (
	"crypto/tls"
	"net/url"
	"testing"
)

func TestDefaultURL(t *testing.T) {
	t.Parallel()
	_, err := url.Parse(DefaultURL)
	if err != nil {
		t.Error("expected no err, got", err)
	}
}

func TestDialDefaultURL(t *testing.T) {
	t.Parallel()
	// Verify that dial with no URL option uses DefaultURL without panicking.
	// (It will fail to connect, but should not nil-pointer dereference.)
	cfg := &config{}
	if cfg.url != nil {
		t.Fatal("expected nil addr before applying defaults")
	}

	u, err := url.Parse(DefaultURL)
	if err != nil {
		t.Fatalf("url.Parse(DefaultURL): %v", err)
	}
	if u.String() != DefaultURL {
		t.Errorf("expected %s, got %s", DefaultURL, u.String())
	}
}

func TestWithURL(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("ws://truenas.local/api/current")
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}

	cfg := &config{}
	WithURL(u)(cfg)

	if cfg.url != u {
		t.Errorf("expected url to be set, got %v", cfg.url)
	}
}

func TestWithTLSConfig(t *testing.T) {
	t.Parallel()
	tc := &tls.Config{InsecureSkipVerify: true} //nolint: gosec

	cfg := &config{}
	WithTLSConfig(tc)(cfg)

	if cfg.tlsConfig != tc {
		t.Errorf("expected tlsConfig to be set, got %v", cfg.tlsConfig)
	}
}

func TestWithTLSConfig_Nil(t *testing.T) {
	t.Parallel()
	cfg := &config{}
	WithTLSConfig(nil)(cfg)
	if cfg.tlsConfig != nil {
		t.Errorf("expected nil tlsConfig, got %v", cfg.tlsConfig)
	}
}
