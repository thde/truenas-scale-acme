package scale

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"strings"
)

type CertificateCreate string

const CertificateCreateImported = "CERTIFICATE_CREATE_IMPORTED"

type CertificateImport struct {
	Name        string            `json:"name"`
	CreateType  CertificateCreate `json:"create_type"`
	Certificate string            `json:"certificate"`
	Privatekey  string            `json:"privatekey"`
}

type Certificate struct {
	ID                 int      `json:"id,omitempty"`
	Type               int      `json:"type,omitempty"`
	Name               string   `json:"name,omitempty"`
	Certificate        string   `json:"certificate,omitempty"`
	Privatekey         string   `json:"privatekey,omitempty"`
	RootPath           string   `json:"root_path,omitempty"`
	CertificatePath    string   `json:"certificate_path,omitempty"`
	PrivatekeyPath     string   `json:"privatekey_path,omitempty"`
	CSRPath            string   `json:"csr_path,omitempty"`
	CertType           string   `json:"cert_type,omitempty"`
	Revoked            bool     `json:"revoked,omitempty"`
	CanBeRevoked       bool     `json:"can_be_revoked,omitempty"`
	Internal           string   `json:"internal,omitempty"`
	CATypeExisting     bool     `json:"CA_type_existing,omitempty"`
	CATypeInternal     bool     `json:"CA_type_internal,omitempty"`
	CATypeIntermediate bool     `json:"CA_type_intermediate,omitempty"`
	CertTypeExisting   bool     `json:"cert_type_existing,omitempty"`
	CertTypeInternal   bool     `json:"cert_type_internal,omitempty"`
	CertTypeCSR        bool     `json:"cert_type_CSR,omitempty"`
	Issuer             string   `json:"issuer,omitempty"`
	ChainList          []string `json:"chain_list,omitempty"`
	KeyLength          int      `json:"key_length,omitempty"`
	KeyType            string   `json:"key_type,omitempty"`
	Country            string   `json:"country,omitempty"`
	State              string   `json:"state,omitempty"`
	City               string   `json:"city,omitempty"`
	Organization       string   `json:"organization,omitempty"`
	Common             string   `json:"common,omitempty"`
	SAN                []string `json:"san,omitempty"`
	Email              string   `json:"email,omitempty"`
	DN                 string   `json:"DN,omitempty"`
	SubjectNameHash    int64    `json:"subject_name_hash,omitempty"`
	DigestAlgorithm    string   `json:"digest_algorithm,omitempty"`
	Lifetime           int      `json:"lifetime,omitempty"`
	From               string   `json:"from,omitempty"`
	Until              string   `json:"until,omitempty"`
	Serial             *big.Int `json:"serial,omitempty"`
	Chain              bool     `json:"chain,omitempty"`
	Fingerprint        string   `json:"fingerprint,omitempty"`
	Expired            bool     `json:"expired,omitempty"`
	Parsed             bool     `json:"parsed,omitempty"`
}

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

// Certificates returns a list of certificates
func (c *Client) Certificates(ctx context.Context) ([]Certificate, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "certificate", nil, nil)
	if err != nil {
		return nil, err
	}

	certificates := []Certificate{}
	_, err = c.doJSON(req, &certificates)
	return certificates, err
}

// Certificates returns a list of certificates
func (c *Client) Certificate(ctx context.Context, id int) (*Certificate, error) {
	req, err := c.newRequest(ctx, http.MethodGet, fmt.Sprintf("certificate/id/%d", id), nil, nil)
	if err != nil {
		return nil, err
	}

	certificate := Certificate{}
	_, err = c.doJSON(req, &certificate)
	return &certificate, err
}

// CertificateImport imports a certificate
func (c *Client) CertificateImport(ctx context.Context, name string, cert tls.Certificate) (*Certificate, error) {
	pkPEM, err := encodePrivateKeyPEM(cert)
	if err != nil {
		return nil, err
	}

	importCert := CertificateImport{
		Name:        name,
		CreateType:  CertificateCreateImported,
		Certificate: encodeChainPEM(cert),
		Privatekey:  pkPEM,
	}

	req, err := c.newRequest(ctx, http.MethodPost, "certificate", nil, importCert)
	if err != nil {
		return nil, err
	}

	_, err = c.doJob(req)
	if err != nil {
		return nil, err
	}

	certs, err := c.Certificates(ctx)
	if err != nil {
		return nil, err
	}

	for _, cert := range certs {
		if cert.Name != name {
			continue
		}

		return &cert, nil
	}

	return nil, fmt.Errorf("error creating cert %s", name)
}

func encodeChainPEM(cert tls.Certificate) string {
	chain := strings.Builder{}
	for _, cert := range cert.Certificate {
		pem := pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: cert,
		})

		chain.Write(pem)
	}

	return chain.String()
}

func encodePrivateKeyPEM(cert tls.Certificate) (string, error) {
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		return "", err
	}

	pem := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: key,
	})
	return string(pem), nil
}

// CertificateDelete deletes a certificate
func (c *Client) CertificateDelete(ctx context.Context, id int) error {
	req, err := c.newRequest(ctx, http.MethodDelete, fmt.Sprintf("certificate/id/%d", id), nil, nil)
	if err != nil {
		return err
	}

	certificate := Certificate{}
	_, err = c.doJSON(req, &certificate)
	return err
}
