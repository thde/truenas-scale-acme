// Package cli provides the command-line interface and core logic for the
// TrueNAS SCALE ACME certificate automation tool.
package cli

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/caddyserver/certmagic"
	"github.com/klauspost/compress/gzhttp"
	"github.com/libdns/acmedns"
	"github.com/libdns/cloudflare"
	"github.com/mholt/acmez/v3/acme"
	flag "github.com/spf13/pflag"
	"github.com/thde/truenas-scale-acme/internal/cron"
	"github.com/thde/truenas-scale-acme/internal/scale"
	"github.com/thde/truenas-scale-acme/internal/zerossl"
	"go.uber.org/zap"
)

var (
	flagConfigPath = flag.String("config", defaultConfigPath(), "Configuration path")
	flagDaemon     = flag.Bool("daemon", false, "Run in daemon mode")
	flagSchedule   = flag.String("schedule", "22 22 * * *", "Cron schedule, if daemon mode is enabled")
	flagHelp       = flag.BoolP("help", "h", false, "Print help message")
	flagVersion    = flag.BoolP("version", "v", false, "Print version information")
)

var (
	defaultResolvers = []string{
		"9.9.9.9", "149.112.112.112", "2620:fe::fe", "2620:fe::9", // quad9
		"86.54.11.100", "86.54.11.200", "2a13:1001::86:54:11:100", "2a13:1001::86:54:11:200", // dns4eu
	}
	defaultConfig = Config{
		Scale: ScaleConfig{
			URL: "http://localhost/api/v2.0/",
		},
		ACME: ACMEConfig{
			Resolvers: defaultResolvers,
			Storage:   defaultDataDir(),
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
			ACMEDNS: &acmedns.Provider{
				Username:  "00000000-0000-0000-0000-000000000000",
				Password:  "s3cure",
				Subdomain: "FFFFFFFF-FFFF-FFFF-FFFF-FFFFFFFFFFFF",
				ServerURL: "https://auth.acme-dns.io",
			},
			Cloudflare: &cloudflare.Provider{
				APIToken: "s3cure",
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
	CertLogger  *zap.Logger
	ScaleLogger *zap.Logger
	CLILogger   *zap.Logger

	*BuildInfo
}

func Run(ctx context.Context, logger *zap.Logger, buildInfo *BuildInfo) error {
	return cmd{
		CertLogger:  logger.Named("certificate"),
		ScaleLogger: logger.Named("scale"),
		CLILogger:   logger.Named("cli"),
		BuildInfo:   buildInfo,
	}.Run(ctx)
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

	c.CLILogger.Info("starting",
		zap.String("version", c.Version),
		zap.String("go", c.GoVersion),
		zap.String("commit", c.Commit),
		zap.String("date", c.Date),
	)

	config, err := c.loadConfig(*flagConfigPath)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", *flagConfigPath, err)
	}
	if config == nil { // if no config existed
		return fmt.Errorf("no config found at %s", *flagConfigPath)
	}

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: gzhttp.Transport(&http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Scale.SkipVerify}, //nolint: gosec
		}),
	}

	u, err := url.Parse(config.Scale.URL)
	if err != nil {
		return fmt.Errorf("error parsing scale url %q: %w", config.Scale.URL, err)
	}

	scaleClient := scale.NewClient(
		scale.WithAPIKey(config.Scale.APIKey),
		scale.WithBaseURL(u),
		scale.WithHTTPClient(httpClient),
	)

	acmeClient, err := c.acmeClient(config.ACME)
	if err != nil {
		return fmt.Errorf("error creating ACME client: %w", err)
	}

	if *flagDaemon {
		c.CLILogger.Info("daemon mode enabled", zap.String("schedule", *flagSchedule))
	}

	err = c.ensureCertificate(ctx, config, acmeClient, scaleClient)
	if err != nil {
		return err
	}
	if !*flagDaemon {
		return nil
	}

	ticker, err := cron.NewTickerWithLocation(*flagSchedule, time.Local)
	if err != nil {
		return fmt.Errorf("error parsing schedule %s: %w", *flagSchedule, err)
	}
	defer ticker.Stop()

	for range ticker.C {
		if err = c.ensureCertificate(ctx, config, acmeClient, scaleClient); err != nil {
			return err
		}
	}

	return nil
}

func (c cmd) ensureCertificate(ctx context.Context, cfg *Config, acmeClient *certmagic.Config, scaleClient *scale.Client) error {
	c.CLILogger.Info("ensure valid certificate is present")
	currentCert, err := c.ensureACMECertificate(ctx, cfg.Domain, acmeClient)
	if err != nil {
		c.CLILogger.Warn("error ensuring certificate, skipping update...", zap.Error(err))

		return nil
	}

	activeCert, err := c.ensureUICertificate(ctx, scaleClient, currentCert)
	if err != nil {
		return fmt.Errorf("error ensuring ui certificate for %s: %w", cfg.Domain, err)
	}

	return c.removeExpiredCerts(ctx, scaleClient, cfg.Domain, activeCert)
}

func (c cmd) ensureACMECertificate(ctx context.Context, domain string, acmeClient *certmagic.Config) (certmagic.Certificate, error) {
	if err := acmeClient.ManageSync(ctx, []string{domain}); err != nil {
		return certmagic.Certificate{}, fmt.Errorf("error ensuring certificate for %q: %w", domain, err)
	}
	currentCert, err := acmeClient.CacheManagedCertificate(ctx, domain)
	if err != nil {
		return currentCert, fmt.Errorf("error storing certificate for %q: %w", domain, err)
	}

	return currentCert, nil
}

func (c cmd) ensureUICertificate(ctx context.Context, client *scale.Client, currentCert certmagic.Certificate) (*scale.Certificate, error) {
	settings, err := client.SystemGeneral(ctx)
	if err != nil {
		return nil, err
	}
	activeCert := &settings.UICertificate
	activeCertTLS, err := activeCert.TLSCertificate()
	if err != nil {
		return nil, err
	}

	if activeCertTLS.Leaf.Equal(currentCert.Leaf) {
		c.ScaleLogger.Info("ui certificate up to date")
		return activeCert, nil
	}

	name := fmt.Sprintf("acme-%s", time.Now().Format("20060102-150405"))
	c.ScaleLogger.Info("importing certificate", zap.String("name", name), zap.Strings("san", currentCert.Leaf.DNSNames))
	certImport, err := client.CertificateImport(ctx, name, currentCert.Certificate)
	if err != nil {
		return activeCert, err
	}

	err = client.SystemGeneralUpdate(ctx, scale.SystemGeneralUpdateParams{UICertificate: certImport.ID})
	if err != nil {
		return activeCert, err
	}
	c.ScaleLogger.Info("ui certificate updated")

	return certImport, nil
}

func (c cmd) acmeClient(config ACMEConfig) (*certmagic.Config, error) {
	certmagicLogger := c.CertLogger.Named("certmagic")
	certmagic.Default.Logger = certmagicLogger.WithOptions(zap.IncreaseLevel(zap.WarnLevel))
	certmagic.DefaultACME.Logger = certmagicLogger.Named("acme")
	certmagic.DefaultACME.Agreed = config.TOSAgreed
	certmagic.DefaultACME.Email = config.Email
	certmagic.Default.Storage = &certmagic.FileStorage{Path: config.Storage}

	provider, err := config.DNSProvider()
	if err != nil {
		return nil, fmt.Errorf("dns provider could not be loaded: %w", err)
	}
	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{DNSManager: certmagic.DNSManager{
		Resolvers:   config.Resolvers,
		DNSProvider: provider,
	}}

	magic := certmagic.NewDefault()
	magic.Issuers = []certmagic.Issuer{
		certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
			CA:     certmagic.LetsEncryptProductionCA,
			TestCA: certmagic.LetsEncryptStagingCA,
		}),
		// ZeroSSL requires EAB
		certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
			CA: certmagic.ZeroSSLProductionCA,
			NewAccountFunc: func(ctx context.Context, issuer *certmagic.ACMEIssuer, account acme.Account) (acme.Account, error) {
				if issuer.ExternalAccount != nil {
					return account, nil
				}
				credentials, account, err := zerossl.EABCredentials(ctx, config.Email, account)
				issuer.ExternalAccount = credentials
				return account, err
			},
		}),
	}

	return magic, nil
}

func (c cmd) removeExpiredCerts(ctx context.Context, client *scale.Client, domain string, activeCert *scale.Certificate) error {
	certs, err := client.Certificates(ctx)
	if err != nil {
		return err
	}

	for _, cert := range certs {
		if cert.Common != domain {
			continue
		}
		if !cert.Expired {
			continue
		}
		if cert.ID == activeCert.ID {
			continue
		}

		c.ScaleLogger.Info("removing expired certificate", zap.Int("id", cert.ID), zap.String("cn", cert.Common), zap.Time("expired", cert.Until.Time))
		err = client.CertificateDelete(ctx, cert.ID)
		if err != nil {
			return fmt.Errorf("error removing certificate %d for %s: %w", cert.ID, cert.Common, err)
		}
	}

	return nil
}
