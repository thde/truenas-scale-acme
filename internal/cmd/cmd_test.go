package cmd

import (
	"net"
	"testing"
)

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
