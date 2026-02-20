package truenas

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"strings"
)

// Certificate represents a TrueNAS certificate entry.
type Certificate struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Common      string   `json:"common"`
	Certificate string   `json:"certificate"`
	Privatekey  string   `json:"privatekey"`
	Expired     bool     `json:"expired"`
	From        Time     `json:"from"`
	Until       Time     `json:"until"`
	SAN         []string `json:"san"`
}

// TLSCertificate parses the certificate's PEM fields into a tls.Certificate.
func (c *Certificate) TLSCertificate() (tls.Certificate, error) {
	cert, err := tls.X509KeyPair([]byte(c.Certificate), []byte(c.Privatekey))
	if err != nil {
		return cert, err
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return cert, err
	}

	cert.Leaf = leaf
	return cert, err
}

// CertificateCreateParams holds parameters for creating/importing a certificate.
type CertificateCreateParams struct {
	Name        string `json:"name"`
	CreateType  string `json:"create_type"`
	Certificate string `json:"certificate,omitempty"`
	Privatekey  string `json:"privatekey,omitempty"`
}

// Certificates returns all certificates from TrueNAS.
func (c *Client) Certificates(ctx context.Context) ([]Certificate, error) {
	var result []Certificate
	err := c.withReconnect(ctx, func() error {
		var err error
		result, err = c.a.CertificateQuery(ctx)
		return err
	})
	return result, err
}

// CertificateImport imports a TLS certificate into TrueNAS.
func (c *Client) CertificateImport(ctx context.Context, name string, cert tls.Certificate) (*Certificate, error) {
	pkPEM, err := encodePrivateKeyPEM(cert)
	if err != nil {
		return nil, err
	}

	params := CertificateCreateParams{
		Name:        name,
		CreateType:  "CERTIFICATE_CREATE_IMPORTED",
		Certificate: encodeChainPEM(cert),
		Privatekey:  pkPEM,
	}

	var result *Certificate
	err = c.withReconnect(ctx, func() error {
		var err error
		result, err = c.a.CertificateCreate(ctx, params)
		return err
	})
	return result, err
}

// CertificateDelete deletes a certificate by ID.
func (c *Client) CertificateDelete(ctx context.Context, id int) error {
	return c.withReconnect(ctx, func() error {
		_, err := c.a.CertificateDelete(ctx, id, false)
		return err
	})
}

func encodeChainPEM(cert tls.Certificate) string {
	chain := strings.Builder{}
	for _, derBytes := range cert.Certificate {
		block := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: derBytes,
		})
		chain.Write(block)
	}
	return chain.String()
}

func encodePrivateKeyPEM(cert tls.Certificate) (string, error) {
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return "", err
	}

	block := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: key,
	})
	return string(block), nil
}
