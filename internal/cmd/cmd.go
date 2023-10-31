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

	c.CertLogger.Info("ensure valid certificate is present")
	currentCert, err := c.ensureACMECertificate(ctx, config.Domain, config.ACME)
	if err != nil {
		return err
	}

	activeCert, err := c.ensureUICertificate(ctx, client, currentCert)
	if err != nil {
		return err
	}

	return c.removeExpiredCerts(ctx, client, config.Domain, activeCert)
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

	if !activeCertTLS.Leaf.Equal(currentCert.Leaf) {
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

		activeCert = certImport
	} else {
		c.ScaleLogger.Info("ui certificate up to date")
	}

	return activeCert, nil
}

func (c cmd) ensureACMECertificate(ctx context.Context, domain string, config ACMEConfig) (certmagic.Certificate, error) {
	certmagicLogger := c.CertLogger.Named("certmagic")
	certmagic.Default.Logger = certmagicLogger.WithOptions(zap.IncreaseLevel(zap.WarnLevel))
	certmagic.DefaultACME.Logger = certmagicLogger.Named("acme")
	certmagic.DefaultACME.Agreed = config.TOSAgreed
	certmagic.DefaultACME.Email = config.Email
	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
		DNSProvider: &config.ACMEDNS,
		Resolvers:   config.Resolvers,
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

	err := magic.ManageSync(ctx, []string{domain})
	if err != nil {
		return certmagic.Certificate{}, err
	}
	return magic.CacheManagedCertificate(ctx, domain)
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
