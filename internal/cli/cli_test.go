package cli

import (
	"net"
	"net/url"
	"testing"
)

func Test_defaultURL(t *testing.T) {
	t.Parallel()

	if _, err := url.Parse(defaultURL); err != nil {
		t.Errorf("defaultURL %q is not a valid URL: %s", defaultURL, err)
	}
}

func Test_defaultResolvers(t *testing.T) {
	for _, resolver := range defaultResolvers {
		if ip := net.ParseIP(resolver); ip == nil {
			t.Errorf("invalid resolver : '%s'", resolver)
		}
	}
}

func Test_exampleConfig(t *testing.T) {
	if err := exampleConfig.Valid(); err != nil {
		t.Errorf("exampleConfig invalid: %s", err)
	}
}
