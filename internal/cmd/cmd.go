package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/libdns/acmedns"
	flag "github.com/spf13/pflag"
	"go.uber.org/zap"

	"github.com/thde/truenas-scale-acme/internal/scale"
)

var (
	configPath  = flag.String("config", defaultConfigPath(), "Configuration path")
	flagHelp    = flag.BoolP("help", "h", false, "Print help message")
	flagVersion = flag.BoolP("version", "v", false, "Print version information")
)

var (
	defaultResolvers = []string{
		"193.110.81.0", "185.253.5.0", "2a0f:fc80::", "2a0f:fc81::", // dns0
		"9.9.9.9", "149.112.112.112", "2620:fe::fe", "2620:fe::9", // quad9
	}
	defaultConfig = Config{
		Scale: ScaleConfig{
			URL: "http://localhost/api/v2.0/",
		},
		ACME: ACMEConfig{
			Resolvers: defaultResolvers,
		},
	}
	exampleConfig = Config{
		Domain: "nas.domain.local",
		Scale: ScaleConfig{
			APIKey:     "s3cure",
			URL:        "http://localhost/api/v2.0/",
			SkipVerify: false,
		},
		ACME: ACMEConfig{
			Email:     "myemail@example.com",
			TOSAgreed: false,
			ACMEDNS: acmedns.Provider{
				Username:  "00000000-0000-0000-0000-000000000000",
				Password:  "s3cure",
				Subdomain: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
				ServerURL: "https://auth.acme-dns.io",
			},
		},
	}
)

type BuildInfo struct {
	Version   string
	Commit    string
	Date      string
	GoVersion string
}

func (b *BuildInfo) String() string {
	return fmt.Sprintf("Version %s\nCommit %s\nDate %s\nGo %s\n", b.Version, b.Commit, b.Date, b.GoVersion)
}

type cmd struct {
	*zap.Logger
	*BuildInfo
}

func Run(ctx context.Context, logger *zap.Logger, buildInfo *BuildInfo) error {
	return cmd{Logger: logger, BuildInfo: buildInfo}.Run(ctx)
}

func (c cmd) Run(ctx context.Context) error {
	flag.Parse()

	if *flagHelp {
		fmt.Printf("Usage of %s %s:\n", os.Args[0], c.Version)
		flag.PrintDefaults()
		return nil
	}

	if *flagVersion {
		fmt.Print(c.BuildInfo)
		return nil
	}

	config, err := c.loadConfig(*configPath)
	if err != nil {
		return err
	}
	if config == nil { // if no config existed
		return nil
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Scale.SkipVerify},
		},
	}

	u, err := url.Parse(config.Scale.URL)
	if err != nil {
		return err
	}

	client := scale.NewClient(
		scale.WithAPIKey(config.Scale.APIKey),
		scale.WithBaseURL(u),
		scale.WithHTTPClient(httpClient),
	)

	c.Info("ensure valid certificate is present")

	certmagic.Default.Logger = c.With(zap.String("module", "certmagic"))
	certmagic.DefaultACME.Logger = certmagic.Default.Logger
	certmagic.DefaultACME.Agreed = config.ACME.TOSAgreed
	certmagic.DefaultACME.Email = config.ACME.Email
	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
		DNSProvider: &config.ACME.ACMEDNS,
		Resolvers:   config.ACME.Resolvers,
	}

	magic := certmagic.NewDefault()
	magic.Issuers = []certmagic.Issuer{
		certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
			CA:     certmagic.LetsEncryptProductionCA,
			TestCA: certmagic.LetsEncryptStagingCA,
		}),
		certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
			CA: certmagic.ZeroSSLProductionCA,
		}),
	}

	err = magic.ManageSync(ctx, []string{config.Domain})
	if err != nil {
		return err
	}
	cert, err := magic.CacheManagedCertificate(ctx, config.Domain)
	if err != nil {
		return err
	}

	settings, err := client.SystemGeneral(ctx)
	if err != nil {
		return err
	}
	currentCert, err := settings.UICertificate.TLSCertificate()
	if err != nil {
		return err
	}

	if currentCert.Leaf.Equal(cert.Leaf) {
		c.Info("ui certificate is up to date")
		return nil
	}

	name := fmt.Sprintf("acme-%s", time.Now().Format("20060102-150405"))
	certImport, err := client.CertificateImport(ctx, name, cert.Certificate)
	if err != nil {
		return err
	}
	c.Info("certificate imported", zap.Int("id", certImport.ID), zap.Strings("san", certImport.SAN))

	err = client.SystemGeneralUpdate(ctx, scale.SystemGeneralUpdateParams{UICertificate: certImport.ID})
	if err != nil {
		return err
	}
	c.Info("ui certificate updated")

	return nil
}
