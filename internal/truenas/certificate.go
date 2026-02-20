package truenas

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
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

	// certificate.create is a job: it returns a job ID, and the certificate is
	// produced once the job completes.
	var jobID int
	err = c.withReconnect(ctx, func() error {
		var err error
		jobID, err = c.a.CertificateCreate(ctx, params)
		return err
	})
	if err != nil {
		return nil, err
	}
	if _, err := c.waitForJob(ctx, jobID); err != nil {
		return nil, err
	}

	// The job result redacts sensitive fields for external clients, so re-query
	// to retrieve the imported certificate by name.
	certs, err := c.Certificates(ctx)
	if err != nil {
		return nil, err
	}
	for i := range certs {
		if certs[i].Name == name {
			return &certs[i], nil
		}
	}
	return nil, fmt.Errorf("certificate %q not found after import", name)
}

// CertificateDelete deletes a certificate by ID.
func (c *Client) CertificateDelete(ctx context.Context, id int) error {
	// certificate.delete is a job: it returns a job ID and completes asynchronously.
	var jobID int
	err := c.withReconnect(ctx, func() error {
		var err error
		jobID, err = c.a.CertificateDelete(ctx, id, false)
		return err
	})
	if err != nil {
		return err
	}
	_, err = c.waitForJob(ctx, jobID)
	return err
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
