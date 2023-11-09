package cli

import (
	"reflect"
	"testing"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/acmedns"
	"github.com/libdns/cloudflare"
)

func TestACMEConfig_DNSProvider(t *testing.T) {
	type fields struct {
		ACMEDNS    *acmedns.Provider
		Cloudflare *cloudflare.Provider
	}
	tests := []struct {
		name    string
		fields  fields
		want    certmagic.ACMEDNSProvider
		wantErr bool
	}{
		{"no configured", fields{nil, nil}, nil, true},
		{"acme", fields{&acmedns.Provider{}, nil}, &acmedns.Provider{}, false},
		{"cloudflare", fields{nil, &cloudflare.Provider{}}, &cloudflare.Provider{}, false},
		{"multiple", fields{&acmedns.Provider{}, &cloudflare.Provider{}}, &acmedns.Provider{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &ACMEConfig{
				ACMEDNS:    tt.fields.ACMEDNS,
				Cloudflare: tt.fields.Cloudflare,
			}
			got, err := ac.DNSProvider()
			if (err != nil) != tt.wantErr {
				t.Errorf("ACMEConfig.DNSProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ACMEConfig.DNSProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}
